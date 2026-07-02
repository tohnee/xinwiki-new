package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/utils"
)

// LLMQueryRewriter rewrites queries using an LLM to produce standalone
// (history-aware) queries, related expanded queries, and extracted entities.
// If the model ID is empty or the model cannot be resolved, Rewrite/
// RewriteWithContext degrade to a simple identity rewrite with keyword-based
// entity extraction so that callers never fail because the rewriter is
// misconfigured.
type LLMQueryRewriter struct {
	modelID string
	// getChatModel resolves a model ID to a chat.Chat. Indirected so tests can
	// inject fakes without touching the global service locator.
	getChatModel func(ctx context.Context, modelID string) (chat.Chat, error)
}

// rewriteResponse is the JSON shape we ask the model to emit.
type rewriteResponse struct {
	StandaloneQuery string   `json:"standalone_query"`
	ExpandedQueries []string `json:"expanded_queries"`
	Entities        []string `json:"entities"`
	Keywords        []string `json:"keywords"`
}

// NewLLMQueryRewriter constructs an LLM-based query rewriter. modelID may be
// empty, in which case the rewriter degrades to a no-op keyword rewriter.
func NewLLMQueryRewriter(
	modelID string,
	getChatModel func(ctx context.Context, modelID string) (chat.Chat, error),
) *LLMQueryRewriter {
	if modelID == "" {
		modelID = os.Getenv("WIKI_QUERY_REWRITE_MODEL")
	}
	return &LLMQueryRewriter{
		modelID:      strings.TrimSpace(modelID),
		getChatModel: getChatModel,
	}
}

// Enabled reports whether the rewriter has a model configured.
func (r *LLMQueryRewriter) Enabled() bool {
	return r != nil && r.modelID != "" && r.getChatModel != nil
}

// Rewrite implements QueryRewriter for a standalone query (no conversation
// history). It expands synonyms/related queries and extracts entities/keywords.
func (r *LLMQueryRewriter) Rewrite(ctx context.Context, query string) (*QueryRewrite, error) {
	return r.RewriteWithContext(ctx, query, nil)
}

// RewriteWithContext produces a decontextualized standalone query, related
// expansion queries, and entities/keywords using the LLM. When history is
// empty or the LLM is unavailable it falls back to a keyword-based extractor
// so retrieval can proceed.
func (r *LLMQueryRewriter) RewriteWithContext(ctx context.Context, query string, history []Message) (*QueryRewrite, error) {
	if strings.TrimSpace(query) == "" {
		return &QueryRewrite{OriginalQuery: query}, nil
	}

	// Fast path: no model configured → use the deterministic fallback.
	if !r.Enabled() {
		return r.keywordFallback(query), nil
	}

	llm, err := r.getChatModel(ctx, r.modelID)
	if err != nil {
		logger.Warnf(ctx, "[wiki/rewriter] failed to resolve model %q: %v; falling back to keyword rewriter", r.modelID, err)
		return r.keywordFallback(query), nil
	}

	systemPrompt := `You are a query rewriting assistant for a knowledge-base retrieval system.

Given a user's latest query and, optionally, the preceding conversation history, you must:
1. Produce a "standalone_query": a self-contained version of the latest query that resolves pronouns ("it", "they", "that") and ellipsis ("how?", "why?") using the conversation context. If there is no history, or the query is already self-contained, return it unchanged.
2. Produce 1-3 "expanded_queries": short rephrasings / synonym queries that a keyword search might miss, written in the language of the original query. Do NOT restate the standalone query verbatim. If no useful expansions exist, return an empty list.
3. Extract "entities": 0-8 key named entities / technical terms / product names / acronyms central to the query. Lowercase; no duplicates.
4. Extract "keywords": 0-8 important content words (nouns/verbs) from the standalone query, lowercased.

Return ONLY strict JSON matching the schema. No prose, no code fences.`

	// Compose the history block. Keep it compact to save tokens.
	var historyText strings.Builder
	if len(history) > 0 {
		historyText.WriteString("## Conversation History (oldest first):\n")
		const maxTurns = 6
		start := 0
		if len(history) > maxTurns {
			start = len(history) - maxTurns
		}
		for _, m := range history[start:] {
			role := strings.ToLower(strings.TrimSpace(m.Role))
			if role == "" {
				role = "user"
			}
			historyText.WriteString(fmt.Sprintf("- %s: %s\n", role, truncate(m.Content, 300)))
		}
	}
	userPrompt := fmt.Sprintf(`%s
## Latest User Query:
%s

Respond with JSON only.`, historyText.String(), query)

	messages := []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := llm.Chat(ctx, messages, &chat.ChatOptions{
		Temperature: 0.2,
		MaxTokens:   600,
		Format:      utils.GenerateSchema[rewriteResponse](),
	})
	if err != nil {
		logger.Warnf(ctx, "[wiki/rewriter] LLM call failed: %v; falling back", err)
		return r.keywordFallback(query), nil
	}

	var parsed rewriteResponse
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		logger.Warnf(ctx, "[wiki/rewriter] failed to parse LLM JSON: %v (content=%q); falling back", err, truncate(resp.Content, 200))
		return r.keywordFallback(query), nil
	}

	standalone := strings.TrimSpace(parsed.StandaloneQuery)
	if standalone == "" {
		standalone = query
	}
	expanded := dedupNonEmpty(append([]string{standalone}, parsed.ExpandedQueries...))
	// The standalone query is already the primary query; callers prepend the
	// original query themselves, so here expanded = [standalone? no, callers will
	// include the original already]. Return expansions but guard against dupes.
	expanded = dedupNonEmpty(parsed.ExpandedQueries)
	entities := dedupLower(parsed.Entities)
	keywords := dedupLower(parsed.Keywords)

	out := &QueryRewrite{
		OriginalQuery:  query,
		ExpandedQueries: expanded,
		Entities:       entities,
		Keywords:       keywords,
	}
	// If the standalone differs from the original query, treat it as the first
	// expansion so retrievers run against the decontextualized form too.
	if standalone != query {
		out.ExpandedQueries = append([]string{standalone}, out.ExpandedQueries...)
	}
	out.ExpandedQueries = dedupNonEmpty(out.ExpandedQueries)

	if len(out.ExpandedQueries) > 5 {
		out.ExpandedQueries = out.ExpandedQueries[:5]
	}
	if len(out.Entities) > 12 {
		out.Entities = out.Entities[:12]
	}
	return out, nil
}

// ExtractEntities pulls entities out of a query. When the LLM is unavailable
// it returns a best-effort keyword set.
func (r *LLMQueryRewriter) ExtractEntities(ctx context.Context, query string) ([]string, error) {
	rewrite, err := r.Rewrite(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(rewrite.Entities) > 0 {
		return rewrite.Entities, nil
	}
	return rewrite.Keywords, nil
}

// keywordFallback is the deterministic rewrite used when the LLM is disabled
// or fails. It returns the query as-is plus a few naive token-based keywords.
func (r *LLMQueryRewriter) keywordFallback(query string) *QueryRewrite {
	return &QueryRewrite{
		OriginalQuery:   query,
		ExpandedQueries: nil,
		Entities:        extractKeywords(query, 8),
		Keywords:        extractKeywords(query, 8),
	}
}

// extractKeywords is a tiny stopword-filtered token extractor used as a
// last-resort fallback when no LLM is available. It is intentionally crude
// (no NLP) — it exists only so retrieval can still get entities/keywords.
func extractKeywords(s string, max int) []string {
	stopwords := map[string]struct{}{
		"a": {}, "an": {}, "the": {}, "is": {}, "are": {}, "was": {}, "were": {},
		"be": {}, "been": {}, "being": {}, "have": {}, "has": {}, "had": {},
		"do": {}, "does": {}, "did": {}, "will": {}, "would": {}, "could": {},
		"should": {}, "may": {}, "might": {}, "shall": {}, "can": {}, "to": {},
		"of": {}, "in": {}, "on": {}, "at": {}, "by": {}, "for": {}, "with": {},
		"about": {}, "from": {}, "as": {}, "into": {}, "through": {}, "and": {},
		"or": {}, "but": {}, "if": {}, "then": {}, "else": {}, "so": {}, "that": {},
		"this": {}, "these": {}, "those": {}, "it": {}, "its": {}, "i": {}, "you": {},
		"he": {}, "she": {}, "we": {}, "they": {}, "what": {}, "which": {}, "who": {},
		"how": {}, "why": {}, "when": {}, "where": {}, "me": {}, "my": {}, "your": {},
	}
	fields := strings.Fields(strings.ToLower(s))
	out := make([]string, 0, max)
	seen := make(map[string]struct{})
	for _, f := range fields {
		tok := strings.Trim(f, " \t\n\r.,;:!?\"'()[]{}<>/\\")
		if len(tok) < 3 {
			continue
		}
		if _, bad := stopwords[tok]; bad {
			continue
		}
		if _, dup := seen[tok]; dup {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
		if len(out) >= max {
			break
		}
	}
	return out
}

func dedupNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{})
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func dedupLower(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{})
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

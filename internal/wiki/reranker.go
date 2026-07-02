package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/utils"
)

// Cross-encoder reranking scores each candidate chunk against the query using
// an LLM prompt that asks for a 0-10 relevance score. This is an approximation
// of a true cross-encoder (which runs query+document through a single
// transformer) but any instruction-tuned chat model can produce usable scores.
//
// Design choices:
//   - Top-N cap (default 50) to bound LLM cost; inputs beyond the cap are
//     returned in their original RRF order after the reranked top-N.
//   - Per-candidate calls are made concurrently (bounded worker pool) and
//     results are merged back into a stable order on score (descending), with
//     original RRF rank used as a tiebreaker so identical scores preserve the
//     stronger fusion result.
//   - Failures on individual candidates are non-fatal: the candidate keeps
//     its incoming score. A complete LLM failure falls back to the NoopReranker.
const (
	defaultRerankTopN    = 50
	defaultRerankWorkers = 8
	// capChunkChars bounds chunk content sent to the LLM per candidate.
	capChunkChars = 800
)

// LLMReranker reranks top-N search results using an LLM as a cross-encoder.
type LLMReranker struct {
	modelID string
	getChatModel func(ctx context.Context, modelID string) (chat.Chat, error)
	topN   int
	workers int
}

// rerankResponse is the JSON shape the model must return per candidate.
type rerankResponse struct {
	Score       float64 `json:"score"`
	Reason      string  `json:"reason,omitempty"`
	IsRelevant  bool    `json:"is_relevant"`
}

// NewLLMReranker constructs an LLM cross-encoder reranker. modelID may be
// empty (reads WIKI_RERANK_MODEL); if still empty, callers should use
// NewNoopReranker instead.
func NewLLMReranker(
	modelID string,
	getChatModel func(ctx context.Context, modelID string) (chat.Chat, error),
) *LLMReranker {
	if modelID == "" {
		modelID = os.Getenv("WIKI_RERANK_MODEL")
	}
	topN := defaultRerankTopN
	if v := os.Getenv("WIKI_RERANK_TOPN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topN = n
		}
	}
	workers := defaultRerankWorkers
	if v := os.Getenv("WIKI_RERANK_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 32 {
			workers = n
		}
	}
	return &LLMReranker{
		modelID:      strings.TrimSpace(modelID),
		getChatModel: getChatModel,
		topN:         topN,
		workers:      workers,
	}
}

// Enabled reports whether a rerank model is configured.
func (r *LLMReranker) Enabled() bool {
	return r != nil && r.modelID != "" && r.getChatModel != nil
}

// NoopReranker returns its input unchanged (in stable sorted order by FinalScore).
type NoopReranker struct{}

// NewNoopReranker returns a reranker that passes input through.
func NewNoopReranker() *NoopReranker { return &NoopReranker{} }

// Rerank returns results sorted by FinalScore descending, unchanged.
func (n *NoopReranker) Rerank(ctx context.Context, _ string, results []*SearchResult, topN int) ([]*SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	sorted := make([]*SearchResult, len(results))
	copy(sorted, results)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].FinalScore > sorted[j].FinalScore
	})
	if topN > 0 && len(sorted) > topN {
		sorted = sorted[:topN]
	}
	return sorted, nil
}

// Rerank applies LLM cross-encoder scoring to the top-N incoming results,
// returning the reranked list followed by any results beyond the top-N cap
// in their original order. On model failure it falls back to NoopReranker.
func (r *LLMReranker) Rerank(ctx context.Context, query string, results []*SearchResult, topN int) ([]*SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}
	if !r.Enabled() {
		return NewNoopReranker().Rerank(ctx, query, results, topN)
	}

	llm, err := r.getChatModel(ctx, r.modelID)
	if err != nil {
		logger.Warnf(ctx, "[wiki/reranker] failed to resolve model %q: %v; no-op", r.modelID, err)
		return NewNoopReranker().Rerank(ctx, query, results, topN)
	}

	// Bound the candidate pool we send to the LLM.
	capN := r.topN
	if topN > 0 && topN < capN {
		capN = topN
	}
	candidates := results
	tail := []*SearchResult(nil)
	if len(candidates) > capN {
		candidates = candidates[:capN]
		tail = results[capN:]
	}

	scores := make([]float64, len(candidates))
	// Pre-seed with incoming final scores so LLM failures do not zero out.
	for i, c := range candidates {
		scores[i] = c.FinalScore
	}

	type idxErr struct {
		idx int
		err error
	}
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		errs    []error
		sem     = make(chan struct{}, r.workers)
	)

	scoreCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, cand := range candidates {
		if cand == nil || cand.Chunk == nil {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, cand *SearchResult) {
			defer wg.Done()
			defer func() { <-sem }()
			s, err := r.scoreOne(scoreCtx, llm, query, cand)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			scores[i] = s
		}(i, cand)
	}
	wg.Wait()

	if len(errs) > 0 {
		logger.Debugf(ctx, "[wiki/reranker] %d candidate(s) failed scoring; using fusion scores for those", len(errs))
	}

	// Build ranked list from the scored candidates, then append tail.
	reranked := make([]*SearchResult, 0, len(candidates)+len(tail))
	for i, c := range candidates {
		if c == nil {
			continue
		}
		clone := *c
		reranked = append(reranked, &clone)
		reranked[len(reranked)-1].FinalScore = scores[i]
	}
	sort.SliceStable(reranked, func(i, j int) bool {
		if reranked[i].FinalScore != reranked[j].FinalScore {
			return reranked[i].FinalScore > reranked[j].FinalScore
		}
		// Tie-break: prefer the candidate that was higher before reranking.
		return i < j
	})

	for _, t := range tail {
		if t == nil {
			continue
		}
		clone := *t
		reranked = append(reranked, &clone)
	}

	if topN > 0 && len(reranked) > topN {
		reranked = reranked[:topN]
	}
	for i, r := range reranked {
		r.Rank = i + 1
	}
	return reranked, nil
}

// scoreOne asks the LLM to judge query-document relevance on a 0-10 scale.
// It returns a score in [0, 1] (normalized by dividing by 10) suitable for
// multiplication with the fusion score.
func (r *LLMReranker) scoreOne(ctx context.Context, llm chat.Chat, query string, cand *SearchResult) (float64, error) {
	content := cand.Chunk.Content
	if len(content) > capChunkChars {
		content = content[:capChunkChars] + "..."
	}
	source := cand.Chunk.Path
	if source == "" {
		source = cand.Chunk.Section
	}
	systemPrompt := `You are a relevance judge for a knowledge-base search engine.

Given a USER QUERY and a single DOCUMENT CHUNK, rate how relevant the chunk is to the query on a 0-10 integer scale where:
- 0 = completely unrelated (off-topic or no overlap)
- 3 = tangentially related (mentions a keyword but does not answer)
- 5 = partially relevant (contains some useful signal but is incomplete)
- 8 = highly relevant (directly addresses the core of the query)
- 10 = perfect match (the chunk contains a direct, complete answer)

Return strict JSON of the form {"score": <int 0-10>, "is_relevant": <bool>, "reason": "<=12 words>"}.
Do NOT wrap in code fences. Do NOT add prose. Ignore the source path when judging; use content only.`

	userPrompt := fmt.Sprintf(`USER QUERY: %s

DOCUMENT SOURCE: %s
DOCUMENT CHUNK:
%s`, query, source, content)

	messages := []chat.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}
	resp, err := llm.Chat(ctx, messages, &chat.ChatOptions{
		Temperature: 0.0,
		MaxTokens:   120,
		Format:      utils.GenerateSchema[rerankResponse](),
	})
	if err != nil {
		return cand.FinalScore, fmt.Errorf("llm rerank call failed: %w", err)
	}
	var parsed rerankResponse
	if err := json.Unmarshal([]byte(resp.Content), &parsed); err != nil {
		// Last-ditch: try to extract a leading number from the response.
		if f, perr := strconv.ParseFloat(strings.TrimSpace(strings.SplitN(resp.Content, "\n", 2)[0]), 64); perr == nil {
			parsed.Score = f
		} else {
			return cand.FinalScore, fmt.Errorf("failed to parse rerank JSON: %w (content=%q)", err, truncate(resp.Content, 200))
		}
	}
	score := parsed.Score
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	// Normalize to [0,1]. Apply a small floor so fusion-relevant but LLM-0 items
	// are not completely crushed; but when is_relevant=false the score is heavily
	// discounted.
	normalized := score / 10.0
	if !parsed.IsRelevant && normalized > 0.3 {
		normalized = math.Min(normalized, 0.3)
	}
	// Blend with original fusion score so the rerank adjusts, not replaces.
	const rerankWeight = 0.6
	return rerankWeight*normalized + (1-rerankWeight)*cand.FinalScore, nil
}

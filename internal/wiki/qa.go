package wiki

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
)

// NewQAEngine creates a new high-precision question answering engine.
func NewQAEngine(
	retriever *HybridRetriever,
	llm LLMClient,
	citationVerifier CitationVerifier,
	confidenceScorer ConfidenceScorer,
) *QAEngine {
	return &QAEngine{
		retriever:        retriever,
		llm:              llm,
		citationVerifier: citationVerifier,
		confidenceScorer: confidenceScorer,
	}
}

// Answer performs high-precision question answering with citations.
func (e *QAEngine) Answer(ctx context.Context, question string, tenantID string, kbIDs []string) (*Answer, error) {
	if e == nil || e.llm == nil {
		return nil, fmt.Errorf("wiki/qa: engine not properly initialized (llm is nil)")
	}
	if e.retriever == nil {
		return nil, fmt.Errorf("wiki/qa: retriever not configured")
	}
	startTime := time.Now()
	answer := &Answer{
		ID:        generateID(),
		Question:  question,
		CreatedAt: time.Now(),
		ModelUsed: e.llm.ModelName(),
	}

	// Step 1: Retrieve relevant context
	thinkingSteps := make([]ReasoningStep, 0)
	thinkingSteps = append(thinkingSteps, ReasoningStep{
		Step:       1,
		Thought:    "Analyzing question and retrieving relevant context",
		Action:     "hybrid_retrieval",
		Timestamp:  time.Now(),
	})

	retrievalStart := time.Now()
	retrievalReq := &RetrievalRequest{
		Query:          question,
		TenantID:       tenantID,
		KnowledgeBaseIDs: kbIDs,
		TopK:           8,
		Methods:        []RetrievalMethod{MethodHybrid},
		UseRRF:         true,
		RRFConstant:    60,
		UseCache:       true,
		MinScore:       0.1,
	}
	retrievalResp, err := e.retriever.Retrieve(ctx, retrievalReq)
	retrievalDuration := time.Since(retrievalStart)

	var resultCount int
	if retrievalResp != nil {
		resultCount = len(retrievalResp.Results)
	}
	thinkingSteps[0].DurationMs = retrievalDuration.Milliseconds()
	thinkingSteps[0].Observation = fmt.Sprintf("Retrieved %d relevant chunks in %dms",
		resultCount, retrievalDuration.Milliseconds())

	if err != nil {
		logger.Warnf(ctx, "[wiki/qa] retrieval failed: %v", err)
		retrievalResp = &RetrievalResponse{Results: nil}
	}

	// Step 2: Build context from retrieved chunks
	chunks := make([]*Chunk, 0, len(retrievalResp.Results))
	for _, r := range retrievalResp.Results {
		if r != nil && r.Chunk != nil {
			chunks = append(chunks, r.Chunk)
		}
	}

	contextText := e.buildContext(chunks)
	thinkingSteps = append(thinkingSteps, ReasoningStep{
		Step:      2,
		Thought:   "Building context from retrieved chunks",
		Action:    "context_construction",
		Observation: fmt.Sprintf("Built context with %d tokens from %d chunks", estimateTokens(contextText), len(chunks)),
		Timestamp: time.Now(),
	})

	// Step 3: Generate answer using LLM
	prompt := e.buildQAPrompt(question, contextText)
	llmStart := time.Now()

	messages := []Message{
		{Role: "system", Content: e.getSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	chatResp, err := e.llm.Chat(ctx, messages)
	llmDuration := time.Since(llmStart)

	thinkingStep := ReasoningStep{
		Step:       3,
		Thought:    "Generating answer with language model",
		Action:     "llm_completion",
		DurationMs: llmDuration.Milliseconds(),
		Timestamp:  time.Now(),
	}

	if err != nil {
		answer.Answer = fmt.Sprintf("I encountered an error while generating the answer: %v", err)
		answer.Confidence = 0
		thinkingStep.Observation = fmt.Sprintf("LLM error: %v", err)
		answer.ThinkingChain = thinkingSteps
		return answer, err
	}

	if chatResp == nil {
		answer.Answer = "I encountered an error while generating the answer: empty response"
		answer.Confidence = 0
		answer.ThinkingChain = thinkingSteps
		return answer, fmt.Errorf("wiki/qa: empty LLM response")
	}

	answer.Answer = chatResp.Content
	tokensUsed := chatResp.Usage.TotalTokens
	answer.TokensUsed = tokensUsed
	thinkingStep.Observation = fmt.Sprintf("Generated answer in %dms, used %d tokens", llmDuration.Milliseconds(), tokensUsed)
	thinkingSteps = append(thinkingSteps, thinkingStep)

	// Step 4: Extract and verify citations
	answer.Citations = e.extractCitations(answer.Answer, chunks)
	if e.citationVerifier != nil && len(answer.Citations) > 0 {
		verifyStart := time.Now()
		verifiedCitations, groundingScore, err := e.citationVerifier.Verify(ctx, answer.Answer, answer.Citations, chunks)
		if err == nil {
			answer.Citations = verifiedCitations
			answer.GroundingScore = groundingScore
		}
		thinkingSteps = append(thinkingSteps, ReasoningStep{
			Step:       4,
			Thought:    "Verifying citations and grounding",
			Action:     "citation_verification",
			DurationMs: time.Since(verifyStart).Milliseconds(),
			Observation: fmt.Sprintf("Verified %d citations, grounding score: %.2f", len(verifiedCitations), groundingScore),
			Timestamp:  time.Now(),
		})
	}

	// Step 5: Compute confidence score
	if e.confidenceScorer != nil {
		confStart := time.Now()
		confidence, err := e.confidenceScorer.Score(ctx, answer.Answer, retrievalResp.Results)
		if err == nil {
			answer.Confidence = e.confidenceScorer.Calibrate(ctx, confidence, map[string]interface{}{
				"num_citations":     len(answer.Citations),
				"grounding_score":   answer.GroundingScore,
				"retrieval_score":   getAvgRetrievalScore(retrievalResp.Results),
			})
		}
		thinkingSteps = append(thinkingSteps, ReasoningStep{
			Step:       5,
			Thought:    "Computing answer confidence",
			Action:     "confidence_scoring",
			DurationMs: time.Since(confStart).Milliseconds(),
			Observation: fmt.Sprintf("Final confidence score: %.2f", answer.Confidence),
			Timestamp:  time.Now(),
		})
	} else {
		// Simple heuristic confidence if no scorer provided
		answer.Confidence = e.estimateConfidence(retrievalResp.Results, answer.Citations, answer.GroundingScore)
	}

	// Include thinking chain if available
	if chatResp.Thinking != "" {
		answer.ThinkingChain = append(answer.ThinkingChain, ReasoningStep{
			Step:      0,
			Thought:   "Model reasoning",
			Action:    "reasoning",
			Observation: chatResp.Thinking,
			Timestamp: time.Now(),
		})
	}
	answer.ThinkingChain = append(answer.ThinkingChain, thinkingSteps...)

	totalDuration := time.Since(startTime)
	logger.Infof(ctx, "[wiki/qa] answered question: confidence=%.2f, citations=%d, tokens=%d, duration=%dms",
		answer.Confidence, len(answer.Citations), answer.TokensUsed, totalDuration.Milliseconds())

	return answer, nil
}

func (e *QAEngine) getSystemPrompt() string {
	return `You are XinWiki's high-precision question answering assistant. Your task is to answer questions based on the provided wiki context.

Rules:
1. Only answer based on the provided context. If the answer is not in the context, say you don't know.
2. Cite your sources using [citation:N] format where N is the citation number.
3. Be accurate, concise, and helpful.
4. If multiple sources provide relevant information, synthesize them.
5. Do not make up information or use external knowledge beyond what's provided.
6. When uncertain, acknowledge the uncertainty rather than guessing.

Citation format:
- After each claim supported by a source, add [citation:N] where N corresponds to the source number.
- Example: "XinWiki supports hybrid retrieval combining BM25 and vector search [citation:1][citation:3]."`
}

func (e *QAEngine) buildQAPrompt(question, context string) string {
	return fmt.Sprintf(`Please answer the following question based on the provided wiki context.

## Wiki Context:
%s

## Question:
%s

## Instructions:
- Answer only based on the context above
- Cite sources using [citation:N] format
- If the answer cannot be found in the context, clearly state that
- Be concise but comprehensive
- Include relevant technical details when appropriate`, context, question)
}

func (e *QAEngine) buildContext(chunks []*Chunk) string {
	var sb strings.Builder
	for i, chunk := range chunks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		source := chunk.Path
		if source == "" {
			source = chunk.Section
		}
		if source == "" {
			source = "Wiki content"
		}
		sb.WriteString(fmt.Sprintf("--- Source %d: %s ---\n", i+1, source))
		sb.WriteString(chunk.Content)
	}
	return sb.String()
}

func (e *QAEngine) extractCitations(answer string, chunks []*Chunk) []*Citation {
	// Simple citation extraction - looks for [citation:N] patterns
	citations := make([]*Citation, 0)
	citationMap := make(map[int]bool)

	for i, chunk := range chunks {
		marker := fmt.Sprintf("[citation:%d]", i+1)
		if strings.Contains(answer, marker) && !citationMap[i+1] {
			citationMap[i+1] = true
			citations = append(citations, &Citation{
				ID:          generateID(),
				ChunkID:     chunk.ID,
				WikiPageID:  chunk.WikiPageID,
				Content:     chunk.Content,
				Section:     chunk.Section,
				Path:        chunk.Path,
				Confidence:  0.8,
			})
		}
	}

	// If no explicit citations but we have chunks, include top ones
	if len(citations) == 0 && len(chunks) > 0 {
		limit := 3
		if len(chunks) < limit {
			limit = len(chunks)
		}
		for i := 0; i < limit; i++ {
			citations = append(citations, &Citation{
				ID:          generateID(),
				ChunkID:     chunks[i].ID,
				WikiPageID:  chunks[i].WikiPageID,
				Content:     chunks[i].Content,
				Section:     chunks[i].Section,
				Path:        chunks[i].Path,
				Confidence:  chunks[i].Score,
			})
		}
	}

	return citations
}

func (e *QAEngine) estimateConfidence(results []*SearchResult, citations []*Citation, groundingScore float64) float64 {
	if len(results) == 0 {
		return 0.1
	}

	avgScore := getAvgRetrievalScore(results)
	retrievalFactor := avgScore * 0.4
	citationFactor := math_min(float64(len(citations))/3.0, 1.0) * 0.3
	groundingFactor := groundingScore * 0.3

	confidence := retrievalFactor + citationFactor + groundingFactor
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0 {
		confidence = 0
	}
	return confidence
}

func getAvgRetrievalScore(results []*SearchResult) float64 {
	if len(results) == 0 {
		return 0
	}
	total := 0.0
	for _, r := range results {
		total += r.FinalScore
	}
	return total / float64(len(results))
}

func math_min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// RelatedSearches generates suggested follow-up questions.
func (e *QAEngine) RelatedSearches(ctx context.Context, question string, answer *Answer) []string {
	// In a production implementation, this would use the LLM to generate related questions
	return []string{
		"Can you explain this in more detail?",
		"What are the key concepts mentioned?",
		"How does this relate to other topics?",
	}
}

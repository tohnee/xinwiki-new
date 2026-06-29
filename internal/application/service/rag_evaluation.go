package service

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// ragEvaluationService implements RAGEvaluationService
type ragEvaluationService struct {
	evalRepo    interfaces.EvaluationRepository
	chunkRepo   interfaces.ChunkRepository
	llmService  interfaces.LLMService
	modelRouter interfaces.ModelRouterService
}

// NewRAGEvaluationService creates a new RAG evaluation service
func NewRAGEvaluationService(
	evalRepo interfaces.EvaluationRepository,
	chunkRepo interfaces.ChunkRepository,
	llmService interfaces.LLMService,
	modelRouter interfaces.ModelRouterService,
) interfaces.RAGEvaluationService {
	return &ragEvaluationService{
		evalRepo:    evalRepo,
		chunkRepo:   chunkRepo,
		llmService:  llmService,
		modelRouter: modelRouter,
	}
}

// EvaluateCitationAccuracy evaluates the accuracy of citations in a RAG response
func (s *ragEvaluationService) EvaluateCitationAccuracy(
	ctx context.Context,
	req *types.CitationEvaluationRequest,
) (*types.CitationAccuracyReport, error) {
	startTime := time.Now()

	report := &types.CitationAccuracyReport{
		TenantID: req.TenantID,
		KBID:     req.KBID,
		Query:    req.Query,
		Response: req.Response,
		Status:   types.EvaluationStatusRunning,
	}

	if err := s.evalRepo.CreateReport(ctx, report); err != nil {
		logger.Warnf(ctx, "[RAGEval] Failed to create initial report: %v", err)
	}

	logger.Infof(ctx, "[RAGEval] Starting citation accuracy evaluation for tenant=%d kb=%s",
		req.TenantID, req.KBID)

	citations := req.Citations
	if len(citations) == 0 {
		chunks := make([]*types.Chunk, len(req.ContextChunks))
		for i := range req.ContextChunks {
			chunks[i] = &req.ContextChunks[i]
		}
		extracted, err := s.ExtractCitations(ctx, req.Response, chunks)
		if err != nil {
			report.Status = types.EvaluationStatusFailed
			report.ErrorMsg = fmt.Sprintf("failed to extract citations: %v", err)
			_ = s.evalRepo.UpdateReport(ctx, report)
			return nil, fmt.Errorf("failed to extract citations: %w", err)
		}
		citations = extracted
	}

	report.Citations = citations

	chunkMap := make(map[string]*types.Chunk)
	if len(req.ContextChunks) > 0 {
		for i := range req.ContextChunks {
			chunk := &req.ContextChunks[i]
			if chunk.ID != "" {
				chunkMap[chunk.ID] = chunk
			}
		}
	}

	if len(citations) > 0 {
		chunkIDs := make([]string, 0)
		for _, c := range citations {
			if c.ChunkID != "" {
				chunkIDs = append(chunkIDs, c.ChunkID)
			}
		}
		if len(chunkIDs) > 0 {
			for _, cid := range chunkIDs {
				if _, exists := chunkMap[cid]; !exists {
					chunk, err := s.chunkRepo.GetChunkByID(ctx, req.TenantID, cid)
					if err == nil && chunk != nil {
						chunkMap[cid] = chunk
					}
				}
			}
		}
	}

	evaluations := make([]types.CitationEvaluation, len(citations))

	for i, citation := range citations {
		var chunk *types.Chunk
		if citation.ChunkID != "" {
			chunk = chunkMap[citation.ChunkID]
		}

		if chunk == nil {
			evaluations[i] = types.CitationEvaluation{
				Citation:   citation,
				Status:     types.CitationStatusMissing,
				Confidence: 1.0,
				IsRelevant: false,
				Reasoning:  "Source chunk not found or not provided in context",
			}
			continue
		}

		eval, err := s.VerifyCitation(ctx, citation.Text, chunk.Content)
		if err != nil {
			logger.Warnf(ctx, "[RAGEval] Failed to verify citation %s: %v", citation.ID, err)
			evaluations[i] = types.CitationEvaluation{
				Citation:   citation,
				Status:     types.CitationStatusUnverified,
				Confidence: 0.0,
				Reasoning:  fmt.Sprintf("Verification failed: %v", err),
			}
			continue
		}

		eval.Citation = citation
		evaluations[i] = *eval
	}

	report.Evaluations = evaluations
	report.CalculateScores()
	report.Summary = s.generateSummary(report)
	report.Status = types.EvaluationStatusCompleted
	report.DurationMs = time.Since(startTime).Milliseconds()

	if err := s.evalRepo.UpdateReport(ctx, report); err != nil {
		logger.Warnf(ctx, "[RAGEval] Failed to update report: %v", err)
	}

	logger.Infof(ctx, "[RAGEval] Evaluation completed: precision=%.3f groundedness=%.3f hallucination=%.3f score=%.3f duration=%dms",
		report.Precision, report.GroundednessScore, report.HallucinationRate, report.OverallScore, report.DurationMs)

	return report, nil
}

// EvaluateBatch evaluates multiple queries in batch
func (s *ragEvaluationService) EvaluateBatch(
	ctx context.Context,
	req *types.BatchEvaluationRequest,
) (*types.BatchEvaluationResult, error) {
	startTime := time.Now()
	result := &types.BatchEvaluationResult{
		TotalQueries: len(req.Queries),
		CreatedAt:    startTime,
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5)

	for _, query := range req.Queries {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(q types.CitationEvaluationRequest) {
			defer wg.Done()
			defer func() { <-semaphore }()

			q.TenantID = req.TenantID
			q.KBID = req.KBID

			report, err := s.EvaluateCitationAccuracy(ctx, &q)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.FailedQueries++
				logger.Warnf(ctx, "[RAGEval] Batch query failed: %v", err)
				return
			}

			result.CompletedQueries++
			result.Reports = append(result.Reports, report)
		}(query)
	}

	wg.Wait()

	if result.CompletedQueries > 0 {
		totalPrecision := 0.0
		totalRecall := 0.0
		totalF1 := 0.0
		totalGroundedness := 0.0
		totalHallucination := 0.0

		for _, r := range result.Reports {
			totalPrecision += r.Precision
			totalRecall += r.Recall
			totalF1 += r.F1Score
			totalGroundedness += r.GroundednessScore
			totalHallucination += r.HallucinationRate
		}

		n := float64(result.CompletedQueries)
		result.AvgPrecision = totalPrecision / n
		result.AvgRecall = totalRecall / n
		result.AvgF1 = totalF1 / n
		result.AvgGroundedness = totalGroundedness / n
		result.AvgHallucination = totalHallucination / n
	}

	result.DurationMs = time.Since(startTime).Milliseconds()

	return result, nil
}

// GetReport retrieves a citation accuracy report by ID
func (s *ragEvaluationService) GetReport(
	ctx context.Context,
	tenantID uint64,
	reportID string,
) (*types.CitationAccuracyReport, error) {
	return s.evalRepo.GetReport(ctx, tenantID, reportID)
}

// ListReports lists citation accuracy reports with filtering
func (s *ragEvaluationService) ListReports(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	from, to time.Time,
	page, pageSize int,
) ([]*types.CitationAccuracyReport, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if from.IsZero() {
		from = time.Now().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}
	return s.evalRepo.ListReports(ctx, tenantID, kbID, from, to, page, pageSize)
}

// GetEvaluationSummary returns aggregate evaluation metrics
func (s *ragEvaluationService) GetEvaluationSummary(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	from, to time.Time,
) (*types.EvaluationSummary, error) {
	if from.IsZero() {
		from = time.Now().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now()
	}

	precision, recall, f1, groundedness, hallucination, count, err := s.evalRepo.GetAggregateMetrics(
		ctx, tenantID, kbID, from, to,
	)
	if err != nil {
		return nil, err
	}

	recentReports, _, err := s.evalRepo.ListReports(ctx, tenantID, kbID, from, to, 1, 5)
	if err != nil {
		logger.Warnf(ctx, "[RAGEval] Failed to get recent reports: %v", err)
	}

	summary := &types.EvaluationSummary{
		TotalEvaluations: count,
		AvgPrecision:     precision,
		AvgRecall:        recall,
		AvgF1:            f1,
		AvgGroundedness:  groundedness,
		AvgHallucination: hallucination,
		RecentEvaluations: recentReports,
	}

	return summary, nil
}

// ExtractCitations extracts citations from a response text
func (s *ragEvaluationService) ExtractCitations(
	ctx context.Context,
	response string,
	chunks []*types.Chunk,
) ([]types.Citation, error) {
	var citations []types.Citation

	citationPattern := regexp.MustCompile(`\[(\d+)\]|\[\[([^\]]+)\]\]|\(source:\s*([^)]+)\)|<sup>(\d+)</sup>`)
	matches := citationPattern.FindAllStringSubmatchIndex(response, -1)

	citationMarkers := make(map[int]string)
	for _, match := range matches {
		fullStart := match[0]
		fullEnd := match[1]

		var refID string
		for group := 1; group < len(match); group += 2 {
			if match[group] != -1 {
				refID = response[match[group]:match[group+1]]
				break
			}
		}

		if refID == "" {
			continue
		}

		textStart := fullStart
		sentenceStart := textStart
		for i := textStart - 1; i >= 0; i-- {
			if response[i] == '.' || response[i] == '!' || response[i] == '?' || response[i] == '\n' {
				sentenceStart = i + 1
				break
			}
			if i == 0 {
				sentenceStart = 0
			}
		}

		text := strings.TrimSpace(response[sentenceStart:fullEnd])

		citation := types.Citation{
			ID:       fmt.Sprintf("cite_%d", len(citations)+1),
			Text:     text,
			StartPos: sentenceStart,
			EndPos:   fullEnd,
		}

		citationMarkers[len(citations)] = refID
		citations = append(citations, citation)
	}

	if len(citations) == 0 && len(chunks) > 0 {
		for i, chunk := range chunks {
			if chunk.Content == "" {
				continue
			}
			citations = append(citations, types.Citation{
				ID:      fmt.Sprintf("chunk_%d", i+1),
				Text:    response,
				ChunkID: chunk.ID,
				DocID:   chunk.DocumentID,
				Content: chunk.Content,
			})
		}
	}

	for i := range citations {
		if citations[i].ChunkID == "" {
			idx := i
			refID := citationMarkers[idx]

			if numIdx, err := parseInt(refID); err == nil && numIdx >= 1 && numIdx <= len(chunks) {
				chunk := chunks[numIdx-1]
				citations[i].ChunkID = chunk.ID
				citations[i].DocID = chunk.DocumentID
				citations[i].Content = chunk.Content
			}
		}
	}

	return citations, nil
}

// VerifyCitation verifies if a single citation is supported by the source chunk
func (s *ragEvaluationService) VerifyCitation(
	ctx context.Context,
	claim string,
	sourceContent string,
) (*types.CitationEvaluation, error) {
	claim = strings.TrimSpace(claim)
	sourceContent = strings.TrimSpace(sourceContent)

	if claim == "" || sourceContent == "" {
		return &types.CitationEvaluation{
			Status:        types.CitationStatusUnverified,
			Confidence:    0.0,
			IsRelevant:    false,
			SupportsClaim: false,
			Reasoning:     "Empty claim or source content",
		}, nil
	}

	claimLower := strings.ToLower(claim)
	sourceLower := strings.ToLower(sourceContent)

	claimWords := tokenize(claimLower)
	sourceWords := tokenize(sourceLower)

	if len(claimWords) == 0 {
		return &types.CitationEvaluation{
			Status:        types.CitationStatusUnverified,
			Confidence:    0.0,
			IsRelevant:    false,
			SupportsClaim: false,
			Reasoning:     "No meaningful words in claim",
		}, nil
	}

	overlap := 0
	for _, w := range claimWords {
		if len(w) < 3 {
			continue
		}
		if containsWord(sourceWords, w) {
			overlap++
		}
	}

	overlapRatio := 0.0
	meaningfulWords := 0
	for _, w := range claimWords {
		if len(w) >= 3 {
			meaningfulWords++
		}
	}
	if meaningfulWords > 0 {
		overlapRatio = float64(overlap) / float64(meaningfulWords)
	}

	evaluation := &types.CitationEvaluation{
		IsRelevant: overlapRatio > 0.2,
		RelevantSnippets: extractRelevantSnippets(sourceContent, claimWords, 200),
	}

	negationWords := []string{"not", "no", "never", "none", "neither", "nor", "cannot", "doesn't", "don't", "won't", "isn't", "aren't", "wasn't", "weren't", "haven't", "hasn't", "hadn't"}

	hasNegationInClaim := false
	for _, neg := range negationWords {
		if strings.Contains(claimLower, " "+neg+" ") || strings.HasPrefix(claimLower, neg+" ") {
			hasNegationInClaim = true
			break
		}
	}

	hasNegationInSource := false
	for _, neg := range negationWords {
		if strings.Contains(sourceLower, " "+neg+" ") || strings.HasPrefix(sourceLower, neg+" ") {
			for _, w := range claimWords {
				if len(w) >= 3 {
					idx := strings.Index(sourceLower, w)
					if idx >= 0 {
						negIdx := strings.Index(sourceLower, " "+neg+" ")
						if negIdx == -1 {
							negIdx = strings.Index(sourceLower, neg+" ")
						}
						if negIdx >= 0 && math.Abs(float64(negIdx-idx)) < 100 {
							hasNegationInSource = true
							break
						}
					}
				}
			}
			if hasNegationInSource {
				break
			}
		}
	}

	evaluation.SupportsClaim = true

	if overlapRatio < 0.2 {
		evaluation.Status = types.CitationStatusMissing
		evaluation.Confidence = 0.9
		evaluation.Reasoning = "Claim has minimal word overlap with source content; likely hallucination or wrong citation"
		evaluation.SupportsClaim = false
	} else if overlapRatio >= 0.6 && (!hasNegationInClaim || !hasNegationInSource) {
		evaluation.Status = types.CitationStatusSupported
		evaluation.Confidence = overlapRatio
		evaluation.Reasoning = "Claim is well-supported by the source content with high lexical overlap"
	} else if overlapRatio >= 0.3 {
		evaluation.Status = types.CitationStatusPartial
		evaluation.Confidence = overlapRatio
		evaluation.Reasoning = "Claim has partial support in the source; some elements are mentioned but may require inference"
	} else if hasNegationInClaim != hasNegationInSource {
		evaluation.Status = types.CitationStatusContradicted
		evaluation.Confidence = 0.7
		evaluation.Reasoning = "Claim may be contradicted by negation in source content"
		evaluation.SupportsClaim = false
	} else {
		evaluation.Status = types.CitationStatusUnverified
		evaluation.Confidence = 0.3
		evaluation.Reasoning = "Could not fully verify; lexical overlap is low and semantic verification via LLM recommended"
	}

	return evaluation, nil
}

// Helper functions

func (s *ragEvaluationService) generateSummary(report *types.CitationAccuracyReport) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Evaluated %d citations.", report.TotalCitations))

	if report.SupportedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d fully supported", report.SupportedCount))
	}
	if report.PartialCount > 0 {
		parts = append(parts, fmt.Sprintf("%d partially supported", report.PartialCount))
	}
	if report.ContradictedCount > 0 {
		parts = append(parts, fmt.Sprintf("%d contradicted", report.ContradictedCount))
	}
	if report.MissingCount > 0 {
		parts = append(parts, fmt.Sprintf("%d missing source", report.MissingCount))
	}

	parts = append(parts, fmt.Sprintf("Groundedness: %.1f%%", report.GroundednessScore*100))
	parts = append(parts, fmt.Sprintf("Hallucination risk: %.1f%%", report.HallucinationRate*100))

	if report.OverallScore >= 0.8 {
		parts = append(parts, "Overall: Excellent citation quality.")
	} else if report.OverallScore >= 0.6 {
		parts = append(parts, "Overall: Good citation quality with minor issues.")
	} else if report.OverallScore >= 0.4 {
		parts = append(parts, "Overall: Acceptable but needs improvement.")
	} else {
		parts = append(parts, "Overall: Poor citation quality; recommend review and improved retrieval.")
	}

	return strings.Join(parts, " ")
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-')
	})
	var result []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

func containsWord(words []string, target string) bool {
	idx := sort.SearchStrings(words, target)
	return idx < len(words) && words[idx] == target
}

func extractRelevantSnippets(content string, keywords []string, maxLen int) []string {
	var snippets []string
	contentLower := strings.ToLower(content)

	for _, kw := range keywords {
		if len(kw) < 4 {
			continue
		}
		idx := strings.Index(contentLower, kw)
		if idx < 0 {
			continue
		}

		start := idx - 50
		if start < 0 {
			start = 0
		}
		end := idx + len(kw) + 100
		if end > len(content) {
			end = len(content)
		}

		snippet := strings.TrimSpace(content[start:end])
		if start > 0 {
			snippet = "..." + snippet
		}
		if end < len(content) {
			snippet = snippet + "..."
		}

		snippets = append(snippets, snippet)

		if len(snippets) >= 3 {
			break
		}
	}

	return snippets
}

func parseInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

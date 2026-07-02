package wiki

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
)

// NewHybridRetriever creates a new hybrid retrieval engine.
func NewHybridRetriever(
	bm25 BM25Retriever,
	vector VectorRetriever,
	graph GraphRetriever,
	queryRewriter QueryRewriter,
	reranker Reranker,
	cacheTTL time.Duration,
	cacheMaxEntries int,
) *HybridRetriever {
	if reranker == nil {
		reranker = NewNoopReranker()
	}
	return &HybridRetriever{
		bm25Retriever:  bm25,
		vectorRetriever: vector,
		graphRetriever: graph,
		cache:          NewRetrievalCache(cacheTTL, cacheMaxEntries),
		queryRewriter:  queryRewriter,
		reranker:       reranker,
	}
}

// Retrieve performs hybrid search combining multiple methods with RRF fusion.
func (h *HybridRetriever) Retrieve(ctx context.Context, req *RetrievalRequest) (*RetrievalResponse, error) {
	traceID := logger.TraceID(ctx)
	startTime := time.Now()
	logger.Infof(ctx, "[wiki:%s] === HYBRID RETRIEVAL START ===", traceID)
	logger.Infof(ctx, "[wiki:%s] query='%s', topK=%d, methods=%v, minScore=%.3f, useRRF=%v, useCache=%v, kbs=%v",
		traceID, req.Query, req.TopK, req.Methods, req.MinScore, req.UseRRF, req.UseCache, req.KnowledgeBaseIDs)

	if req.TopK <= 0 {
		req.TopK = 10
	}
	if req.RRFConstant <= 0 {
		req.RRFConstant = 60 // Standard RRF k value
	}
	if len(req.Methods) == 0 {
		req.Methods = []RetrievalMethod{MethodBM25, MethodVector}
	}

	// Check cache first
	cacheCheckStart := time.Now()
	if req.UseCache {
		cacheKey := h.buildCacheKey(req)
		if cached := h.cache.Get(cacheKey); cached != nil {
			cached.DurationMs = time.Since(startTime).Milliseconds()
			logger.Infof(ctx, "[wiki:%s] CACHE HIT, key=%s, duration=%dms, results=%d",
				traceID, cacheKey, cached.DurationMs, cached.TotalChunks)
			logger.Infof(ctx, "[wiki:%s] === HYBRID RETRIEVAL END (cached) ===", traceID)
			return cached, nil
		}
		logger.Debugf(ctx, "[wiki:%s] cache miss, key=%s, check_duration=%dμs",
			traceID, cacheKey, time.Since(cacheCheckStart).Microseconds())
	}

	// Query rewriting for better retrieval (history-aware when provided).
	rewriteStart := time.Now()
	var expandedQueries []string
	var entities []string
	if h.queryRewriter != nil {
		var rewrite *QueryRewrite
		var err error
		if len(req.History) > 0 {
			rewrite, err = h.queryRewriter.RewriteWithContext(ctx, req.Query, req.History)
		} else {
			rewrite, err = h.queryRewriter.Rewrite(ctx, req.Query)
		}
		rewriteDuration := time.Since(rewriteStart).Microseconds()
		if err == nil && rewrite != nil {
			expandedQueries = rewrite.ExpandedQueries
			entities = rewrite.Entities
			logger.Infof(ctx, "[wiki:%s] query rewrite completed in %dμs: expanded=%d queries, entities=%v",
				traceID, rewriteDuration, len(expandedQueries), entities)
		} else if err != nil {
			logger.Warnf(ctx, "[wiki:%s] query rewrite failed after %dμs: %v", traceID, rewriteDuration, err)
		}
	} else {
		logger.Debugf(ctx, "[wiki:%s] query rewriter not configured, skipping", traceID)
	}

	// Determine which retrievers to use
	useBM25 := containsMethod(req.Methods, MethodBM25) || containsMethod(req.Methods, MethodHybrid)
	useVector := containsMethod(req.Methods, MethodVector) || containsMethod(req.Methods, MethodHybrid)
	useGraph := (containsMethod(req.Methods, MethodGraph) || containsMethod(req.Methods, MethodHybrid)) && h.graphRetriever != nil && len(entities) > 0

	logger.Debugf(ctx, "[wiki:%s] retrievers enabled: bm25=%v, vector=%v, graph=%v",
		traceID, useBM25, useVector, useGraph)

	// Run retrievers in parallel
	var (
		bm25Results   []*SearchResult
		vectorResults []*SearchResult
		graphResults  []*SearchResult
		bm25Err, vectorErr, graphErr error
		wg            sync.WaitGroup
		bm25Duration, vectorDuration, graphDuration int64
	)

	queries := append([]string{req.Query}, expandedQueries...)
	retrievalStart := time.Now()

	if useBM25 && h.bm25Retriever != nil {
		wg.Add(1)
		bm25Start := time.Now()
		go func() {
			defer wg.Done()
			defer func() { bm25Duration = time.Since(bm25Start).Milliseconds() }()
			allResults := make([]*SearchResult, 0)
			for _, q := range queries {
				qStart := time.Now()
				results, err := h.bm25Retriever.Search(ctx, q, req.KnowledgeBaseIDs, req.TopK*2, req.Filters)
				qDuration := time.Since(qStart).Milliseconds()
				if err != nil {
					bm25Err = err
					logger.Warnf(ctx, "[wiki:%s] BM25 search failed for query '%s' after %dms: %v",
						traceID, truncate(q, 50), qDuration, err)
					continue
				}
				logger.Debugf(ctx, "[wiki:%s] BM25 subquery '%s' returned %d results in %dms",
					traceID, truncate(q, 50), len(results), qDuration)
				allResults = mergeSearchResults(allResults, results)
			}
			bm25Results = allResults
		}()
	}

	if useVector && h.vectorRetriever != nil {
		wg.Add(1)
		vectorStart := time.Now()
		go func() {
			defer wg.Done()
			defer func() { vectorDuration = time.Since(vectorStart).Milliseconds() }()
			embeddingStart := time.Now()
			// Vector search uses embeddings - embedding generation handled by vector retriever implementation
			results, err := h.vectorRetriever.Search(ctx, nil, req.KnowledgeBaseIDs, req.TopK*2, req.Filters)
			vectorSearchDuration := time.Since(vectorStart).Milliseconds()
			embeddingMs := time.Since(embeddingStart).Milliseconds()
			if err != nil {
				vectorErr = err
				logger.Warnf(ctx, "[wiki:%s] Vector search failed after %dms (embedding=%dms): %v",
					traceID, vectorSearchDuration, embeddingMs, err)
				return
			}
			vectorResults = results
			logger.Debugf(ctx, "[wiki:%s] Vector search returned %d results in %dms (embedding=%dms)",
				traceID, len(results), vectorSearchDuration, embeddingMs)
		}()
	}

	if useGraph && h.graphRetriever != nil {
		wg.Add(1)
		graphStart := time.Now()
		go func() {
			defer wg.Done()
			defer func() { graphDuration = time.Since(graphStart).Milliseconds() }()
			results, err := h.graphRetriever.Search(ctx, entities, req.KnowledgeBaseIDs, req.TopK*2, 2)
			graphSearchDuration := time.Since(graphStart).Milliseconds()
			if err != nil {
				graphErr = err
				logger.Warnf(ctx, "[wiki:%s] Graph search failed after %dms: %v",
					traceID, graphSearchDuration, err)
				return
			}
			graphResults = results
			logger.Debugf(ctx, "[wiki:%s] Graph search returned %d results in %dms",
				traceID, len(results), graphSearchDuration)
		}()
	}

	wg.Wait()
	parallelRetrievalDuration := time.Since(retrievalStart).Milliseconds()
	logger.Infof(ctx, "[wiki:%s] parallel retrieval completed: bm25=%d results in %dms, vector=%d results in %dms, graph=%d results in %dms, total_parallel_time=%dms",
		traceID,
		len(bm25Results), bm25Duration,
		len(vectorResults), vectorDuration,
		len(graphResults), graphDuration,
		parallelRetrievalDuration)

	// Combine results using RRF or score-based fusion
	fusionStart := time.Now()
	var finalResults []*SearchResult
	if req.UseRRF {
		finalResults = h.reciprocalRankFusion(bm25Results, vectorResults, graphResults, req.RRFConstant)
	} else {
		finalResults = h.scoreFusion(bm25Results, vectorResults, graphResults)
	}
	fusionDuration := time.Since(fusionStart).Microseconds()
	logger.Debugf(ctx, "[wiki:%s] result fusion (%s) completed in %dμs, raw_fused=%d results",
		traceID, map[bool]string{true: "RRF", false: "weighted"}[req.UseRRF], fusionDuration, len(finalResults))

	// Sort by fused score descending before handing off to reranker (so the
	// reranker sees the strongest fusion candidates first when it caps input).
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].FinalScore > finalResults[j].FinalScore
	})

	// Cross-encoder style reranking. The reranker may down-rank irrelevant
	// fusion matches and reorders the top-N; it falls back to a noop when
	// unconfigured or on error.
	rerankStart := time.Now()
	if h.reranker != nil && len(finalResults) > 0 {
		rerankTopN := 0
		const rerankFloor = 50
		if req.TopK > 0 {
			// Rerank a larger window than the final TopK so reordering has
			// room to surface good results that the fusion ranked lower.
			rerankTopN = req.TopK * 3
			if rerankTopN < rerankFloor {
				rerankTopN = rerankFloor
			}
		}
		reranked, rerr := h.reranker.Rerank(ctx, req.Query, finalResults, rerankTopN)
		rerankDuration := time.Since(rerankStart).Microseconds()
		if rerr != nil {
			logger.Warnf(ctx, "[wiki:%s] rerank failed after %dμs: %v; keeping fusion order", traceID, rerankDuration, rerr)
		} else {
			finalResults = reranked
			logger.Debugf(ctx, "[wiki:%s] rerank completed in %dμs, results=%d", traceID, rerankDuration, len(finalResults))
		}
	}

	// Filter by minimum score
	if req.MinScore > 0 {
		filtered := make([]*SearchResult, 0, len(finalResults))
		for _, r := range finalResults {
			if r.FinalScore >= req.MinScore {
				filtered = append(filtered, r)
			}
		}
		logger.Debugf(ctx, "[wiki:%s] min_score filter: %.3f, before=%d, after=%d",
			traceID, req.MinScore, len(finalResults), len(filtered))
		finalResults = filtered
	}

	// Sort by final score descending (reranker already sorts, but keep this
	// as a safety net for the noop path).
	sortStart := time.Now()
	sort.Slice(finalResults, func(i, j int) bool {
		return finalResults[i].FinalScore > finalResults[j].FinalScore
	})
	sortDuration := time.Since(sortStart).Microseconds()

	// Limit to TopK
	if req.TopK > 0 && len(finalResults) > req.TopK {
		finalResults = finalResults[:req.TopK]
	}

	// Assign ranks
	for i, r := range finalResults {
		r.Rank = i + 1
	}
	totalDuration := time.Since(startTime).Milliseconds()

	// Log top results for debugging
	if len(finalResults) > 0 {
		logger.Debugf(ctx, "[wiki:%s] top 3 results:", traceID)
		for i := 0; i < min(3, len(finalResults)); i++ {
			r := finalResults[i]
			logger.Debugf(ctx, "[wiki:%s]   #%d score=%.4f page=%s section=%s path=%s",
				traceID, i+1, r.FinalScore, r.Chunk.WikiPageID, r.Chunk.Section, truncate(r.Chunk.Path, 40))
		}
	}

	response := &RetrievalResponse{
		Results:     finalResults,
		Query:       req.Query,
		TotalChunks: len(finalResults),
		DurationMs:  totalDuration,
		Method:      h.determineMethod(req.Methods),
		CacheHit:    false,
	}

	// Cache the result
	if req.UseCache {
		cacheKey := h.buildCacheKey(req)
		h.cache.Set(cacheKey, response)
	}

	logger.Infof(ctx, "[wiki:%s] performance breakdown: rewrite=%dμs, retrieval=%dms (bm25=%dms/vector=%dms/graph=%dms), fusion=%dμs, sort=%dμs, total=%dms",
		traceID,
		time.Since(rewriteStart).Microseconds(),
		parallelRetrievalDuration,
		bm25Duration,
		vectorDuration,
		graphDuration,
		fusionDuration,
		sortDuration,
		totalDuration)
	logger.Infof(ctx, "[wiki:%s] === HYBRID RETRIEVAL END: results=%d, total=%dms, errors=(bm25=%v, vector=%v, graph=%v) ===",
		traceID, len(finalResults), totalDuration, bm25Err, vectorErr, graphErr)

	return response, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// reciprocalRankFusion implements the RRF algorithm for combining ranked lists.
// RRF score = sum(1/(k + rank_i)) for each retriever i
func (h *HybridRetriever) reciprocalRankFusion(bm25Results, vectorResults, graphResults []*SearchResult, k int) []*SearchResult {
	rrfScores := make(map[string]*SearchResult)

	// Process BM25 results
	for rank, result := range bm25Results {
		chunkID := result.Chunk.ID
		rrfScore := 1.0 / float64(k+rank+1)
		if existing, ok := rrfScores[chunkID]; ok {
			existing.FinalScore += rrfScore
			existing.BM25Score = result.BM25Score
			if result.Chunk.ScoreBreakdown == nil {
				result.Chunk.ScoreBreakdown = make(map[RetrievalMethod]float64)
			}
			result.Chunk.ScoreBreakdown[MethodBM25] = rrfScore
		} else {
			result.FinalScore = rrfScore
			result.BM25Score = rrfScore
			if result.Chunk.ScoreBreakdown == nil {
				result.Chunk.ScoreBreakdown = make(map[RetrievalMethod]float64)
			}
			result.Chunk.ScoreBreakdown[MethodBM25] = rrfScore
			rrfScores[chunkID] = result
		}
	}

	// Process Vector results
	for rank, result := range vectorResults {
		chunkID := result.Chunk.ID
		rrfScore := 1.0 / float64(k+rank+1)
		if existing, ok := rrfScores[chunkID]; ok {
			existing.FinalScore += rrfScore
			existing.VectorScore = rrfScore
			existing.Chunk.ScoreBreakdown[MethodVector] = rrfScore
		} else {
			result.FinalScore = rrfScore
			result.VectorScore = rrfScore
			if result.Chunk.ScoreBreakdown == nil {
				result.Chunk.ScoreBreakdown = make(map[RetrievalMethod]float64)
			}
			result.Chunk.ScoreBreakdown[MethodVector] = rrfScore
			rrfScores[chunkID] = result
		}
	}

	// Process Graph results
	for rank, result := range graphResults {
		chunkID := result.Chunk.ID
		rrfScore := 1.0 / float64(k+rank+1) * 0.8 // Slightly lower weight for graph
		if existing, ok := rrfScores[chunkID]; ok {
			existing.FinalScore += rrfScore
			existing.GraphScore = rrfScore
			existing.Chunk.ScoreBreakdown[MethodGraph] = rrfScore
		} else {
			result.FinalScore = rrfScore
			result.GraphScore = rrfScore
			if result.Chunk.ScoreBreakdown == nil {
				result.Chunk.ScoreBreakdown = make(map[RetrievalMethod]float64)
			}
			result.Chunk.ScoreBreakdown[MethodGraph] = rrfScore
			rrfScores[chunkID] = result
		}
	}

	results := make([]*SearchResult, 0, len(rrfScores))
	for _, r := range rrfScores {
		results = append(results, r)
	}
	return results
}

// scoreFusion combines results using weighted score normalization.
func (h *HybridRetriever) scoreFusion(bm25Results, vectorResults, graphResults []*SearchResult) []*SearchResult {
	const (
		bm25Weight   = 0.3
		vectorWeight = 0.5
		graphWeight  = 0.2
	)

	combined := make(map[string]*SearchResult)

	// Normalize and combine BM25
	bm25Max := getMaxScore(bm25Results)
	for _, r := range bm25Results {
		normalized := 0.0
		if bm25Max > 0 {
			normalized = r.BM25Score / bm25Max
		}
		r.FinalScore = normalized * bm25Weight
		r.Chunk.ScoreBreakdown = map[RetrievalMethod]float64{
			MethodBM25: normalized * bm25Weight,
		}
		combined[r.Chunk.ID] = r
	}

	// Normalize and combine Vector
	vectorMax := getMaxScore(vectorResults)
	for _, r := range vectorResults {
		normalized := 0.0
		if vectorMax > 0 {
			normalized = r.VectorScore / vectorMax
		}
		if existing, ok := combined[r.Chunk.ID]; ok {
			existing.FinalScore += normalized * vectorWeight
			existing.VectorScore = normalized * vectorWeight
			existing.Chunk.ScoreBreakdown[MethodVector] = normalized * vectorWeight
		} else {
			r.FinalScore = normalized * vectorWeight
			r.VectorScore = normalized * vectorWeight
			r.Chunk.ScoreBreakdown = map[RetrievalMethod]float64{
				MethodVector: normalized * vectorWeight,
			}
			combined[r.Chunk.ID] = r
		}
	}

	// Normalize and combine Graph
	graphMax := getMaxScore(graphResults)
	for _, r := range graphResults {
		normalized := 0.0
		if graphMax > 0 {
			normalized = r.GraphScore / graphMax
		}
		if existing, ok := combined[r.Chunk.ID]; ok {
			existing.FinalScore += normalized * graphWeight
			existing.GraphScore = normalized * graphWeight
			existing.Chunk.ScoreBreakdown[MethodGraph] = normalized * graphWeight
		} else {
			r.FinalScore = normalized * graphWeight
			r.GraphScore = normalized * graphWeight
			r.Chunk.ScoreBreakdown = map[RetrievalMethod]float64{
				MethodGraph: normalized * graphWeight,
			}
			combined[r.Chunk.ID] = r
		}
	}

	results := make([]*SearchResult, 0, len(combined))
	for _, r := range combined {
		results = append(results, r)
	}
	return results
}

func (h *HybridRetriever) buildCacheKey(req *RetrievalRequest) string {
	return fmt.Sprintf("%s:%s:%v:%d:%v",
		req.TenantID,
		req.Query,
		req.KnowledgeBaseIDs,
		req.TopK,
		req.Methods,
	)
}

func (h *HybridRetriever) determineMethod(methods []RetrievalMethod) RetrievalMethod {
	if len(methods) == 1 {
		return methods[0]
	}
	return MethodHybrid
}

func mergeSearchResults(existing, new []*SearchResult) []*SearchResult {
	seen := make(map[string]bool)
	for _, r := range existing {
		seen[r.Chunk.ID] = true
	}
	for _, r := range new {
		if !seen[r.Chunk.ID] {
			existing = append(existing, r)
			seen[r.Chunk.ID] = true
		}
	}
	return existing
}

func getMaxScore(results []*SearchResult) float64 {
	max := 0.0
	for _, r := range results {
		score := r.FinalScore
		if score == 0 {
			score = r.BM25Score
		}
		if score > max {
			max = score
		}
	}
	return max
}

func containsMethod(methods []RetrievalMethod, target RetrievalMethod) bool {
	for _, m := range methods {
		if m == target {
			return true
		}
	}
	return false
}

// ExpandWithContext expands initial results with contextually related chunks.
func (h *HybridRetriever) ExpandWithContext(ctx context.Context, results []*SearchResult, depth int) ([]*SearchResult, error) {
	if h.graphRetriever == nil || depth <= 0 || len(results) == 0 {
		return results, nil
	}

	chunkIDs := make([]string, len(results))
	for i, r := range results {
		chunkIDs[i] = r.Chunk.ID
	}

	expanded, err := h.graphRetriever.Expand(ctx, chunkIDs, depth)
	if err != nil {
		return results, err
	}

	return mergeSearchResults(results, expanded), nil
}

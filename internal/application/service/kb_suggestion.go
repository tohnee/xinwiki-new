// Package service implements the KnowledgeBaseSuggestionService, which provides
// KB-level suggested questions for the NotebookLM-style "Notebook Guide" feature.
// The logic mirrors customAgentService.GetSuggestedQuestions but is scoped to a
// single knowledge base and skips the agent_config bucket (no agent context).
// Cross-tenant KB access is resolved by the caller (the handler passes the
// effective source tenant ID), so this service does not need kbShareService.
package service

import (
	"context"
	"math/rand"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// kbSuggestionService implements interfaces.KnowledgeBaseSuggestionService.
type kbSuggestionService struct {
	chunkRepo    interfaces.ChunkRepository
	wikiPageRepo interfaces.WikiPageRepository
}

// NewKnowledgeBaseSuggestionService creates a new KnowledgeBaseSuggestionService.
// Both dependencies are required: chunkRepo for FAQ + document question sources,
// wikiPageRepo for wiki page title suggestions.
func NewKnowledgeBaseSuggestionService(
	chunkRepo interfaces.ChunkRepository,
	wikiPageRepo interfaces.WikiPageRepository,
) interfaces.KnowledgeBaseSuggestionService {
	return &kbSuggestionService{
		chunkRepo:    chunkRepo,
		wikiPageRepo: wikiPageRepo,
	}
}

// defaultKBSuggestionLimit is the default number of questions returned when the
// caller does not specify a limit (limit <= 0).
const defaultKBSuggestionLimit = 6

// GetSuggestedQuestions returns up to `limit` suggested questions for the given
// knowledge base. Questions are collected from three sources (FAQ recommended
// chunks, document chunks with AI-generated questions, wiki page titles),
// deduplicated, then round-robin sampled across knowledge_id buckets for
// diversity. Errors from individual data sources are logged and skipped so a
// single failing source does not blank out the entire response.
func (s *kbSuggestionService) GetSuggestedQuestions(
	ctx context.Context,
	kbID string,
	effectiveTenantID uint64,
	knowledgeIDs []string,
	limit int,
) ([]types.SuggestedQuestion, error) {
	if limit <= 0 {
		limit = defaultKBSuggestionLimit
	}

	result := make([]types.SuggestedQuestion, 0, limit)
	seen := make(map[string]bool)

	kbIDs := []string{kbID}
	fetchLimit := limit * 5
	if fetchLimit < 20 {
		fetchLimit = 20
	}

	// buckets keyed by knowledge_id (or wiki page ID) for round-robin diversity.
	buckets := make(map[string][]types.SuggestedQuestion)

	// 1. FAQ recommended chunks
	faqChunks, err := s.chunkRepo.ListRecommendedFAQChunks(ctx, effectiveTenantID, kbIDs, knowledgeIDs, fetchLimit)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"kb_id":     kbID,
			"tenant_id": effectiveTenantID,
			"source":    "faq",
		})
	} else {
		for _, chunk := range faqChunks {
			meta, metaErr := chunk.FAQMetadata()
			if metaErr != nil || meta == nil || meta.StandardQuestion == "" {
				continue
			}
			if seen[meta.StandardQuestion] {
				continue
			}
			seen[meta.StandardQuestion] = true
			buckets[chunk.KnowledgeID] = append(buckets[chunk.KnowledgeID], types.SuggestedQuestion{
				Question:        meta.StandardQuestion,
				Source:          "faq",
				KnowledgeBaseID: chunk.KnowledgeBaseID,
			})
		}
	}

	// 2. Document chunks with AI-generated questions
	docChunks, err := s.chunkRepo.ListRecentDocumentChunksWithQuestions(ctx, effectiveTenantID, kbIDs, knowledgeIDs, fetchLimit)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"kb_id":     kbID,
			"tenant_id": effectiveTenantID,
			"source":    "document",
		})
	} else {
		for _, chunk := range docChunks {
			meta, metaErr := chunk.DocumentMetadata()
			if metaErr != nil || meta == nil || len(meta.GeneratedQuestions) == 0 {
				continue
			}
			q := meta.GeneratedQuestions[0].Question
			if q == "" || seen[q] {
				continue
			}
			seen[q] = true
			buckets[chunk.KnowledgeID] = append(buckets[chunk.KnowledgeID], types.SuggestedQuestion{
				Question:        q,
				Source:          "document",
				KnowledgeBaseID: chunk.KnowledgeBaseID,
			})
		}
	}

	// 3. Wiki pages (title-based suggestions)
	if s.wikiPageRepo != nil {
		wikiPages, err := s.wikiPageRepo.ListRecentForSuggestions(ctx, effectiveTenantID, kbIDs, fetchLimit)
		if err != nil {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{
				"kb_id":     kbID,
				"tenant_id": effectiveTenantID,
				"source":    "wiki",
			})
		} else {
			locale, _ := types.LanguageFromContext(ctx)
			for _, page := range wikiPages {
				q := wikiSuggestionFromPage(page, locale)
				if q == "" || seen[q] {
					continue
				}
				seen[q] = true
				// Use page.ID as the bucket key so round-robin mixes pages
				// from different wiki entries rather than clumping them.
				buckets[page.ID] = append(buckets[page.ID], types.SuggestedQuestion{
					Question:        q,
					Source:          "wiki",
					KnowledgeBaseID: page.KnowledgeBaseID,
				})
			}
		}
	}

	// 4. Shuffle within each bucket, then round-robin across buckets.
	bucketKeys := make([]string, 0, len(buckets))
	for k, qs := range buckets {
		bucketKeys = append(bucketKeys, k)
		rand.Shuffle(len(qs), func(i, j int) { qs[i], qs[j] = qs[j], qs[i] })
		buckets[k] = qs
	}
	rand.Shuffle(len(bucketKeys), func(i, j int) {
		bucketKeys[i], bucketKeys[j] = bucketKeys[j], bucketKeys[i]
	})

	offsets := make(map[string]int, len(bucketKeys))
	for len(result) < limit {
		picked := false
		for _, key := range bucketKeys {
			if len(result) >= limit {
				break
			}
			qs := buckets[key]
			idx := offsets[key]
			if idx < len(qs) {
				result = append(result, qs[idx])
				offsets[key] = idx + 1
				picked = true
			}
		}
		if !picked {
			break
		}
	}

	return truncateSuggestedQuestions(result, limit), nil
}

// truncateSuggestedQuestions truncates the question list to the specified limit.
func truncateSuggestedQuestions(questions []types.SuggestedQuestion, limit int) []types.SuggestedQuestion {
	if len(questions) > limit {
		return questions[:limit]
	}
	return questions
}

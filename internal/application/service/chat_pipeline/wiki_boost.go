package chatpipeline

import (
	"context"
	"sort"
	"strings"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

const (
	// wikiChunkIDPrefix is the prefix for wiki page chunk IDs ("wp-" + page.ID)
	wikiChunkIDPrefix = "wp-"
	// defaultWikiBoostFactor is the fallback multiplier when page-specific boost is unavailable
	defaultWikiBoostFactor = 1.3
)

// PluginWikiBoost boosts the relevance score of wiki page chunks in search results.
// Wiki pages contain pre-synthesized knowledge that is more coherent and
// cross-referenced than raw document chunks, so they should rank higher.
//
// This plugin runs in the CHUNK_RERANK phase, after initial retrieval and reranking.
// It identifies chunks with ChunkType == "wiki_page" and applies a dynamic boost
// factor based on the page's RetrievalBoost score (confidence/quality/freshness).
type PluginWikiBoost struct {
	kbService       interfaces.KnowledgeBaseService
	wikiPageService interfaces.WikiPageService
}

// NewPluginWikiBoost creates and registers the wiki boost plugin
func NewPluginWikiBoost(
	eventManager *EventManager,
	kbService interfaces.KnowledgeBaseService,
	wikiPageService interfaces.WikiPageService,
) *PluginWikiBoost {
	p := &PluginWikiBoost{
		kbService:       kbService,
		wikiPageService: wikiPageService,
	}
	eventManager.Register(p)
	return p
}

// ActivationEvents returns the event types this plugin handles
func (p *PluginWikiBoost) ActivationEvents() []types.EventType {
	return []types.EventType{types.CHUNK_RERANK}
}

// extractWikiPageID extracts the wiki page ID from a chunk ID.
// Wiki chunk IDs are formatted as "wp-" + page.ID. Returns empty string if not a wiki chunk.
func extractWikiPageID(chunkID string) string {
	if strings.HasPrefix(chunkID, wikiChunkIDPrefix) {
		return strings.TrimPrefix(chunkID, wikiChunkIDPrefix)
	}
	return ""
}

// OnEvent boosts wiki page chunk scores after reranking
func (p *PluginWikiBoost) OnEvent(
	ctx context.Context,
	eventType types.EventType,
	chatManage *types.ChatManage,
	next func() *PluginError,
) *PluginError {
	// Run the normal reranking first
	if err := next(); err != nil {
		return err
	}

	// Fast path: skip all work if there are no wiki_page chunks in the result set.
	// This avoids hitting the KB service on every chat turn for non-wiki queries.
	var wikiChunkIndices []int
	for i := range chatManage.RerankResult {
		if chatManage.RerankResult[i].ChunkType == string(types.ChunkTypeWikiPage) {
			wikiChunkIndices = append(wikiChunkIndices, i)
		}
	}
	if len(wikiChunkIndices) == 0 {
		return nil
	}

	// Confirm at least one search target is actually a wiki KB.
	hasWikiKB := false
	for _, target := range chatManage.SearchTargets {
		kb, err := p.kbService.GetKnowledgeBaseByIDOnly(ctx, target.KnowledgeBaseID)
		if err == nil && kb != nil && kb.IsWikiEnabled() {
			hasWikiKB = true
			break
		}
	}
	if !hasWikiKB {
		return nil
	}

	// Boost wiki page chunks in RerankResult using page-specific RetrievalBoost
	boostedCount := 0
	pageBoostCache := make(map[string]float64)

	for _, idx := range wikiChunkIndices {
		chunk := chatManage.RerankResult[idx]
		pageID := extractWikiPageID(chunk.ID)

		boostFactor := defaultWikiBoostFactor
		if pageID != "" {
			// Check cache first to avoid duplicate lookups
			if cachedBoost, ok := pageBoostCache[pageID]; ok {
				boostFactor = cachedBoost
			} else {
				// Fetch page to get its dynamic RetrievalBoost
				page, err := p.wikiPageService.GetPageByID(ctx, pageID)
				if err == nil && page != nil && page.RetrievalBoost > 0 {
					boostFactor = page.RetrievalBoost
				}
				// Cache the result (even if failed, to avoid repeated lookups)
				pageBoostCache[pageID] = boostFactor
			}
		}

		chunk.Score *= boostFactor
		boostedCount++
	}

	if boostedCount > 0 {
		logger.Infof(ctx, "WikiBoost: boosted %d wiki page chunks with dynamic RetrievalBoost", boostedCount)
		// Re-sort by score after boosting; stable sort preserves ordering for ties.
		sort.SliceStable(chatManage.RerankResult, func(i, j int) bool {
			return chatManage.RerankResult[i].Score > chatManage.RerankResult[j].Score
		})
	}

	return nil
}

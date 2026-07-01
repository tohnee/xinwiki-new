package service

import (
	"context"
	"strings"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

// graphQueryTokenMinLen is the minimum token length passed to SearchNode.
// Single-character tokens (English articles, punctuation) are filtered to
// avoid noisy CONTAINS matches against graph node names.
const graphQueryTokenMinLen = 2

// graphQueryTokens splits a query into the search terms passed to
// RetrieveGraphRepository.SearchNode, which does a CONTAINS match on node
// names. Whitespace tokenisation handles multi-term English queries; a
// whitespace-free query (e.g. Chinese) is passed whole so the substring match
// can still hit. Returns nil for a blank query so the caller can skip the
// graph lookup entirely.
func graphQueryTokens(query string) []string {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	var tokens []string
	for _, p := range strings.Fields(query) {
		if len(p) >= graphQueryTokenMinLen {
			tokens = append(tokens, p)
		}
	}
	if len(tokens) == 0 {
		return []string{query}
	}
	return tokens
}

// retrieveFromGraph is the wiki/graph-only fan-out path for HybridSearch
// (review 5.3.2): when every KB in scope lacks vector/keyword indexing,
// HybridSearch would otherwise return nil. This method queries the knowledge
// graph for nodes whose name matches the query terms, collects the chunk ids
// attached to those nodes, and loads the chunks to build IndexWithScore
// entries (MatchTypeGraph) the caller can run through the same
// processSearchResults + ACL-filter tail as vector/keyword results. Mirrors
// the chat-pipeline entity-search plugin (chat_pipeline/search_entity.go) but
// driven by query tokens rather than LLM-extracted entities, since the
// synchronous search endpoint does not run entity extraction.
//
// KBs without graph enabled are skipped; a SearchNode failure on one KB is
// logged and does not abort the others (graceful degradation). Returns
// (nil, nil) when no graph KB is in scope, the query is empty, or the graph
// yields no chunk ids. Chunks are loaded once here so KnowledgeID is populated
// for processSearchResults' knowledge fetch (the Content is re-fetched there
// for enrichment, matching the vector/keyword path).
func (s *knowledgeBaseService) retrieveFromGraph(
	ctx context.Context,
	kbs []*types.KnowledgeBase,
	query string,
	matchCount int,
	tenantID uint64,
) ([]*types.IndexWithScore, error) {
	var graphKBs []*types.KnowledgeBase
	for _, kb := range kbs {
		if kb != nil && kb.IsGraphEnabled() {
			graphKBs = append(graphKBs, kb)
		}
	}
	if len(graphKBs) == 0 {
		return nil, nil
	}
	tokens := graphQueryTokens(query)
	if len(tokens) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{})
	var chunkIDs []string
	for _, kb := range graphKBs {
		data, err := s.graphEngine.SearchNode(ctx, types.NameSpace{KnowledgeBase: kb.ID}, tokens)
		if err != nil {
			logger.Warnf(ctx, "graph SearchNode failed kb=%s: %v", kb.ID, err)
			continue
		}
		if data == nil {
			continue
		}
		for _, node := range data.Node {
			for _, cid := range node.Chunks {
				if cid == "" {
					continue
				}
				if _, ok := seen[cid]; !ok {
					seen[cid] = struct{}{}
					chunkIDs = append(chunkIDs, cid)
				}
			}
		}
	}
	if len(chunkIDs) == 0 {
		return nil, nil
	}
	// Cap the load to keep the synchronous path bounded; graph traversal can
	// otherwise surface a large fan of chunk ids on high-degree entities.
	cap := matchCount * 4
	if cap < 20 {
		cap = 20
	}
	if len(chunkIDs) > cap {
		chunkIDs = chunkIDs[:cap]
	}
	chunks, err := s.chunkRepo.ListChunksByID(ctx, tenantID, chunkIDs)
	if err != nil {
		return nil, err
	}
	results := make([]*types.IndexWithScore, 0, len(chunks))
	for _, ch := range chunks {
		if ch == nil {
			continue
		}
		results = append(results, &types.IndexWithScore{
			ChunkID:         ch.ID,
			KnowledgeID:     ch.KnowledgeID,
			KnowledgeBaseID: ch.KnowledgeBaseID,
			Content:         ch.Content,
			Score:           1.0, // graph match: uniform score; re-ranking/RRF applies downstream
			MatchType:       types.MatchTypeGraph,
			IsEnabled:       true,
		})
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results, nil
}

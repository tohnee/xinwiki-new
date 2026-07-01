package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// graphSearchFakeGraphRepo fakes RetrieveGraphRepository for graph-into-QA
// tests. Embedding the interface satisfies the compiler for unused methods.
type graphSearchFakeGraphRepo struct {
	interfaces.RetrieveGraphRepository
	searchNode func(ctx context.Context, ns types.NameSpace, nodes []string) (*types.GraphData, error)
}

func (f *graphSearchFakeGraphRepo) SearchNode(ctx context.Context, ns types.NameSpace, nodes []string) (*types.GraphData, error) {
	if f.searchNode != nil {
		return f.searchNode(ctx, ns, nodes)
	}
	return nil, nil
}

// graphSearchFakeChunkRepo fakes ChunkRepository.ListChunksByID.
type graphSearchFakeChunkRepo struct {
	interfaces.ChunkRepository
	listByID func(ctx context.Context, tenantID uint64, ids []string) ([]*types.Chunk, error)
}

func (f *graphSearchFakeChunkRepo) ListChunksByID(ctx context.Context, tenantID uint64, ids []string) ([]*types.Chunk, error) {
	if f.listByID != nil {
		return f.listByID(ctx, tenantID, ids)
	}
	return nil, nil
}

// graphOnlyKB builds a KB with graph extraction enabled and no vector/keyword
// indexing (the wiki/graph-only case review 5.3.2 calls out).
func graphOnlyKB(id string) *types.KnowledgeBase {
	return &types.KnowledgeBase{
		ID:               id,
		IndexingStrategy: types.IndexingStrategy{GraphEnabled: true},
		ExtractConfig:    &types.ExtractConfig{Enabled: true},
	}
}

// TestRetrieveFromGraph_NoGraphKBsReturnsNil: with no graph-enabled KB in
// scope there is nothing to search; return nil without calling the repo.
func TestRetrieveFromGraph_NoGraphKBsReturnsNil(t *testing.T) {
	called := false
	s := &knowledgeBaseService{
		graphEngine: &graphSearchFakeGraphRepo{searchNode: func(context.Context, types.NameSpace, []string) (*types.GraphData, error) {
			called = true
			return nil, nil
		}},
	}
	kbs := []*types.KnowledgeBase{{ID: "kb1", IndexingStrategy: types.IndexingStrategy{}}} // graph disabled

	chunks, err := s.retrieveFromGraph(context.Background(), kbs, "query", 10, 1)
	require.NoError(t, err)
	require.Nil(t, chunks)
	require.False(t, called, "SearchNode must not be called when no graph KB is in scope")
}

// TestRetrieveFromGraph_EmptyQueryReturnsNil: a blank query yields no tokens,
// so the graph is not queried.
func TestRetrieveFromGraph_EmptyQueryReturnsNil(t *testing.T) {
	called := false
	s := &knowledgeBaseService{
		graphEngine: &graphSearchFakeGraphRepo{searchNode: func(context.Context, types.NameSpace, []string) (*types.GraphData, error) {
			called = true
			return nil, nil
		}},
	}
	chunks, err := s.retrieveFromGraph(context.Background(), []*types.KnowledgeBase{graphOnlyKB("kb1")}, "   ", 10, 1)
	require.NoError(t, err)
	require.Nil(t, chunks)
	require.False(t, called, "SearchNode must not be called for an empty query")
}

// TestRetrieveFromGraph_CollectsChunksFromGraphNodes: chunk ids attached to
// matched graph nodes are loaded and returned; the query is tokenised into the
// nodes argument passed to SearchNode.
func TestRetrieveFromGraph_CollectsChunksFromGraphNodes(t *testing.T) {
	var capturedNodes []string
	graphRepo := &graphSearchFakeGraphRepo{searchNode: func(_ context.Context, _ types.NameSpace, nodes []string) (*types.GraphData, error) {
		capturedNodes = nodes
		return &types.GraphData{Node: []*types.GraphNode{
			{Name: "entity1", Chunks: []string{"c1", "c2"}},
		}}, nil
	}}
	chunkRepo := &graphSearchFakeChunkRepo{listByID: func(_ context.Context, _ uint64, ids []string) ([]*types.Chunk, error) {
		out := make([]*types.Chunk, 0, len(ids))
		for _, id := range ids {
			out = append(out, &types.Chunk{ID: id, Content: "content-" + id})
		}
		return out, nil
	}}
	s := &knowledgeBaseService{graphEngine: graphRepo, chunkRepo: chunkRepo}

	chunks, err := s.retrieveFromGraph(context.Background(), []*types.KnowledgeBase{graphOnlyKB("kb1")}, "entity1 report", 10, 1)
	require.NoError(t, err)
	require.Len(t, chunks, 2)
	assert.Equal(t, []string{"c1", "c2"}, []string{chunks[0].ChunkID, chunks[1].ChunkID})
	// Query is tokenised on whitespace; both terms survive (len >= 2).
	assert.Contains(t, capturedNodes, "entity1")
	assert.Contains(t, capturedNodes, "report")
}

// TestRetrieveFromGraph_DedupsChunkIDs: the same chunk id appearing on
// multiple nodes / KBs is loaded once.
func TestRetrieveFromGraph_DedupsChunkIDs(t *testing.T) {
	graphRepo := &graphSearchFakeGraphRepo{searchNode: func(context.Context, types.NameSpace, []string) (*types.GraphData, error) {
		return &types.GraphData{Node: []*types.GraphNode{
			{Name: "a", Chunks: []string{"c1", "c2"}},
			{Name: "b", Chunks: []string{"c2", "c3"}},
		}}, nil
	}}
	var loadedIDs []string
	chunkRepo := &graphSearchFakeChunkRepo{listByID: func(_ context.Context, _ uint64, ids []string) ([]*types.Chunk, error) {
		loadedIDs = ids
		return []*types.Chunk{{ID: "c1"}, {ID: "c2"}, {ID: "c3"}}, nil
	}}
	s := &knowledgeBaseService{graphEngine: graphRepo, chunkRepo: chunkRepo}

	chunks, err := s.retrieveFromGraph(context.Background(), []*types.KnowledgeBase{graphOnlyKB("kb1")}, "query", 10, 1)
	require.NoError(t, err)
	require.Len(t, chunks, 3)
	assert.ElementsMatch(t, []string{"c1", "c2", "c3"}, loadedIDs)
}

// TestRetrieveFromGraph_SearchNodeErrorSkipsKB: a failing KB is skipped; the
// other KB's chunks are still returned (graceful degradation, no hard error).
func TestRetrieveFromGraph_SearchNodeErrorSkipsKB(t *testing.T) {
	graphRepo := &graphSearchFakeGraphRepo{searchNode: func(_ context.Context, ns types.NameSpace, _ []string) (*types.GraphData, error) {
		if ns.KnowledgeBase == "bad" {
			return nil, errors.New("neo4j down")
		}
		return &types.GraphData{Node: []*types.GraphNode{{Name: "ok", Chunks: []string{"c1"}}}}, nil
	}}
	chunkRepo := &graphSearchFakeChunkRepo{listByID: func(_ context.Context, _ uint64, ids []string) ([]*types.Chunk, error) {
		return []*types.Chunk{{ID: "c1"}}, nil
	}}
	s := &knowledgeBaseService{graphEngine: graphRepo, chunkRepo: chunkRepo}

	chunks, err := s.retrieveFromGraph(context.Background(),
		[]*types.KnowledgeBase{graphOnlyKB("bad"), graphOnlyKB("good")}, "query", 10, 1)
	require.NoError(t, err)
	require.Len(t, chunks, 1)
	assert.Equal(t, "c1", chunks[0].ChunkID)
}

// TestRetrieveFromGraph_NoChunksReturnsNil: graph nodes with no chunk refs
// yield nothing to load.
func TestRetrieveFromGraph_NoChunksReturnsNil(t *testing.T) {
	graphRepo := &graphSearchFakeGraphRepo{searchNode: func(context.Context, types.NameSpace, []string) (*types.GraphData, error) {
		return &types.GraphData{Node: []*types.GraphNode{{Name: "entity"}}}, nil
	}}
	s := &knowledgeBaseService{graphEngine: graphRepo}

	chunks, err := s.retrieveFromGraph(context.Background(), []*types.KnowledgeBase{graphOnlyKB("kb1")}, "query", 10, 1)
	require.NoError(t, err)
	require.Nil(t, chunks)
}

// TestGraphQueryTokens: whitespace tokenisation filters short tokens and
// falls back to the whole query when nothing survives.
func TestGraphQueryTokens(t *testing.T) {
	assert.Equal(t, []string{"entity1", "report"}, graphQueryTokens("entity1 report"))
	// "a" is len 1 -> filtered; only "report" survives.
	assert.Equal(t, []string{"report"}, graphQueryTokens("a report"))
	// Chinese / single token: no whitespace -> whole query as one token.
	assert.Equal(t, []string{"第三季度报告"}, graphQueryTokens("第三季度报告"))
	assert.Nil(t, graphQueryTokens("   "))
}

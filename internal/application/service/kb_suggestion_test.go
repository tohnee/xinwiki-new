package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// fakeSuggestionChunkRepo fakes the two chunk-repo methods used by
// kbSuggestionService. It embeds interfaces.ChunkRepository so only the
// methods under test need to be overridden.
type fakeSuggestionChunkRepo struct {
	interfaces.ChunkRepository
	faqChunks []*types.Chunk
	faqErr    error
	docChunks []*types.Chunk
	docErr    error

	capturedFAQTenant  uint64
	capturedFAQKBIDs   []string
	capturedFAQKIDs    []string
	capturedDocTenant  uint64
	capturedDocKBIDs   []string
	capturedDocKIDs    []string
}

func (f *fakeSuggestionChunkRepo) ListRecommendedFAQChunks(
	_ context.Context, tenantID uint64, kbIDs []string, knowledgeIDs []string, _ int,
) ([]*types.Chunk, error) {
	f.capturedFAQTenant = tenantID
	f.capturedFAQKBIDs = append([]string{}, kbIDs...)
	f.capturedFAQKIDs = append([]string{}, knowledgeIDs...)
	return f.faqChunks, f.faqErr
}

func (f *fakeSuggestionChunkRepo) ListRecentDocumentChunksWithQuestions(
	_ context.Context, tenantID uint64, kbIDs []string, knowledgeIDs []string, _ int,
) ([]*types.Chunk, error) {
	f.capturedDocTenant = tenantID
	f.capturedDocKBIDs = append([]string{}, kbIDs...)
	f.capturedDocKIDs = append([]string{}, knowledgeIDs...)
	return f.docChunks, f.docErr
}

// fakeSuggestionWikiRepo fakes WikiPageRepository.ListRecentForSuggestions.
type fakeSuggestionWikiRepo struct {
	interfaces.WikiPageRepository
	pages []types.WikiPage
	err   error

	capturedTenant uint64
	capturedKBIDs  []string
}

func (f *fakeSuggestionWikiRepo) ListRecentForSuggestions(
	_ context.Context, tenantID uint64, kbIDs []string, _ int,
) ([]*types.WikiPage, error) {
	f.capturedTenant = tenantID
	f.capturedKBIDs = append([]string{}, kbIDs...)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]*types.WikiPage, 0, len(f.pages))
	for i := range f.pages {
		out = append(out, &f.pages[i])
	}
	return out, nil
}

// makeFAQChunk builds a Chunk whose FAQMetadata.StandardQuestion is set.
func makeFAQChunk(knowledgeID, kbID, question string) *types.Chunk {
	c := &types.Chunk{
		KnowledgeID:     knowledgeID,
		KnowledgeBaseID: kbID,
	}
	_ = c.SetFAQMetadata(&types.FAQChunkMetadata{StandardQuestion: question})
	return c
}

// makeDocChunk builds a Chunk whose DocumentMetadata.GeneratedQuestions[0] is set.
func makeDocChunk(knowledgeID, kbID, question string) *types.Chunk {
	c := &types.Chunk{
		KnowledgeID:     knowledgeID,
		KnowledgeBaseID: kbID,
	}
	_ = c.SetDocumentMetadata(&types.DocumentChunkMetadata{
		GeneratedQuestions: []types.GeneratedQuestion{{ID: "g1", Question: question}},
	})
	return c
}

func TestKBSuggestion_EmptyReturnsEmpty(t *testing.T) {
	svc := NewKnowledgeBaseSuggestionService(
		&fakeSuggestionChunkRepo{},
		&fakeSuggestionWikiRepo{},
	)
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestKBSuggestion_FAQOnly(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "什么是 RAG？"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "什么是 RAG？", out[0].Question)
	require.Equal(t, "faq", out[0].Source)
	require.Equal(t, "kb1", out[0].KnowledgeBaseID)
}

func TestKBSuggestion_DocumentOnly(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		docChunks: []*types.Chunk{
			makeDocChunk("k2", "kb1", "如何配置嵌入模型？"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "如何配置嵌入模型？", out[0].Question)
	require.Equal(t, "document", out[0].Source)
}

func TestKBSuggestion_WikiOnly(t *testing.T) {
	wikiRepo := &fakeSuggestionWikiRepo{
		pages: []types.WikiPage{
			{ID: "p1", Title: "混合检索", PageType: types.WikiPageTypeConcept, KnowledgeBaseID: "kb1"},
		},
	}
	svc := NewKnowledgeBaseSuggestionService(&fakeSuggestionChunkRepo{}, wikiRepo)
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Contains(t, out[0].Question, "混合检索")
	require.Equal(t, "wiki", out[0].Source)
}

func TestKBSuggestion_MixedSourcesRoundRobin(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "FAQ 问题 1"),
			makeFAQChunk("k1", "kb1", "FAQ 问题 2"),
		},
		docChunks: []*types.Chunk{
			makeDocChunk("k2", "kb1", "文档问题 1"),
			makeDocChunk("k2", "kb1", "文档问题 2"),
		},
	}
	wikiRepo := &fakeSuggestionWikiRepo{
		pages: []types.WikiPage{
			{ID: "p1", Title: "Wiki 页面 1", PageType: types.WikiPageTypeConcept, KnowledgeBaseID: "kb1"},
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, wikiRepo)
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 3)
	require.NoError(t, err)
	require.Len(t, out, 3)

	// With 3 buckets (k1, k2, p1) and limit 3, round-robin picks one from each.
	sources := map[string]bool{}
	for _, q := range out {
		sources[q.Source] = true
	}
	// Should have at least 2 different sources (round-robin diversity).
	require.GreaterOrEqual(t, len(sources), 2, "round-robin should produce diverse sources")
}

func TestKBSuggestion_Dedup(t *testing.T) {
	// Same question from FAQ and document sources - only the first occurrence
	// (FAQ, since it's collected first) should survive.
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "重复问题"),
		},
		docChunks: []*types.Chunk{
			makeDocChunk("k2", "kb1", "重复问题"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Len(t, out, 1, "duplicate question should be deduped")
	require.Equal(t, "faq", out[0].Source, "FAQ collected first wins the dedup")
}

func TestKBSuggestion_LimitEnforced(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "Q1"),
			makeFAQChunk("k1", "kb1", "Q2"),
			makeFAQChunk("k1", "kb1", "Q3"),
			makeFAQChunk("k1", "kb1", "Q4"),
			makeFAQChunk("k1", "kb1", "Q5"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 2)
	require.NoError(t, err)
	require.Len(t, out, 2, "result must be truncated to limit")
}

func TestKBSuggestion_DefaultLimit(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "Q1"),
			makeFAQChunk("k1", "kb1", "Q2"),
			makeFAQChunk("k1", "kb1", "Q3"),
			makeFAQChunk("k1", "kb1", "Q4"),
			makeFAQChunk("k1", "kb1", "Q5"),
			makeFAQChunk("k1", "kb1", "Q6"),
			makeFAQChunk("k1", "kb1", "Q7"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	// limit = 0 should default to 6
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 0)
	require.NoError(t, err)
	require.Len(t, out, 6, "limit <= 0 should default to 6")
}

func TestKBSuggestion_ChunkRepoErrorSkipped(t *testing.T) {
	// Both chunk sources error, but wiki still returns results.
	chunkRepo := &fakeSuggestionChunkRepo{
		faqErr: errors.New("db down"),
		docErr: errors.New("db down"),
	}
	wikiRepo := &fakeSuggestionWikiRepo{
		pages: []types.WikiPage{
			{ID: "p1", Title: "Wiki", PageType: types.WikiPageTypeConcept, KnowledgeBaseID: "kb1"},
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, wikiRepo)
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err, "service must not propagate repo errors")
	require.Len(t, out, 1)
	require.Equal(t, "wiki", out[0].Source)
}

func TestKBSuggestion_KnowledgeIDsPassedThrough(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "Q1"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	_, _ = svc.GetSuggestedQuestions(context.Background(), "kb1", 7, []string{"ka", "kb"}, 6)
	require.Equal(t, []string{"ka", "kb"}, chunkRepo.capturedFAQKIDs, "knowledgeIDs must be forwarded to FAQ query")
	require.Equal(t, []string{"ka", "kb"}, chunkRepo.capturedDocKIDs, "knowledgeIDs must be forwarded to doc query")
}

func TestKBSuggestion_EffectiveTenantIDUsed(t *testing.T) {
	// The service must query using the effectiveTenantID passed in, NOT the
	// caller's tenant from context. This is critical for cross-tenant shared
	// KBs where chunks live under the source tenant.
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "Q1"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	_, _ = svc.GetSuggestedQuestions(context.Background(), "kb1", 42, nil, 6)
	require.Equal(t, uint64(42), chunkRepo.capturedFAQTenant, "must use the effective tenant ID, not context tenant")
	require.Equal(t, uint64(42), chunkRepo.capturedDocTenant)
}

func TestKBSuggestion_KBIDScoped(t *testing.T) {
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "Q1"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	_, _ = svc.GetSuggestedQuestions(context.Background(), "kb-target", 7, nil, 6)
	require.Equal(t, []string{"kb-target"}, chunkRepo.capturedFAQKBIDs, "must scope FAQ query to the single KB ID")
	require.Equal(t, []string{"kb-target"}, chunkRepo.capturedDocKBIDs, "must scope doc query to the single KB ID")
}

func TestKBSuggestion_SkipsChunkWithEmptyQuestion(t *testing.T) {
	// A FAQ chunk with an empty StandardQuestion should be silently skipped.
	faqMeta, _ := json.Marshal(types.FAQChunkMetadata{StandardQuestion: ""})
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			{KnowledgeID: "k1", KnowledgeBaseID: "kb1", Metadata: types.JSON(faqMeta)},
			makeFAQChunk("k2", "kb1", "有效问题"),
		},
	}
	svc := NewKnowledgeBaseSuggestionService(chunkRepo, &fakeSuggestionWikiRepo{})
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "有效问题", out[0].Question)
}

func TestKBSuggestion_NilWikiRepoSafe(t *testing.T) {
	// If wikiPageRepo is nil (not wired), the service should still return
	// FAQ + document results without panicking.
	chunkRepo := &fakeSuggestionChunkRepo{
		faqChunks: []*types.Chunk{
			makeFAQChunk("k1", "kb1", "Q1"),
		},
	}
	svc := &kbSuggestionService{
		chunkRepo:    chunkRepo,
		wikiPageRepo: nil,
	}
	out, err := svc.GetSuggestedQuestions(context.Background(), "kb1", 7, nil, 6)
	require.NoError(t, err)
	require.Len(t, out, 1)
}

func TestTruncateSuggestedQuestions(t *testing.T) {
	qs := []types.SuggestedQuestion{{Question: "a"}, {Question: "b"}, {Question: "c"}}
	require.Equal(t, 2, len(truncateSuggestedQuestions(qs, 2)))
	require.Equal(t, 3, len(truncateSuggestedQuestions(qs, 5)))
	require.Equal(t, 0, len(truncateSuggestedQuestions(nil, 5)))
}

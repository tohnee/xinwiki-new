package wiki

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// --- 测试用的 mock 实现 ---

// mockChunker 记录每次 Chunk 调用的内容，用于验证增量编译是否仅处理变更部分。
type mockChunker struct {
	chunkedContents []string // 按调用顺序记录被 chunk 的内容
}

func (m *mockChunker) Chunk(content string, metadata map[string]string) ([]*Chunk, error) {
	m.chunkedContents = append(m.chunkedContents, content)
	// 简单按行分割，每行一个 chunk
	lines := strings.Split(content, "\n")
	var chunks []*Chunk
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		chunks = append(chunks, &Chunk{
			ID:         fmt.Sprintf("chunk-%d", i),
			Content:    line,
			TokenCount: estimateTokens(line),
		})
	}
	return chunks, nil
}

func (m *mockChunker) ChunkSize() int    { return 500 }
func (m *mockChunker) ChunkOverlap() int { return 50 }

// mockEmbedder 记录每次 EmbedBatch 调用的文本，用于验证增量编译。
type mockEmbedder struct {
	embeddedTexts []string // 按调用顺序记录被 embed 的文本
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.embeddedTexts = append(m.embeddedTexts, text)
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	m.embeddedTexts = append(m.embeddedTexts, texts...)
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = []float32{0.1, 0.2, 0.3}
	}
	return embeddings, nil
}

func (m *mockEmbedder) ModelName() string { return "mock-embedder" }
func (m *mockEmbedder) Dimension() int    { return 3 }

// mockChunkRepo 内存存储 chunks，用于测试。
type mockChunkRepo struct {
	savedChunks []*Chunk
}

func (m *mockChunkRepo) SaveChunks(ctx context.Context, chunks []*Chunk) error {
	m.savedChunks = append(m.savedChunks, chunks...)
	return nil
}

func (m *mockChunkRepo) GetChunksByWikiPage(ctx context.Context, wikiPageID string) ([]*Chunk, error) {
	var result []*Chunk
	for _, c := range m.savedChunks {
		if c.WikiPageID == wikiPageID {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockChunkRepo) DeleteChunksByWikiPage(ctx context.Context, wikiPageID string) error {
	var remaining []*Chunk
	for _, c := range m.savedChunks {
		if c.WikiPageID != wikiPageID {
			remaining = append(remaining, c)
		}
	}
	m.savedChunks = remaining
	return nil
}

func (m *mockChunkRepo) GetChunk(ctx context.Context, chunkID string) (*Chunk, error) {
	for _, c := range m.savedChunks {
		if c.ID == chunkID {
			return c, nil
		}
	}
	return nil, fmt.Errorf("chunk not found")
}

func (m *mockChunkRepo) GetChunks(ctx context.Context, chunkIDs []string) ([]*Chunk, error) {
	var result []*Chunk
	for _, c := range m.savedChunks {
		for _, id := range chunkIDs {
			if c.ID == id {
				result = append(result, c)
				break
			}
		}
	}
	return result, nil
}

// --- splitIntoSections 测试 ---

func TestSplitIntoSections(t *testing.T) {
	content := `# Title 1
content 1 line 1
content 1 line 2
## Subtitle
sub content
# Title 2
content 2`

	sections := splitIntoSections(content)

	if len(sections) != 3 {
		t.Fatalf("期望 3 个 sections，实际 %d", len(sections))
	}

	// 第一个 section: "# Title 1"
	if sections[0].title != "Title 1" {
		t.Errorf("section[0] title 期望 'Title 1'，实际 '%s'", sections[0].title)
	}
	if sections[0].level != 1 {
		t.Errorf("section[0] level 期望 1，实际 %d", sections[0].level)
	}
	if !strings.Contains(sections[0].content, "content 1 line 1") {
		t.Errorf("section[0] content 应包含正文，实际 '%s'", sections[0].content)
	}
	if !strings.Contains(sections[0].content, "# Title 1") {
		t.Errorf("section[0] content 应包含标题行，实际 '%s'", sections[0].content)
	}

	// 第二个 section: "## Subtitle"
	if sections[1].title != "Subtitle" {
		t.Errorf("section[1] title 期望 'Subtitle'，实际 '%s'", sections[1].title)
	}
	if sections[1].level != 2 {
		t.Errorf("section[1] level 期望 2，实际 %d", sections[1].level)
	}
	if !strings.Contains(sections[1].content, "sub content") {
		t.Errorf("section[1] content 应包含正文，实际 '%s'", sections[1].content)
	}

	// 第三个 section: "# Title 2"
	if sections[2].title != "Title 2" {
		t.Errorf("section[2] title 期望 'Title 2'，实际 '%s'", sections[2].title)
	}
	if sections[2].level != 1 {
		t.Errorf("section[2] level 期望 1，实际 %d", sections[2].level)
	}
	if !strings.Contains(sections[2].content, "content 2") {
		t.Errorf("section[2] content 应包含正文，实际 '%s'", sections[2].content)
	}
}

func TestSplitIntoSections_EmptyContent(t *testing.T) {
	sections := splitIntoSections("")
	if len(sections) != 0 {
		t.Fatalf("空内容应返回 0 个 sections，实际 %d", len(sections))
	}
}

func TestSplitIntoSections_NoHeading(t *testing.T) {
	content := "just some text\nwithout headings"
	sections := splitIntoSections(content)
	if len(sections) != 1 {
		t.Fatalf("无标题内容应返回 1 个 section，实际 %d", len(sections))
	}
	if sections[0].title != "" {
		t.Errorf("无标题 section title 应为空，实际 '%s'", sections[0].title)
	}
	if sections[0].level != 0 {
		t.Errorf("无标题 section level 应为 0，实际 %d", sections[0].level)
	}
}

// --- diffSections 测试 ---

func TestDiffSections_NoChange(t *testing.T) {
	old := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
		{title: "B", level: 1, content: "# B\ncontentB"},
	}
	new := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
		{title: "B", level: 1, content: "# B\ncontentB"},
	}

	diff := diffSections(old, new)

	if len(diff.added) != 0 {
		t.Errorf("added 应为空，实际 %d", len(diff.added))
	}
	if len(diff.modified) != 0 {
		t.Errorf("modified 应为空，实际 %d", len(diff.modified))
	}
	if len(diff.removed) != 0 {
		t.Errorf("removed 应为空，实际 %d", len(diff.removed))
	}
}

func TestDiffSections_AddedSection(t *testing.T) {
	old := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
	}
	new := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
		{title: "B", level: 1, content: "# B\ncontentB"},
	}

	diff := diffSections(old, new)

	if len(diff.added) != 1 {
		t.Fatalf("added 应为 1，实际 %d", len(diff.added))
	}
	if diff.added[0].title != "B" {
		t.Errorf("added[0] title 期望 'B'，实际 '%s'", diff.added[0].title)
	}
	if len(diff.modified) != 0 {
		t.Errorf("modified 应为空，实际 %d", len(diff.modified))
	}
	if len(diff.removed) != 0 {
		t.Errorf("removed 应为空，实际 %d", len(diff.removed))
	}
}

func TestDiffSections_ModifiedSection(t *testing.T) {
	old := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
		{title: "B", level: 1, content: "# B\ncontentB"},
	}
	new := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
		{title: "B", level: 1, content: "# B\ncontentBModified"},
	}

	diff := diffSections(old, new)

	if len(diff.added) != 0 {
		t.Errorf("added 应为空，实际 %d", len(diff.added))
	}
	if len(diff.modified) != 1 {
		t.Fatalf("modified 应为 1，实际 %d", len(diff.modified))
	}
	if diff.modified[0].title != "B" {
		t.Errorf("modified[0] title 期望 'B'，实际 '%s'", diff.modified[0].title)
	}
	if !strings.Contains(diff.modified[0].content, "contentBModified") {
		t.Errorf("modified[0] content 应包含修改后内容，实际 '%s'", diff.modified[0].content)
	}
	if len(diff.removed) != 0 {
		t.Errorf("removed 应为空，实际 %d", len(diff.removed))
	}
}

func TestDiffSections_RemovedSection(t *testing.T) {
	old := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
		{title: "B", level: 1, content: "# B\ncontentB"},
	}
	new := []wikiSection{
		{title: "A", level: 1, content: "# A\ncontentA"},
	}

	diff := diffSections(old, new)

	if len(diff.added) != 0 {
		t.Errorf("added 应为空，实际 %d", len(diff.added))
	}
	if len(diff.modified) != 0 {
		t.Errorf("modified 应为空，实际 %d", len(diff.modified))
	}
	if len(diff.removed) != 1 {
		t.Fatalf("removed 应为 1，实际 %d", len(diff.removed))
	}
	if diff.removed[0].title != "B" {
		t.Errorf("removed[0] title 期望 'B'，实际 '%s'", diff.removed[0].title)
	}
}

// --- IncrementalUpdate 测试 ---

func TestIncrementalUpdate_PartialRecompile(t *testing.T) {
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	repo := &mockChunkRepo{}

	compiler := NewIncrementalCompiler(embedder, chunker, repo, time.Hour, 100)

	ctx := context.Background()
	wikiPageID := "page-1"
	kbID := "kb-1"

	// oldContent 包含两个 section，A 和 B
	oldContent := "# A\ncontentA\n# B\ncontentB"
	// newContent 仅修改了 B，A 保持不变
	newContent := "# A\ncontentA\n# B\ncontentBModified"

	metadata := map[string]string{"source": "test"}

	compiled, err := compiler.IncrementalUpdate(ctx, wikiPageID, kbID, oldContent, newContent, metadata)
	if err != nil {
		t.Fatalf("IncrementalUpdate 失败: %v", err)
	}

	// 验证 chunker 只处理了变更的 section（B），不应处理未变更的 section（A）
	// chunker 被调用的次数应等于 added + modified 的 section 数量
	chunkCallCount := len(chunker.chunkedContents)
	if chunkCallCount != 1 {
		t.Errorf("chunker 应只被调用 1 次（仅修改的 section B），实际 %d 次", chunkCallCount)
	}

	// 验证被 chunk 的内容是修改后的 B section，应包含 "contentBModified"
	if chunkCallCount > 0 {
		chunkedContent := chunker.chunkedContents[0]
		if !strings.Contains(chunkedContent, "contentBModified") {
			t.Errorf("被 chunk 的内容应包含 'contentBModified'，实际 '%s'", chunkedContent)
		}
		if strings.Contains(chunkedContent, "contentA") {
			t.Errorf("被 chunk 的内容不应包含未变更的 'contentA'，实际 '%s'", chunkedContent)
		}
	}

	// 验证 embedder 也只处理了变更部分
	embedCallCount := len(embedder.embeddedTexts)
	if embedCallCount == 0 {
		t.Errorf("embedder 应至少被调用一次处理变更 section")
	}
	for _, text := range embedder.embeddedTexts {
		if strings.Contains(text, "contentA") {
			t.Errorf("embedder 不应处理未变更的 'contentA'，实际处理了 '%s'", text)
		}
	}

	// 验证编译结果包含两个 section 的 chunks（A 复用，B 重新编译）
	if len(compiled.Chunks) == 0 {
		t.Errorf("编译结果应包含 chunks")
	}

	// 验证最终 chunks 中既有 A 的内容也有 B 的内容
	hasA := false
	hasB := false
	for _, c := range compiled.Chunks {
		if strings.Contains(c.Content, "contentA") {
			hasA = true
		}
		if strings.Contains(c.Content, "contentBModified") {
			hasB = true
		}
	}
	if !hasA {
		t.Errorf("编译结果应包含未变更 section A 的内容")
	}
	if !hasB {
		t.Errorf("编译结果应包含变更 section B 的新内容")
	}

	// 验证不应包含旧的 B 内容
	for _, c := range compiled.Chunks {
		if strings.Contains(c.Content, "contentB") && !strings.Contains(c.Content, "contentBModified") {
			t.Errorf("编译结果不应包含旧的 B 内容 'contentB'，实际 '%s'", c.Content)
		}
	}
}

// TestIncrementalUpdate_ReusesExistingChunksForUnchangedSection verifies the
// core incremental guarantee: when a section is unchanged, its pre-existing
// chunks (and their embeddings) are reused verbatim — the chunker and
// embedder are NOT invoked for it. This is what distinguishes
// IncrementalUpdate from CompileWiki's whole-page recompile.
func TestIncrementalUpdate_ReusesExistingChunksForUnchangedSection(t *testing.T) {
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	repo := &mockChunkRepo{}

	// Pre-populate the repo with a chunk for unchanged section A, carrying a
	// sentinel embedding so we can prove it is preserved (not overwritten).
	existingA := &Chunk{
		ID:              "existing-A",
		KnowledgeBaseID: "kb-r",
		WikiPageID:      "page-r",
		Content:         "# A\ncontentA",
		Section:         "A",
		Embedding:       []float32{9.9, 9.9, 9.9},
		TokenCount:      7,
	}
	repo.savedChunks = []*Chunk{existingA}

	compiler := NewIncrementalCompiler(embedder, chunker, repo, time.Hour, 100)
	ctx := context.Background()

	// A unchanged; B modified.
	oldContent := "# A\ncontentA\n# B\ncontentB"
	newContent := "# A\ncontentA\n# B\ncontentBModified"

	compiled, err := compiler.IncrementalUpdate(ctx, "page-r", "kb-r", oldContent, newContent, nil)
	if err != nil {
		t.Fatalf("IncrementalUpdate 失败: %v", err)
	}

	// Chunker must only process the modified section B, never A.
	if got := len(chunker.chunkedContents); got != 1 {
		t.Fatalf("chunker 应只被调用 1 次（仅 B），实际 %d 次", got)
	}
	if !strings.Contains(chunker.chunkedContents[0], "contentBModified") {
		t.Errorf("chunker 处理的应是修改后的 B，实际 '%s'", chunker.chunkedContents[0])
	}

	// Embedder must never see A's content.
	for _, text := range embedder.embeddedTexts {
		if strings.Contains(text, "contentA") {
			t.Errorf("embedder 不应处理未变更的 A，实际处理了 '%s'", text)
		}
	}

	// The reused A chunk must be present with its original embedding intact.
	var reusedA *Chunk
	for _, c := range compiled.Chunks {
		if c.ID == "existing-A" {
			reusedA = c
		}
	}
	if reusedA == nil {
		t.Fatalf("未变更 section A 的既有 chunk 应被复用，但未在结果中找到 existing-A")
	}
	if len(reusedA.Embedding) != 3 || reusedA.Embedding[0] != 9.9 {
		t.Errorf("复用 chunk 的 embedding 应保持原值 [9.9 9.9 9.9]，实际 %v", reusedA.Embedding)
	}
}

// TestIncrementalUpdate_RemovedSectionDropped verifies that a section present
// in the old content but absent from the new content is dropped from the
// compiled result (its chunks are not carried over).
func TestIncrementalUpdate_RemovedSectionDropped(t *testing.T) {
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	repo := &mockChunkRepo{}

	compiler := NewIncrementalCompiler(embedder, chunker, repo, time.Hour, 100)
	ctx := context.Background()

	// B is removed in the new content.
	oldContent := "# A\ncontentA\n# B\ncontentB"
	newContent := "# A\ncontentA"

	compiled, err := compiler.IncrementalUpdate(ctx, "page-d", "kb-d", oldContent, newContent, nil)
	if err != nil {
		t.Fatalf("IncrementalUpdate 失败: %v", err)
	}

	// Chunker must not run at all: A is unchanged (materialized), B is gone.
	if got := len(chunker.chunkedContents); got != 0 {
		t.Errorf("chunker 不应被调用（A 未变更、B 已删除），实际 %d 次", got)
	}

	// Result must not contain the removed section B's content.
	for _, c := range compiled.Chunks {
		if strings.Contains(c.Content, "contentB") {
			t.Errorf("已删除 section B 的内容不应出现在结果中，实际 '%s'", c.Content)
		}
	}

	// And must still contain the unchanged section A.
	hasA := false
	for _, c := range compiled.Chunks {
		if strings.Contains(c.Content, "contentA") {
			hasA = true
		}
	}
	if !hasA {
		t.Errorf("未变更 section A 的内容应保留在结果中")
	}
}

func TestIncrementalUpdate_AllChanged(t *testing.T) {
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	repo := &mockChunkRepo{}

	compiler := NewIncrementalCompiler(embedder, chunker, repo, time.Hour, 100)

	ctx := context.Background()
	wikiPageID := "page-2"
	kbID := "kb-2"

	oldContent := "# A\ncontentA"
	newContent := "# A\ncontentAModified"

	compiled, err := compiler.IncrementalUpdate(ctx, wikiPageID, kbID, oldContent, newContent, nil)
	if err != nil {
		t.Fatalf("IncrementalUpdate 失败: %v", err)
	}

	// 所有内容都变更，chunker 应被调用
	if len(chunker.chunkedContents) != 1 {
		t.Errorf("chunker 应被调用 1 次，实际 %d 次", len(chunker.chunkedContents))
	}

	// 编译结果应包含修改后的内容
	hasModified := false
	for _, c := range compiled.Chunks {
		if strings.Contains(c.Content, "contentAModified") {
			hasModified = true
		}
	}
	if !hasModified {
		t.Errorf("编译结果应包含修改后的内容 'contentAModified'")
	}
}

// TestCompileWiki_SetsChunkMetadataAndPersists is a characterization test for
// CompileWiki's whole-page path: chunk metadata is populated, embeddings are
// attached, chunks are persisted to the repo, and the result carries a
// content hash. CompileWiki had zero coverage before this; these assertions
// pin the contract that IncrementalUpdate is measured against.
func TestCompileWiki_SetsChunkMetadataAndPersists(t *testing.T) {
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	repo := &mockChunkRepo{}
	compiler := NewIncrementalCompiler(embedder, chunker, repo, time.Hour, 100)

	ctx := context.Background()
	content := "# A\ncontentA\n# B\ncontentB"

	compiled, err := compiler.CompileWiki(ctx, "page-c", "kb-c", content, nil)
	if err != nil {
		t.Fatalf("CompileWiki 失败: %v", err)
	}

	if compiled.WikiPageID != "page-c" || compiled.KnowledgeBaseID != "kb-c" {
		t.Errorf("compiled 归属错误: page=%s kb=%s", compiled.WikiPageID, compiled.KnowledgeBaseID)
	}
	if compiled.ContentHash == "" {
		t.Errorf("ContentHash 应被填充")
	}
	if compiled.EmbeddingVersion != "mock-embedder" {
		t.Errorf("EmbeddingVersion 期望 'mock-embedder'，实际 '%s'", compiled.EmbeddingVersion)
	}
	if len(compiled.Chunks) == 0 {
		t.Fatalf("应产生 chunks")
	}

	for i, c := range compiled.Chunks {
		if c.ID == "" {
			t.Errorf("chunk[%d] ID 应被填充", i)
		}
		if c.WikiPageID != "page-c" || c.KnowledgeBaseID != "kb-c" {
			t.Errorf("chunk[%d] 归属错误: page=%s kb=%s", i, c.WikiPageID, c.KnowledgeBaseID)
		}
		if c.ChunkIndex != i {
			t.Errorf("chunk[%d] ChunkIndex 期望 %d，实际 %d", i, i, c.ChunkIndex)
		}
		if c.TokenCount == 0 {
			t.Errorf("chunk[%d] TokenCount 应被填充", i)
		}
		if len(c.Embedding) != 3 {
			t.Errorf("chunk[%d] Embedding 应被填充（dim=3），实际 len=%d", i, len(c.Embedding))
		}
	}

	if len(repo.savedChunks) != len(compiled.Chunks) {
		t.Errorf("repo 应保存全部 chunks，期望 %d，实际 %d", len(compiled.Chunks), len(repo.savedChunks))
	}
}

// TestCompileWiki_CacheHitSkipsRecompile verifies the whole-page cache: a
// second compile with unchanged content returns the cached artifact without
// re-invoking the chunker or embedder.
func TestCompileWiki_CacheHitSkipsRecompile(t *testing.T) {
	chunker := &mockChunker{}
	embedder := &mockEmbedder{}
	repo := &mockChunkRepo{}
	compiler := NewIncrementalCompiler(embedder, chunker, repo, time.Hour, 100)

	ctx := context.Background()
	content := "# A\ncontentA"

	if _, err := compiler.CompileWiki(ctx, "page-h", "kb-h", content, nil); err != nil {
		t.Fatalf("首次 CompileWiki 失败: %v", err)
	}
	firstChunks := len(chunker.chunkedContents)
	firstEmbeds := len(embedder.embeddedTexts)
	if firstChunks == 0 {
		t.Fatalf("首次编译应调用 chunker")
	}

	// Second compile with identical content → cache hit, no re-chunk/re-embed.
	compiled2, err := compiler.CompileWiki(ctx, "page-h", "kb-h", content, nil)
	if err != nil {
		t.Fatalf("二次 CompileWiki 失败: %v", err)
	}
	if len(chunker.chunkedContents) != firstChunks {
		t.Errorf("缓存命中不应再次调用 chunker：期望 %d 次，实际 %d 次", firstChunks, len(chunker.chunkedContents))
	}
	if len(embedder.embeddedTexts) != firstEmbeds {
		t.Errorf("缓存命中不应再次调用 embedder：期望 %d 次，实际 %d 次", firstEmbeds, len(embedder.embeddedTexts))
	}
	if compiled2.ContentHash == "" {
		t.Errorf("缓存返回的 compiled 应携带 ContentHash")
	}
}

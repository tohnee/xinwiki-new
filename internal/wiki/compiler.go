package wiki

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
)

// NewIncrementalCompiler creates a new incremental wiki compiler.
func NewIncrementalCompiler(
	embedder Embedder,
	chunker Chunker,
	chunkRepo ChunkRepository,
	cacheTTL time.Duration,
	cacheMaxEntries int,
) *IncrementalCompiler {
	return &IncrementalCompiler{
		cache:     NewCompilationCache(cacheTTL, cacheMaxEntries),
		embedder:  embedder,
		chunker:   chunker,
		chunkRepo: chunkRepo,
	}
}

// CompileWiki compiles a wiki page, using cache if content hasn't changed.
func (c *IncrementalCompiler) CompileWiki(ctx context.Context, wikiPageID, kbID, content string, metadata map[string]string) (*CompiledWiki, error) {
	startTime := time.Now()
	contentHash := hashContent(content)

	// Check cache first
	if cached := c.cache.Get(wikiPageID); cached != nil {
		if cached.ContentHash == contentHash {
			logger.Infof(ctx, "[wiki] compilation cache hit for page %s (hash=%s)", wikiPageID, contentHash[:8])
			return cached, nil
		}
		logger.Infof(ctx, "[wiki] content changed for page %s, recompiling (old=%s, new=%s)",
			wikiPageID, cached.ContentHash[:8], contentHash[:8])
	}

	// 1. Chunk the content
	chunks, err := c.chunker.Chunk(content, metadata)
	if err != nil {
		return nil, fmt.Errorf("chunking failed: %w", err)
	}

	// 2. Set chunk metadata
	for i, chunk := range chunks {
		if chunk.ID == "" {
			chunk.ID = generateID()
		}
		chunk.WikiPageID = wikiPageID
		chunk.KnowledgeBaseID = kbID
		chunk.ChunkIndex = i
		chunk.LastUpdated = time.Now()
		if chunk.TokenCount == 0 {
			chunk.TokenCount = estimateTokens(chunk.Content)
		}
	}

	// 3. Generate embeddings (batch for efficiency)
	if c.embedder != nil {
		texts := make([]string, len(chunks))
		for i, chunk := range chunks {
			texts[i] = chunk.Content
		}

		embeddings, err := c.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("embedding generation failed: %w", err)
		}

		for i, chunk := range chunks {
			if i < len(embeddings) {
				chunk.Embedding = embeddings[i]
			}
		}
	}

	// 4. Delete old chunks and save new ones
	if err := c.chunkRepo.DeleteChunksByWikiPage(ctx, wikiPageID); err != nil {
		logger.Warnf(ctx, "[wiki] failed to delete old chunks for page %s: %v", wikiPageID, err)
	}

	if err := c.chunkRepo.SaveChunks(ctx, chunks); err != nil {
		return nil, fmt.Errorf("saving chunks failed: %w", err)
	}

	// 5. Create compiled wiki entry
	compiled := &CompiledWiki{
		WikiPageID:       wikiPageID,
		KnowledgeBaseID:  kbID,
		Chunks:           chunks,
		EmbeddingVersion: c.embedder.ModelName(),
		ContentHash:      contentHash,
		CompiledAt:       time.Now(),
	}

	// 6. Update cache
	c.cache.Set(compiled)

	duration := time.Since(startTime)
	logger.Infof(ctx, "[wiki] compiled page %s: %d chunks, %d tokens, duration=%v",
		wikiPageID, len(chunks), totalTokens(chunks), duration)

	return compiled, nil
}

// IncrementalUpdate updates only the changed portions of a wiki page.
//
// Unlike CompileWiki — which re-chunks, re-embeds, and re-writes every chunk
// on any edit — IncrementalUpdate splits old/new content into heading-delimited
// sections, diffs them, and re-processes ONLY the added/modified sections.
// Unchanged sections reuse their existing chunks (fetched from the repository)
// without re-chunking or re-embedding, and removed sections are dropped. This
// is the section-level incremental compilation that CompileWiki's cache cannot
// provide: the cache is whole-page (any edit invalidates all chunks), whereas
// this path keeps untouched sections' embeddings intact.
func (c *IncrementalCompiler) IncrementalUpdate(ctx context.Context, wikiPageID, kbID, oldContent, newContent string, metadata map[string]string) (*CompiledWiki, error) {
	startTime := time.Now()

	oldSections := splitIntoSections(oldContent)
	newSections := splitIntoSections(newContent)
	diff := diffSections(oldSections, newSections)

	// Titles that must be re-processed (added or modified).
	changedTitles := make(map[string]bool, len(diff.added)+len(diff.modified))
	for _, s := range diff.added {
		changedTitles[s.title] = true
	}
	for _, s := range diff.modified {
		changedTitles[s.title] = true
	}

	// Existing chunks for this page, bucketed by section title, so unchanged
	// sections can be reused verbatim. A repository error is non-fatal: we
	// fall back to materializing a chunk from the section content below.
	existingBySection := make(map[string][]*Chunk)
	if existing, err := c.chunkRepo.GetChunksByWikiPage(ctx, wikiPageID); err == nil {
		for _, ch := range existing {
			existingBySection[ch.Section] = append(existingBySection[ch.Section], ch)
		}
	}

	var result []*Chunk
	chunkIndex := 0
	for _, sec := range newSections {
		if !changedTitles[sec.title] {
			// Unchanged section: reuse existing chunks. If none are stored
			// (e.g. first incremental update after a cache miss, or the page
			// was never compiled through this repo), materialize a single
			// chunk from the section content so the section stays queryable
			// without paying for a re-chunk/re-embed.
			if reused := existingBySection[sec.title]; len(reused) > 0 {
				for _, ch := range reused {
					ch.ChunkIndex = chunkIndex
					chunkIndex++
				}
				result = append(result, reused...)
				continue
			}
			ch := &Chunk{
				ID:              generateID(),
				KnowledgeBaseID: kbID,
				WikiPageID:      wikiPageID,
				Content:         sec.content,
				Section:         sec.title,
				ChunkIndex:      chunkIndex,
				TokenCount:      estimateTokens(sec.content),
				LastUpdated:     time.Now(),
			}
			chunkIndex++
			result = append(result, ch)
			continue
		}

		// Changed section: re-chunk and re-embed.
		chunks, err := c.chunker.Chunk(sec.content, metadata)
		if err != nil {
			return nil, fmt.Errorf("chunking section %q failed: %w", sec.title, err)
		}
		for _, ch := range chunks {
			if ch.ID == "" {
				ch.ID = generateID()
			}
			ch.WikiPageID = wikiPageID
			ch.KnowledgeBaseID = kbID
			ch.Section = sec.title
			ch.ChunkIndex = chunkIndex
			chunkIndex++
			ch.LastUpdated = time.Now()
			if ch.TokenCount == 0 {
				ch.TokenCount = estimateTokens(ch.Content)
			}
		}
		if c.embedder != nil {
			texts := make([]string, len(chunks))
			for i, ch := range chunks {
				texts[i] = ch.Content
			}
			embeddings, err := c.embedder.EmbedBatch(ctx, texts)
			if err != nil {
				return nil, fmt.Errorf("embedding section %q failed: %w", sec.title, err)
			}
			for i, ch := range chunks {
				if i < len(embeddings) {
					ch.Embedding = embeddings[i]
				}
			}
		}
		result = append(result, chunks...)
	}

	// Persist the new chunk set. We rewrite the whole page's chunks: the
	// reused chunks are re-saved unchanged, changed sections' chunks replace
	// their old versions, and removed sections' chunks are gone.
	if err := c.chunkRepo.DeleteChunksByWikiPage(ctx, wikiPageID); err != nil {
		logger.Warnf(ctx, "[wiki] incremental: failed to delete old chunks for page %s: %v", wikiPageID, err)
	}
	if err := c.chunkRepo.SaveChunks(ctx, result); err != nil {
		return nil, fmt.Errorf("saving incremental chunks failed: %w", err)
	}

	contentHash := hashContent(newContent)
	compiled := &CompiledWiki{
		WikiPageID:       wikiPageID,
		KnowledgeBaseID:  kbID,
		Chunks:           result,
		EmbeddingVersion: embedderName(c.embedder),
		ContentHash:      contentHash,
		CompiledAt:       time.Now(),
	}
	c.cache.Set(compiled)

	duration := time.Since(startTime)
	logger.Infof(ctx, "[wiki] incremental update page %s: %d sections (%d added, %d modified, %d removed), %d chunks, duration=%v",
		wikiPageID, len(newSections), len(diff.added), len(diff.modified), len(diff.removed), len(result), duration)
	return compiled, nil
}

// embedderName returns the embedder's model name, or "" when no embedder is
// configured. Mirrors the EmbeddingVersion population in CompileWiki without
// risking a nil-pointer dereference on the nil-embedder path.
func embedderName(e Embedder) string {
	if e == nil {
		return ""
	}
	return e.ModelName()
}

// InvalidateCache removes a wiki page from the compilation cache.
func (c *IncrementalCompiler) InvalidateCache(wikiPageID string) {
	c.cache.Invalidate(wikiPageID)
}

// GetCached returns a cached compiled wiki if available.
func (c *IncrementalCompiler) GetCached(wikiPageID string) *CompiledWiki {
	return c.cache.Get(wikiPageID)
}

// Precompile pre-compiles a set of wiki pages, useful for warm-up.
func (c *IncrementalCompiler) Precompile(ctx context.Context, pages []struct {
	WikiPageID, KBID, Content string
	Metadata                  map[string]string
}) error {
	for _, p := range pages {
		_, err := c.CompileWiki(ctx, p.WikiPageID, p.KBID, p.Content, p.Metadata)
		if err != nil {
			logger.Warnf(ctx, "[wiki] precompile failed for page %s: %v", p.WikiPageID, err)
		}
	}
	return nil
}

// hashContent creates a SHA-256 hash of content for cache validation.
func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// estimateTokens provides a rough token estimate (4 chars ≈ 1 token for English/Chinese mix).
func estimateTokens(text string) int {
	charCount := len([]rune(text))
	return charCount / 3
}

func totalTokens(chunks []*Chunk) int {
	total := 0
	for _, c := range chunks {
		total += c.TokenCount
	}
	return total
}

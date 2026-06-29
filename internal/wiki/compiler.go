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
		WikiPageID:     wikiPageID,
		KnowledgeBaseID: kbID,
		Chunks:         chunks,
		EmbeddingVersion: c.embedder.ModelName(),
		ContentHash:    contentHash,
		CompiledAt:     time.Now(),
	}

	// 6. Update cache
	c.cache.Set(compiled)

	duration := time.Since(startTime)
	logger.Infof(ctx, "[wiki] compiled page %s: %d chunks, %d tokens, duration=%v",
		wikiPageID, len(chunks), totalTokens(chunks), duration)

	return compiled, nil
}

// IncrementalUpdate updates only changed portions of a wiki page.
func (c *IncrementalCompiler) IncrementalUpdate(ctx context.Context, wikiPageID, kbID, oldContent, newContent string, metadata map[string]string) (*CompiledWiki, error) {
	// For now, fall back to full compilation
	// In a more sophisticated implementation, we would diff and only recompile changed sections
	return c.CompileWiki(ctx, wikiPageID, kbID, newContent, metadata)
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
func (c *IncrementalCompiler) Precompile(ctx context.Context, pages []struct{ WikiPageID, KBID, Content string; Metadata map[string]string }) error {
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

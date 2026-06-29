package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

type memoryCacheIndex struct {
	entries map[uint64][]*types.SemanticCacheEntry
	mu      sync.RWMutex
	config  types.SemanticCacheConfig
	stats   struct {
		hits    int64
		misses  int64
	}
}

type MemorySemanticCache struct {
	idx *memoryCacheIndex
}

func NewMemorySemanticCache(config types.SemanticCacheConfig) *MemorySemanticCache {
	return &MemorySemanticCache{
		idx: &memoryCacheIndex{
			entries: make(map[uint64][]*types.SemanticCacheEntry),
			config:  config,
		},
	}
}

func (m *MemorySemanticCache) Get(ctx context.Context, tenantID uint64, kbIDs []string, queryEmbedding []float32, threshold float64) (*types.SemanticCacheEntry, error) {
	m.idx.mu.RLock()
	defer m.idx.mu.RUnlock()

	if threshold <= 0 {
		threshold = m.idx.config.SimilarityThreshold
	}

	entries, ok := m.idx.entries[tenantID]
	if !ok {
		m.idx.stats.misses++
		return nil, nil
	}

	now := time.Now()
	var bestMatch *types.SemanticCacheEntry
	bestScore := 0.0

	for _, entry := range entries {
		if now.After(entry.ExpiresAt) {
			continue
		}

		kbMatch := false
		if len(kbIDs) == len(entry.KnowledgeBaseIDs) {
			kbMatch = true
			sortedKBs := make([]string, len(kbIDs))
			copy(sortedKBs, kbIDs)
			sort.Strings(sortedKBs)
			sortedEntryKBs := make([]string, len(entry.KnowledgeBaseIDs))
			copy(sortedEntryKBs, entry.KnowledgeBaseIDs)
			sort.Strings(sortedEntryKBs)
			for i := range sortedKBs {
				if sortedKBs[i] != sortedEntryKBs[i] {
					kbMatch = false
					break
				}
			}
		}
		if !kbMatch {
			continue
		}

		score := cosineSimilarity(queryEmbedding, entry.QueryEmbedding)
		if score >= threshold && score > bestScore {
			bestScore = score
			bestMatch = entry
		}
	}

	if bestMatch != nil {
		m.idx.stats.hits++
		bestMatch.HitCount++
		return bestMatch, nil
	}

	m.idx.stats.misses++
	return nil, nil
}

func (m *MemorySemanticCache) Set(ctx context.Context, entry *types.SemanticCacheEntry) error {
	m.idx.mu.Lock()
	defer m.idx.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateEntryID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.ExpiresAt.IsZero() {
		entry.ExpiresAt = time.Now().Add(m.idx.config.TTL)
	}

	entries := m.idx.entries[entry.TenantID]
	now := time.Now()
	validEntries := make([]*types.SemanticCacheEntry, 0, len(entries)+1)
	for _, e := range entries {
		if now.Before(e.ExpiresAt) {
			validEntries = append(validEntries, e)
		}
	}

	validEntries = append(validEntries, entry)

	if len(validEntries) > m.idx.config.MaxEntries {
		validEntries = validEntries[len(validEntries)-m.idx.config.MaxEntries:]
	}

	m.idx.entries[entry.TenantID] = validEntries
	return nil
}

func (m *MemorySemanticCache) InvalidateByKB(ctx context.Context, tenantID uint64, kbID string) error {
	m.idx.mu.Lock()
	defer m.idx.mu.Unlock()

	entries, ok := m.idx.entries[tenantID]
	if !ok {
		return nil
	}

	validEntries := make([]*types.SemanticCacheEntry, 0, len(entries))
	for _, entry := range entries {
		hasKB := false
		for _, id := range entry.KnowledgeBaseIDs {
			if id == kbID {
				hasKB = true
				break
			}
		}
		if !hasKB {
			validEntries = append(validEntries, entry)
		}
	}

	m.idx.entries[tenantID] = validEntries
	return nil
}

func (m *MemorySemanticCache) InvalidateAll(ctx context.Context, tenantID uint64) error {
	m.idx.mu.Lock()
	defer m.idx.mu.Unlock()

	delete(m.idx.entries, tenantID)
	return nil
}

func (m *MemorySemanticCache) Stats(ctx context.Context) (*types.SemanticCacheStats, error) {
	m.idx.mu.RLock()
	defer m.idx.mu.RUnlock()

	totalEntries := 0
	for _, entries := range m.idx.entries {
		totalEntries += len(entries)
	}

	total := m.idx.stats.hits + m.idx.stats.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(m.idx.stats.hits) / float64(total)
	}

	return &types.SemanticCacheStats{
		Enabled:      m.idx.config.Enabled,
		Backend:      "memory",
		TotalEntries: int64(totalEntries),
		TotalHits:    m.idx.stats.hits,
		TotalMisses:  m.idx.stats.misses,
		HitRate:      hitRate,
	}, nil
}

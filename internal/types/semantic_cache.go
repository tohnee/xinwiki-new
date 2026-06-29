package types

import "time"

type SemanticCacheEntry struct {
	ID               string            `json:"id"`
	TenantID         uint64            `json:"tenant_id"`
	KnowledgeBaseIDs []string          `json:"knowledge_base_ids"`
	QueryText        string            `json:"query_text"`
	QueryEmbedding   []float32         `json:"query_embedding"`
	Results          []*SearchResult   `json:"results"`
	ChunkMap         map[string]*Chunk `json:"chunk_map"`
	CreatedAt        time.Time         `json:"created_at"`
	ExpiresAt        time.Time         `json:"expires_at"`
	HitCount         int64             `json:"hit_count"`
}

type SemanticCacheStats struct {
	Enabled      bool    `json:"enabled"`
	Backend      string  `json:"backend"`
	TotalEntries int64   `json:"total_entries"`
	TotalHits    int64   `json:"total_hits"`
	TotalMisses  int64   `json:"total_misses"`
	HitRate      float64 `json:"hit_rate"`
}

type SemanticCacheConfig struct {
	Enabled             bool          `json:"enabled"`
	SimilarityThreshold float64       `json:"similarity_threshold"`
	TTL                 time.Duration `json:"ttl"`
	MaxEntries          int           `json:"max_entries"`
}

func DefaultSemanticCacheConfig() SemanticCacheConfig {
	return SemanticCacheConfig{
		Enabled:             true,
		SimilarityThreshold: 0.95,
		TTL:                 time.Hour,
		MaxEntries:          1000,
	}
}

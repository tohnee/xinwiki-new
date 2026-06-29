package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

const (
	semanticCachePrefix = "sc:"
	semanticCacheIndex  = "sc:t:%d:index"
	semanticCacheEntry  = "sc:t:%d:e:%s"
	semanticCacheKBSet  = "sc:t:%d:kb:%s"
	semanticCacheHits   = "sc:stats:hits"
	semanticCacheMisses = "sc:stats:misses"
)

type RedisSemanticCache struct {
	redis  *redis.Client
	config types.SemanticCacheConfig
}

func NewRedisSemanticCache(redisClient *redis.Client, config types.SemanticCacheConfig) *RedisSemanticCache {
	return &RedisSemanticCache{
		redis:  redisClient,
		config: config,
	}
}

func (r *RedisSemanticCache) Get(ctx context.Context, tenantID uint64, kbIDs []string, queryEmbedding []float32, threshold float64) (*types.SemanticCacheEntry, error) {
	if threshold <= 0 {
		threshold = r.config.SimilarityThreshold
	}

	kbKey := generateCacheKey(tenantID, kbIDs)
	indexKey := fmt.Sprintf(semanticCacheIndex, tenantID)

	now := time.Now()
	var bestMatch *types.SemanticCacheEntry
	bestScore := 0.0

	var cursor uint64
	for {
		keys, nextCursor, err := r.redis.HScan(ctx, indexKey, cursor, "*", 100).Result()
		if err != nil {
			if err == redis.Nil {
				r.incrementMiss(ctx)
				return nil, nil
			}
			return nil, fmt.Errorf("scan cache index: %w", err)
		}
		cursor = nextCursor

		for i := 0; i < len(keys); i += 2 {
			entryID := keys[i]
			entryKBKey := keys[i+1]

			if entryKBKey != kbKey {
				continue
			}

			entryKey := fmt.Sprintf(semanticCacheEntry, tenantID, entryID)
			data, err := r.redis.Get(ctx, entryKey).Bytes()
			if err != nil {
				if err == redis.Nil {
					r.redis.HDel(ctx, indexKey, entryID)
					continue
				}
				logger.Warnf(ctx, "Failed to get cache entry %s: %v", entryID, err)
				continue
			}

			var entry types.SemanticCacheEntry
			if err := json.Unmarshal(data, &entry); err != nil {
				logger.Warnf(ctx, "Failed to unmarshal cache entry %s: %v", entryID, err)
				continue
			}

			if now.After(entry.ExpiresAt) {
				r.redis.Del(ctx, entryKey)
				r.redis.HDel(ctx, indexKey, entryID)
				continue
			}

			score := cosineSimilarity(queryEmbedding, entry.QueryEmbedding)
			if score >= threshold && score > bestScore {
				bestScore = score
				bestMatch = &entry
			}
		}

		if cursor == 0 {
			break
		}
	}

	if bestMatch != nil {
		r.incrementHit(ctx)
		bestMatch.HitCount++
		return bestMatch, nil
	}

	r.incrementMiss(ctx)
	return nil, nil
}

func (r *RedisSemanticCache) Set(ctx context.Context, entry *types.SemanticCacheEntry) error {
	if entry.ID == "" {
		entry.ID = generateEntryID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	if entry.ExpiresAt.IsZero() {
		entry.ExpiresAt = time.Now().Add(r.config.TTL)
	}

	ttl := time.Until(entry.ExpiresAt)
	if ttl <= 0 {
		return nil
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	kbKey := generateCacheKey(entry.TenantID, entry.KnowledgeBaseIDs)
	indexKey := fmt.Sprintf(semanticCacheIndex, entry.TenantID)
	entryKey := fmt.Sprintf(semanticCacheEntry, entry.TenantID, entry.ID)

	pipe := r.redis.Pipeline()
	pipe.Set(ctx, entryKey, data, ttl)
	pipe.HSet(ctx, indexKey, entry.ID, kbKey)
	pipe.Expire(ctx, indexKey, ttl+time.Minute)

	for _, kbID := range entry.KnowledgeBaseIDs {
		kbSetKey := fmt.Sprintf(semanticCacheKBSet, entry.TenantID, kbID)
		pipe.SAdd(ctx, kbSetKey, entry.ID)
		pipe.Expire(ctx, kbSetKey, ttl+time.Minute)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("set cache entry: %w", err)
	}

	return nil
}

func (r *RedisSemanticCache) InvalidateByKB(ctx context.Context, tenantID uint64, kbID string) error {
	kbSetKey := fmt.Sprintf(semanticCacheKBSet, tenantID, kbID)
	indexKey := fmt.Sprintf(semanticCacheIndex, tenantID)

	entryIDs, err := r.redis.SMembers(ctx, kbSetKey).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get kb cache entries: %w", err)
	}

	if len(entryIDs) == 0 {
		return nil
	}

	pipe := r.redis.Pipeline()
	for _, entryID := range entryIDs {
		entryKey := fmt.Sprintf(semanticCacheEntry, tenantID, entryID)
		pipe.Del(ctx, entryKey)
		pipe.HDel(ctx, indexKey, entryID)
	}
	pipe.Del(ctx, kbSetKey)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("invalidate kb cache: %w", err)
	}

	return nil
}

func (r *RedisSemanticCache) InvalidateAll(ctx context.Context, tenantID uint64) error {
	indexKey := fmt.Sprintf(semanticCacheIndex, tenantID)
	pattern := fmt.Sprintf(semanticCacheEntry, tenantID, "*")
	kbPattern := fmt.Sprintf(semanticCacheKBSet, tenantID, "*")

	var cursor uint64
	for {
		keys, nextCursor, err := r.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan entry keys: %w", err)
		}
		cursor = nextCursor
		if len(keys) > 0 {
			r.redis.Del(ctx, keys...)
		}
		if cursor == 0 {
			break
		}
	}

	cursor = 0
	for {
		keys, nextCursor, err := r.redis.Scan(ctx, cursor, kbPattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan kb set keys: %w", err)
		}
		cursor = nextCursor
		if len(keys) > 0 {
			r.redis.Del(ctx, keys...)
		}
		if cursor == 0 {
			break
		}
	}

	r.redis.Del(ctx, indexKey)
	return nil
}

func (r *RedisSemanticCache) Stats(ctx context.Context) (*types.SemanticCacheStats, error) {
	hits, _ := r.redis.Get(ctx, semanticCacheHits).Int64()
	misses, _ := r.redis.Get(ctx, semanticCacheMisses).Int64()

	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	totalEntries := int64(0)
	pattern := "sc:t:*:e:*"
	var cursor uint64
	for {
		keys, nextCursor, err := r.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			break
		}
		cursor = nextCursor
		totalEntries += int64(len(keys))
		if cursor == 0 {
			break
		}
	}

	return &types.SemanticCacheStats{
		Enabled:      r.config.Enabled,
		Backend:      "redis",
		TotalEntries: totalEntries,
		TotalHits:    hits,
		TotalMisses:  misses,
		HitRate:      hitRate,
	}, nil
}

func (r *RedisSemanticCache) incrementHit(ctx context.Context) {
	r.redis.Incr(ctx, semanticCacheHits)
}

func (r *RedisSemanticCache) incrementMiss(ctx context.Context) {
	r.redis.Incr(ctx, semanticCacheMisses)
}

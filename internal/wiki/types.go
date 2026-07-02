// Package wiki provides optimized Wiki compilation, hybrid retrieval, and
// high-precision question answering for XinWiki.
package wiki

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RetrievalMethod represents different retrieval strategies.
type RetrievalMethod string

const (
	MethodBM25   RetrievalMethod = "bm25"
	MethodVector RetrievalMethod = "vector"
	MethodGraph  RetrievalMethod = "graph"
	MethodHybrid RetrievalMethod = "hybrid"
)

// Chunk represents a chunk of indexed wiki content.
type Chunk struct {
	ID            string            `json:"id"`
	KnowledgeBaseID string          `json:"knowledge_base_id"`
	WikiPageID    string            `json:"wiki_page_id"`
	Content       string            `json:"content"`
	Section       string            `json:"section,omitempty"`
	Path          string            `json:"path,omitempty"` // Hierarchical path in wiki
	ChunkIndex    int               `json:"chunk_index"`
	TokenCount    int               `json:"token_count"`
	Embedding     []float32         `json:"-"` // Vector embedding
	Metadata      map[string]string `json:"metadata,omitempty"`
	Score         float64           `json:"score"`
	ScoreBreakdown map[RetrievalMethod]float64 `json:"score_breakdown,omitempty"`
	LastUpdated   time.Time         `json:"last_updated"`
}

// SearchResult represents a ranked search result with citation info.
type SearchResult struct {
	Chunk         *Chunk  `json:"chunk"`
	FinalScore    float64 `json:"final_score"`
	BM25Score     float64 `json:"bm25_score"`
	VectorScore   float64 `json:"vector_score"`
	GraphScore    float64 `json:"graph_score"`
	Rank          int     `json:"rank"`
	CitationCount int     `json:"citation_count"`
}

// RetrievalRequest represents a search/retrieval request.
type RetrievalRequest struct {
	Query          string   `json:"query"`
	TenantID       string   `json:"tenant_id"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	TopK           int      `json:"top_k"`
	Methods        []RetrievalMethod `json:"methods,omitempty"`
	MinScore       float64  `json:"min_score"`
	UseRRF         bool     `json:"use_rrf"` // Use Reciprocal Rank Fusion
	RRFConstant    int      `json:"rrf_constant"`
	UseCache       bool     `json:"use_cache"`
	Filters        map[string]interface{} `json:"filters,omitempty"`
	// History contains the recent conversation turns (oldest first) used for
	// history-aware query rewriting. Nil/empty means standalone query.
	History []Message `json:"history,omitempty"`
}

// RetrievalResponse contains retrieval results and timing info.
type RetrievalResponse struct {
	Results     []*SearchResult `json:"results"`
	Query       string          `json:"query"`
	TotalChunks int             `json:"total_chunks"`
	DurationMs  int64           `json:"duration_ms"`
	Method      RetrievalMethod `json:"method"`
	CacheHit    bool            `json:"cache_hit"`
}

// QueryRewrite represents a rewritten query for better retrieval.
type QueryRewrite struct {
	OriginalQuery  string   `json:"original_query"`
	ExpandedQueries []string `json:"expanded_queries"`
	Entities       []string `json:"entities"`
	Keywords       []string `json:"keywords"`
}

// Citation represents a source citation in an answer.
type Citation struct {
	ID             string `json:"id"`
	ChunkID        string `json:"chunk_id"`
	WikiPageID     string `json:"wiki_page_id"`
	Content        string `json:"content"`
	Section        string `json:"section,omitempty"`
	Path           string `json:"path,omitempty"`
	QuoteStart     int    `json:"quote_start"`
	QuoteEnd       int    `json:"quote_end"`
	Confidence     float64 `json:"confidence"`
}

// ThinkingStep represents a single step in the AI reasoning chain.
type ThinkingStep struct {
	ID          string           `json:"id"`
	StepType    string           `json:"step_type"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Status      string           `json:"status"` // pending, running, completed, error
	DurationMs  int64            `json:"duration_ms"`
	StartTime   time.Time        `json:"start_time"`
	EndTime     time.Time        `json:"end_time"`
	Input       interface{}      `json:"input,omitempty"`
	Output      interface{}      `json:"output,omitempty"`
	Error       string           `json:"error,omitempty"`
	Children    []*ThinkingStep  `json:"children,omitempty"`
}

// QAResponse represents the complete QA response with thinking chain.
type QAResponse struct {
	*Answer
	ThinkingChain *ThinkingStep `json:"thinking_chain,omitempty"`
	TokenUsage    *TokenUsage   `json:"token_usage,omitempty"`
}

// Answer represents a high-precision QA result.
type Answer struct {
	ID              string          `json:"id"`
	Question        string          `json:"question"`
	Answer          string          `json:"answer"`
	Citations       []*Citation     `json:"citations"`
	Confidence      float64         `json:"confidence"`
	GroundingScore  float64         `json:"grounding_score"`
	TokensUsed      int             `json:"tokens_used"`
	ModelUsed       string          `json:"model_used"`
	ThinkingChain   []ReasoningStep `json:"thinking_chain,omitempty"`
	RelatedSearches []string        `json:"related_searches,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// ReasoningStep represents a step in the flat reasoning chain.
type ReasoningStep struct {
	Step        int       `json:"step"`
	Thought     string    `json:"thought"`
	Action      string    `json:"action,omitempty"`
	Observation string    `json:"observation,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	DurationMs  int64     `json:"duration_ms"`
}

// CompilationCache manages cached compiled wiki artifacts.
type CompilationCache struct {
	mu             sync.RWMutex
	cache          map[string]*CompiledWiki
	ttl            time.Duration
	maxEntries     int
}

// CompiledWiki represents a compiled and indexed wiki.
type CompiledWiki struct {
	WikiPageID     string    `json:"wiki_page_id"`
	KnowledgeBaseID string   `json:"knowledge_base_id"`
	Chunks         []*Chunk  `json:"chunks"`
	EmbeddingVersion string  `json:"embedding_version"`
	ContentHash    string    `json:"content_hash"`
	CompiledAt     time.Time `json:"compiled_at"`
	AccessCount    int64     `json:"access_count"`
}

// IncrementalCompiler handles incremental wiki compilation with caching.
type IncrementalCompiler struct {
	cache         *CompilationCache
	embedder      Embedder
	chunker       Chunker
	chunkRepo     ChunkRepository
}

// HybridRetriever combines BM25, vector, and graph retrieval with RRF fusion.
type HybridRetriever struct {
	bm25Retriever  BM25Retriever
	vectorRetriever VectorRetriever
	graphRetriever GraphRetriever
	cache          *RetrievalCache
	queryRewriter  QueryRewriter
	reranker       Reranker
}

// RetrievalCache caches retrieval results for common queries.
type RetrievalCache struct {
	mu         sync.RWMutex
	cache      map[string]*cachedRetrieval
	ttl        time.Duration
	maxEntries int
}

type cachedRetrieval struct {
	response   *RetrievalResponse
	expiresAt  time.Time
}

// QAEngine provides high-precision question answering with citation support.
type QAEngine struct {
	retriever      *HybridRetriever
	llm            LLMClient
	citationVerifier CitationVerifier
	confidenceScorer ConfidenceScorer
}

// --- Interfaces for pluggable implementations ---

// Embedder generates vector embeddings for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	ModelName() string
	Dimension() int
}

// Chunker splits content into chunks.
type Chunker interface {
	Chunk(content string, metadata map[string]string) ([]*Chunk, error)
	ChunkSize() int
	ChunkOverlap() int
}

// BM25Retriever performs keyword-based retrieval.
type BM25Retriever interface {
	Search(ctx context.Context, query string, kbIDs []string, topK int, filters map[string]interface{}) ([]*SearchResult, error)
	IndexDocument(ctx context.Context, chunks []*Chunk) error
	RemoveDocument(ctx context.Context, chunkIDs []string) error
}

// VectorRetriever performs semantic vector retrieval.
type VectorRetriever interface {
	Search(ctx context.Context, queryEmbedding []float32, kbIDs []string, topK int, filters map[string]interface{}) ([]*SearchResult, error)
	IndexDocument(ctx context.Context, chunks []*Chunk) error
	RemoveDocument(ctx context.Context, chunkIDs []string) error
}

// GraphRetriever performs knowledge graph traversal retrieval.
type GraphRetriever interface {
	Search(ctx context.Context, entities []string, kbIDs []string, topK int, depth int) ([]*SearchResult, error)
	Expand(ctx context.Context, chunkIDs []string, depth int) ([]*SearchResult, error)
}

// QueryRewriter expands and rewrites queries for better retrieval.
type QueryRewriter interface {
	Rewrite(ctx context.Context, query string) (*QueryRewrite, error)
	RewriteWithContext(ctx context.Context, query string, history []Message) (*QueryRewrite, error)
	ExtractEntities(ctx context.Context, query string) ([]string, error)
}

// Reranker re-scores and reorders retrieved results relative to the query.
// Rerank returns a new slice (in sorted order) of the top results. Implementations
// MUST be safe to call with a nil/empty input and must always return a valid slice.
type Reranker interface {
	Rerank(ctx context.Context, query string, results []*SearchResult, topN int) ([]*SearchResult, error)
}

// LLMClient abstracts LLM operations.
type LLMClient interface {
	Complete(ctx context.Context, prompt string, opts ...LLMOption) (string, error)
	Chat(ctx context.Context, messages []Message, opts ...LLMOption) (*ChatResponse, error)
	ModelName() string
}

// LLMOption configures LLM requests.
type LLMOption func(*LLMOptions)

// LLMOptions contains LLM request options.
type LLMOptions struct {
	MaxTokens   int
	Temperature float64
	Stream      bool
	ReasoningEffort string
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	Content string `json:"content"`
	Usage   TokenUsage `json:"usage"`
	Thinking string `json:"thinking,omitempty"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// CitationVerifier validates citations against source content.
type CitationVerifier interface {
	Verify(ctx context.Context, answer string, citations []*Citation, chunks []*Chunk) ([]*Citation, float64, error)
	CheckGrounding(ctx context.Context, claim string, context string) (float64, error)
}

// ConfidenceScorer computes answer confidence.
type ConfidenceScorer interface {
	Score(ctx context.Context, answer string, results []*SearchResult) (float64, error)
	Calibrate(ctx context.Context, score float64, metadata map[string]interface{}) float64
}

// ChunkRepository provides chunk data access.
type ChunkRepository interface {
	SaveChunks(ctx context.Context, chunks []*Chunk) error
	GetChunksByWikiPage(ctx context.Context, wikiPageID string) ([]*Chunk, error)
	DeleteChunksByWikiPage(ctx context.Context, wikiPageID string) error
	GetChunk(ctx context.Context, chunkID string) (*Chunk, error)
	GetChunks(ctx context.Context, chunkIDs []string) ([]*Chunk, error)
}

// --- Implementations ---

// NewCompilationCache creates a new compilation cache.
func NewCompilationCache(ttl time.Duration, maxEntries int) *CompilationCache {
	return &CompilationCache{
		cache:      make(map[string]*CompiledWiki),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

func (c *CompilationCache) Get(wikiPageID string) *CompiledWiki {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if entry, ok := c.cache[wikiPageID]; ok {
		if time.Since(entry.CompiledAt) < c.ttl {
			entry.AccessCount++
			return entry
		}
	}
	return nil
}

func (c *CompilationCache) Set(wiki *CompiledWiki) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= c.maxEntries {
		c.evict()
	}
	c.cache[wiki.WikiPageID] = wiki
}

func (c *CompilationCache) Invalidate(wikiPageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, wikiPageID)
}

func (c *CompilationCache) evict() {
	// Simple LRU eviction based on last access
	var oldestKey string
	var oldestTime time.Time = time.Now()
	for k, v := range c.cache {
		if v.CompiledAt.Before(oldestTime) {
			oldestTime = v.CompiledAt
			oldestKey = k
		}
	}
	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// NewRetrievalCache creates a new retrieval cache.
func NewRetrievalCache(ttl time.Duration, maxEntries int) *RetrievalCache {
	return &RetrievalCache{
		cache:      make(map[string]*cachedRetrieval),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

func (c *RetrievalCache) Get(key string) *RetrievalResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if entry, ok := c.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		resp := *entry.response
		resp.CacheHit = true
		return &resp
	}
	return nil
}

func (c *RetrievalCache) Set(key string, resp *RetrievalResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= c.maxEntries {
		// Evict expired entries first
		for k, v := range c.cache {
			if time.Now().After(v.expiresAt) {
				delete(c.cache, k)
			}
		}
	}
	c.cache[key] = &cachedRetrieval{
		response:  resp,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// generateID creates a new UUID.
func generateID() string {
	return uuid.New().String()
}

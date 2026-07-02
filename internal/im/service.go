package im

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/config"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/ratelimit"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	defaultRateLimitWindow      = 60 * time.Second
	defaultRateLimitMaxRequests = 10
)

// ── Redis key prefixes ──────────────────────────────────────────────────────
const (
	RedisKeyLeader     = "im:ws:leader:"
	RedisKeyDedup      = "im:dedup:"
	RedisKeyStop       = "im:stop:"
	RedisKeyInflight   = "im:inflight:"
	RedisKeyQueueUser  = "im:queue:user:"
	RedisKeyRateLimit  = "im:ratelimit:"
	RedisKeyGlobalGate = "im:global:active"
)

// channelState holds runtime state for a running IM channel.
type channelState struct {
	Channel      *IMChannel
	Adapter      Adapter
	Cancel       context.CancelFunc
	leaderCancel context.CancelFunc
}

// AdapterFactory creates an Adapter from an IMChannel configuration.
type AdapterFactory func(ctx context.Context, channel *IMChannel, msgHandler func(ctx context.Context, msg *IncomingMessage) error) (Adapter, context.CancelFunc, error)

// inflightEntry tracks a running QA request, keyed by userKey in the inflight map.
type inflightEntry struct {
	cancel             context.CancelFunc
	sessionID          string
	assistantMessageID string
}

// Service orchestrates IM message handling:
// 1. Receives a unified IncomingMessage from an Adapter
// 2. Resolves or creates a XinWiki session for the IM channel
// 3. Dispatches slash-commands (/help, /kb, /clear, etc.) without entering QA
// 4. Calls the XinWiki QA pipeline for normal messages
// 5. Collects the streaming answer and sends it back via the Adapter
type Service struct {
	db             *gorm.DB
	sessionService interfaces.SessionService
	messageService interfaces.MessageService
	tenantService  interfaces.TenantService
	agentService   interfaces.CustomAgentService

	knowledgeService interfaces.KnowledgeService

	kbService interfaces.KnowledgeBaseService

	modelService interfaces.ModelService

	streamManager interfaces.StreamManager

	defaultFileSvc interfaces.FileService

	cmdRegistry *CommandRegistry

	channels map[string]*channelState
	mu       sync.RWMutex

	adapterFactories map[string]AdapterFactory

	processedMsgs sync.Map

	rateLimiter  *ratelimit.Limiter
	rateLimitMax int

	inflight sync.Map

	qaQueue *qaQueue

	redis *redis.Client

	instanceID string

	stopCh chan struct{}
}

// NewService creates a new IM service.
func NewService(
	db *gorm.DB,
	sessionService interfaces.SessionService,
	messageService interfaces.MessageService,
	tenantService interfaces.TenantService,
	agentService interfaces.CustomAgentService,
	knowledgeService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
	modelService interfaces.ModelService,
	streamManager interfaces.StreamManager,
	defaultFileSvc interfaces.FileService,
	redisClient *redis.Client,
	appCfg *config.Config,
) *Service {
	workers, maxQueue, maxPerUser, globalMaxWorkers, rlWindow, rlMax := resolveIMConfig(appCfg)

	registry := NewCommandRegistry()
	registry.Register(newHelpCommand(registry))
	registry.Register(newInfoCommand(kbService))
	registry.Register(newSearchCommand(sessionService, kbService))
	registry.Register(newStopCommand())
	registry.Register(newClearCommand())

	instanceID := uuid.New().String()

	s := &Service{
		db:               db,
		sessionService:   sessionService,
		messageService:   messageService,
		tenantService:    tenantService,
		agentService:     agentService,
		knowledgeService: knowledgeService,
		kbService:        kbService,
		modelService:     modelService,
		streamManager:    streamManager,
		defaultFileSvc:   defaultFileSvc,
		cmdRegistry:      registry,
		channels:         make(map[string]*channelState),
		adapterFactories: make(map[string]AdapterFactory),
		rateLimiter:      ratelimit.New(redisClient, RedisKeyRateLimit, rlWindow, instanceID),
		rateLimitMax:     rlMax,
		redis:            redisClient,
		instanceID:       instanceID,
		stopCh:           make(chan struct{}),
	}

	s.qaQueue = newQAQueue(workers, maxQueue, maxPerUser, globalMaxWorkers, s.executeQARequest, redisClient)
	s.qaQueue.Start(s.stopCh)

	if redisClient == nil {
		go s.dedupCleanupLoop()
	}
	go s.rateLimiter.StartCleanup(s.stopCh)

	if redisClient != nil {
		globalInfo := "unlimited"
		if globalMaxWorkers > 0 {
			globalInfo = fmt.Sprintf("%d", globalMaxWorkers)
		}
		logger.Infof(context.Background(), "[IM] Multi-instance mode enabled (instance=%s, workers=%d, queue=%d, global_max=%s)",
			s.instanceID[:8], workers, maxQueue, globalInfo)
	} else {
		logger.Infof(context.Background(), "[IM] Single-instance mode (no Redis, workers=%d, queue=%d)",
			workers, maxQueue)
	}

	return s
}

// RegisterAdapterFactory registers a factory for creating adapters for a given platform.
func (s *Service) RegisterAdapterFactory(platform string, factory AdapterFactory) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adapterFactories[platform] = factory
}

// Stop gracefully shuts down the service.
func (s *Service) Stop() {
	close(s.stopCh)
	s.qaQueue.Stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, cs := range s.channels {
		s.stopChannelLocked(id, cs)
	}
}

// dedupCleanupLoop periodically cleans up expired entries from the dedup map.
func (s *Service) dedupCleanupLoop() {
	ticker := time.NewTicker(dedupCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().Add(-dedupTTL)
			s.processedMsgs.Range(func(key, value interface{}) bool {
				if t, ok := value.(time.Time); ok && t.Before(cutoff) {
					s.processedMsgs.Delete(key)
				}
				return true
			})
		case <-s.stopCh:
			return
		}
	}
}

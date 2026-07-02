package im

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

const (
	stopMarkerTTL    = 30 * time.Second
	stopPollInterval = 500 * time.Millisecond
)

func (s *Service) checkAndClearStopMarker(ctx context.Context, userKey string) bool {
	if s.redis == nil {
		return false
	}
	stopKey := RedisKeyStop + userKey
	deleted, err := s.redis.Del(ctx, stopKey).Result()
	if err != nil {
		return false
	}
	return deleted > 0
}

func (s *Service) storeInflightMapping(ctx context.Context, userKey, sessionID, messageID string) {
	if s.redis == nil {
		return
	}
	val := sessionID + ":" + messageID
	if err := s.redis.Set(ctx, RedisKeyInflight+userKey, val, 10*time.Minute).Err(); err != nil {
		logger.Warnf(ctx, "[IM] Failed to store inflight mapping: %v", err)
	}
}

func (s *Service) clearInflightMapping(ctx context.Context, userKey string) {
	if s.redis == nil {
		return
	}
	s.redis.Del(ctx, RedisKeyInflight+userKey)
}

func (s *Service) loadInflightMapping(ctx context.Context, userKey string) (sessionID, messageID string, ok bool) {
	if s.redis == nil {
		return "", "", false
	}
	val, err := s.redis.Get(ctx, RedisKeyInflight+userKey).Result()
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(val, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (s *Service) writeStopEvent(ctx context.Context, sessionID, messageID string) {
	stopEvt := interfaces.StreamEvent{
		ID:        fmt.Sprintf("stop-%d", time.Now().UnixNano()),
		Type:      types.ResponseType(event.EventStop),
		Content:   "",
		Done:      true,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"session_id": sessionID,
			"message_id": messageID,
			"reason":     "user_requested",
			"source":     "im",
		},
	}
	if err := s.streamManager.AppendEvent(ctx, sessionID, messageID, stopEvt); err != nil {
		logger.Warnf(ctx, "[IM] Failed to write stop event to StreamManager: %v", err)
	}
}

func (s *Service) watchStreamManagerStop(ctx context.Context, sessionID, messageID string, cancel context.CancelFunc) {
	ticker := time.NewTicker(stopPollInterval)
	defer ticker.Stop()

	offset := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			events, newOffset, err := s.streamManager.GetEvents(ctx, sessionID, messageID, offset)
			if err != nil {
				continue
			}
			for _, evt := range events {
				if evt.Type == types.ResponseType(event.EventStop) {
					logger.Infof(ctx, "[IM] Stop event from StreamManager, cancelling: session=%s message=%s",
						sessionID, messageID)
					cancel()
					return
				}
			}
			offset = newOffset
		}
	}
}

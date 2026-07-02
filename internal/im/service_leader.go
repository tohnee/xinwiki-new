package im

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/redis/go-redis/v9"
)

const (
	wsLeaderTTL           = 15 * time.Second
	wsLeaderRenewInterval = 5 * time.Second
	wsLeaderRetryInterval = 10 * time.Second
)

func (s *Service) tryAcquireWSLeader(channelID string) bool {
	if s.redis == nil {
		return true
	}
	key := RedisKeyLeader + channelID
	ok, err := s.redis.SetNX(context.Background(), key, s.instanceID, wsLeaderTTL).Result()
	if err != nil {
		logger.Warnf(context.Background(), "[IM] Redis leader election failed for %s: %v, assuming leader", channelID, err)
		return true
	}
	return ok
}

func (s *Service) releaseWSLeader(channelID string) {
	if s.redis == nil {
		return
	}
	key := RedisKeyLeader + channelID
	script := redis.NewScript(`
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('DEL', KEYS[1])
		end
		return 0
	`)
	script.Run(context.Background(), s.redis, []string{key}, s.instanceID)
}

func (s *Service) wsLeaderRenewLoop(ctx context.Context, channelID string) {
	key := RedisKeyLeader + channelID
	ticker := time.NewTicker(wsLeaderRenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			script := redis.NewScript(`
				if redis.call('GET', KEYS[1]) == ARGV[1] then
					redis.call('PEXPIRE', KEYS[1], ARGV[2])
					return 1
				end
				return 0
			`)
			result, err := script.Run(ctx, s.redis, []string{key}, s.instanceID, wsLeaderTTL.Milliseconds()).Int64()
			if err != nil || result == 0 {
				logger.Warnf(context.Background(),
					"[IM] Lost leadership for channel %s, stopping adapter", channelID)
				s.StopChannel(channelID)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) wsLeaderRetryLoop(channel *IMChannel) {
	ticker := time.NewTicker(wsLeaderRetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, _, ok := s.GetChannelAdapter(channel.ID); ok {
				return
			}
			if s.tryAcquireWSLeader(channel.ID) {
				logger.Infof(context.Background(),
					"[IM] Acquired leadership for channel %s, starting adapter", channel.ID)
				s.mu.RLock()
				factory, ok := s.adapterFactories[channel.Platform]
				s.mu.RUnlock()
				if !ok {
					return
				}
				if err := s.startChannelInternal(channel, factory); err != nil {
					logger.Warnf(context.Background(),
						"[IM] Failed to start channel %s after acquiring leadership: %v", channel.ID, err)
				}
				return
			}
		case <-s.stopCh:
			return
		}
	}
}

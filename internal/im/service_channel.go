package im

import (
	"context"
	"fmt"

	"github.com/Tencent/XinWiki/internal/logger"
)

func (s *Service) LoadAndStartChannels() error {
	ctx := context.Background()
	var channels []IMChannel
	if err := s.db.Where("enabled = ? AND deleted_at IS NULL", true).Find(&channels).Error; err != nil {
		return fmt.Errorf("load im channels: %w", err)
	}

	for i := range channels {
		ch := channels[i]
		if err := s.StartChannel(&ch); err != nil {
			logger.Warnf(ctx, "[IM] Failed to start channel %s (%s/%s): %v", ch.ID, ch.Platform, ch.Name, err)
		} else {
			logger.Infof(ctx, "[IM] Started channel: id=%s platform=%s name=%s mode=%s agent=%s",
				ch.ID, ch.Platform, ch.Name, ch.Mode, ch.AgentID)
		}
	}

	logger.Infof(ctx, "[IM] Loaded %d enabled channels", len(channels))
	return nil
}

func (s *Service) StartChannel(channel *IMChannel) error {
	s.mu.Lock()
	factory, ok := s.adapterFactories[channel.Platform]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("no adapter factory for platform: %s", channel.Platform)
	}
	if existing, ok := s.channels[channel.ID]; ok {
		s.stopChannelLocked(channel.ID, existing)
	}
	s.mu.Unlock()

	if (channel.Mode == "websocket" || channel.Mode == "longpoll") && s.redis != nil {
		acquired := s.tryAcquireWSLeader(channel.ID)
		if !acquired {
			logger.Infof(context.Background(),
				"[IM] Channel %s %s owned by another instance, will retry", channel.ID, channel.Mode)
			go s.wsLeaderRetryLoop(channel)
			return nil
		}
	}

	return s.startChannelInternal(channel, factory)
}

func (s *Service) startChannelInternal(channel *IMChannel, factory AdapterFactory) error {
	msgHandler := func(msgCtx context.Context, msg *IncomingMessage) error {
		return s.HandleMessage(msgCtx, msg, channel.ID)
	}

	ctx := context.Background()
	adapter, cancelFn, err := factory(ctx, channel, msgHandler)
	if err != nil {
		s.releaseWSLeader(channel.ID)
		return fmt.Errorf("create adapter: %w", err)
	}

	var leaderCancel context.CancelFunc
	if (channel.Mode == "websocket" || channel.Mode == "longpoll") && s.redis != nil {
		leaderCtx, lCancel := context.WithCancel(context.Background())
		leaderCancel = lCancel
		go s.wsLeaderRenewLoop(leaderCtx, channel.ID)
	}

	s.mu.Lock()
	s.channels[channel.ID] = &channelState{
		Channel:      channel,
		Adapter:      adapter,
		Cancel:       cancelFn,
		leaderCancel: leaderCancel,
	}
	s.mu.Unlock()

	return nil
}

func (s *Service) StopChannel(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cs, ok := s.channels[channelID]; ok {
		s.stopChannelLocked(channelID, cs)
	}
}

func (s *Service) stopChannelLocked(channelID string, cs *channelState) {
	if cs.leaderCancel != nil {
		cs.leaderCancel()
	}
	if cs.Cancel != nil {
		cs.Cancel()
	}
	delete(s.channels, channelID)
	if cs.Channel != nil && cs.Channel.Mode == "longpoll" {
		logger.Infof(context.Background(), "[IM] Stopped longpoll channel: id=%s (leader lock will expire via TTL)", channelID)
	} else {
		s.releaseWSLeader(channelID)
		logger.Infof(context.Background(), "[IM] Stopped channel: id=%s", channelID)
	}
}

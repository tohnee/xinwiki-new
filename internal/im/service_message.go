package im

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

func (s *Service) GetChannelAdapter(channelID string) (Adapter, *IMChannel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs, ok := s.channels[channelID]
	if !ok {
		return nil, nil, false
	}
	return cs.Adapter, cs.Channel, true
}

func (s *Service) GetChannelByID(channelID string) (*IMChannel, error) {
	var ch IMChannel
	if err := s.db.Where("id = ? AND deleted_at IS NULL", channelID).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *Service) GetChannelByIDAndTenant(channelID string, tenantID uint64) (*IMChannel, error) {
	var ch IMChannel
	if err := s.db.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", channelID, tenantID).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func (s *Service) isDuplicate(ctx context.Context, messageID string) bool {
	if s.redis != nil {
		key := RedisKeyDedup + messageID
		ok, err := s.redis.SetNX(ctx, key, "1", dedupTTL).Result()
		if err == nil {
			return !ok
		}
		logger.Errorf(ctx, "[IM] Redis dedup failed (fail-closed, message dropped): %v", err)
		return true
	}
	_, loaded := s.processedMsgs.LoadOrStore(messageID, time.Now())
	return loaded
}

func (s *Service) HandleMessage(ctx context.Context, msg *IncomingMessage, channelID string) error {
	if msg.MessageID != "" {
		if s.isDuplicate(ctx, msg.MessageID) {
			logger.Infof(ctx, "[IM] Skipping duplicate message: %s", msg.MessageID)
			return nil
		}
	}

	contentRunes := []rune(msg.Content)
	if len(contentRunes) > maxContentLength {
		logger.Warnf(ctx, "[IM] Message too long (%d runes), truncating to %d", len(contentRunes), maxContentLength)
		msg.Content = string(contentRunes[:maxContentLength])
	}

	adapter, channel, ok := s.GetChannelAdapter(channelID)
	if !ok {
		ch, err := s.GetChannelByID(channelID)
		if err != nil {
			return fmt.Errorf("channel not found: %s", channelID)
		}
		if err := s.StartChannel(ch); err != nil {
			return fmt.Errorf("start channel %s: %w", channelID, err)
		}
		adapter, channel, ok = s.GetChannelAdapter(channelID)
		if !ok {
			return fmt.Errorf("channel adapter not available after start: %s", channelID)
		}
	}

	threadID := ""
	if channel.SessionMode == string(SessionModeThread) {
		threadID = msg.ThreadID
	}

	isCommand := s.cmdRegistry.IsRegistered(msg.Content)
	if !isCommand {
		rateLimitKey := makeUserKey(channelID, msg.UserID, msg.ChatID, threadID)
		if !s.rateLimiter.Allow(ctx, rateLimitKey, s.rateLimitMax) {
			logger.Warnf(ctx, "[IM] Rate limited: channel=%s user=%s chat=%s", channelID, msg.UserID, msg.ChatID)
			_ = adapter.SendReply(ctx, msg, &ReplyMessage{
				Content: "您的消息发送过于频繁，请稍后再试。",
				IsFinal: true,
			})
			return nil
		}
	}

	tenantID := channel.TenantID
	agentID := channel.AgentID

	logger.Infof(ctx, "[IM] HandleMessage: channel=%s platform=%s user=%s chat=%s msgtype=%s content_len=%d",
		channelID, msg.Platform, msg.UserID, msg.ChatID, msg.MessageType, len(msg.Content))
	logger.Debugf(ctx, "[IM] HandleMessage detail: msgid=%s filekey=%s filename=%s",
		msg.MessageID, msg.FileKey, msg.FileName)

	if (msg.MessageType == MessageTypeFile || msg.MessageType == MessageTypeImage) && channel.KnowledgeBaseID != "" {
		return s.handleFileMessage(ctx, msg, adapter, channel)
	}

	if msg.Content == "" && (msg.MessageType == MessageTypeImage || msg.MessageType == MessageTypeFile) {
		logger.Infof(ctx, "[IM] Skipping QA for non-text message without content: type=%s", msg.MessageType)
		if err := adapter.SendReply(ctx, msg, &ReplyMessage{
			Content: "当前渠道未配置文件知识库，无法处理图片/文件消息。请在渠道设置中配置文件知识库后再发送，或直接用文字描述您的问题。",
			IsFinal: true,
		}); err != nil {
			logger.Warnf(ctx, "[IM] Failed to send non-text hint reply: %v", err)
		}
		return nil
	}

	tenant, err := s.tenantService.GetTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("get tenant: %w", err)
	}
	sessionCtx := context.WithValue(ctx, types.TenantInfoContextKey, tenant)
	sessionCtx = withIMIdentity(sessionCtx, tenantID)

	channelSession, err := s.resolveSession(sessionCtx, msg, tenantID, agentID, channelID, channel.SessionMode)
	if err != nil {
		return fmt.Errorf("resolve session: %w", err)
	}

	var customAgent *types.CustomAgent
	if agentID != "" {
		agent, err := s.agentService.GetAgentByID(sessionCtx, agentID)
		if err != nil {
			logger.Warnf(ctx, "[IM] Failed to get agent %s: %v, using default", agentID, err)
		} else {
			customAgent = agent
		}
	}

	if cmd, args, ok := s.cmdRegistry.Parse(msg.Content); ok {
		return s.handleCommand(sessionCtx, cmd, args, msg, adapter, channel, channelSession, customAgent)
	}
	if LooksLikeCommand(msg.Content) {
		_ = adapter.SendReply(ctx, msg, &ReplyMessage{
			Content: "未知指令，发送 `/help` 查看所有可用指令。",
			IsFinal: true,
		})
		return nil
	}

	session, err := s.sessionService.GetSession(sessionCtx, channelSession.SessionID)
	if err != nil {
		if isSessionNotFound(err) {
			logger.Warnf(ctx, "[IM] Session %s not found (deleted?), recycling stale channel session %s",
				channelSession.SessionID, channelSession.ID)
			if delErr := s.db.Delete(&ChannelSession{}, "id = ?", channelSession.ID).Error; delErr != nil {
				logger.Warnf(ctx, "[IM] Failed to delete stale channel session %s: %v", channelSession.ID, delErr)
			}
			channelSession, err = s.resolveSession(sessionCtx, msg, tenantID, agentID, channelID, channel.SessionMode)
			if err != nil {
				return fmt.Errorf("resolve session (retry): %w", err)
			}
			session, err = s.sessionService.GetSession(sessionCtx, channelSession.SessionID)
			if err != nil {
				return fmt.Errorf("get session (retry): %w", err)
			}
		} else {
			return fmt.Errorf("get session: %w", err)
		}
	}

	if session.Title == "" && strings.TrimSpace(msg.Content) != "" {
		sessionForTitle := *session
		titleModelID := ""
		if customAgent != nil && customAgent.Config.ModelID != "" {
			titleModelID = customAgent.Config.ModelID
		}
		s.sessionService.GenerateTitleAsync(sessionCtx, &sessionForTitle, msg.Content, titleModelID, nil)
	}

	s.persistIMLastRequestState(sessionCtx, session.ID, agentID, customAgent, nil)

	qaCtx, qaCancel := context.WithCancel(sessionCtx)
	userKey := makeUserKey(channelID, msg.UserID, msg.ChatID, threadID)

	req := &qaRequest{
		ctx:       qaCtx,
		cancel:    qaCancel,
		msg:       msg,
		session:   session,
		agent:     customAgent,
		adapter:   adapter,
		channel:   channel,
		channelID: channelID,
		tenant:    tenant,
		userKey:   userKey,
	}

	pos, enqueueErr := s.qaQueue.Enqueue(req)
	if enqueueErr != nil {
		qaCancel()
		logger.Warnf(ctx, "[IM] Queue rejected: user=%s reason=%v", msg.UserID, enqueueErr)
		_ = adapter.SendReply(ctx, msg, &ReplyMessage{
			Content: "当前排队人数较多，请稍后再试。",
			IsFinal: true,
		})
		return nil
	}

	if pos > 0 {
		logger.Infof(ctx, "[IM] Enqueued: user=%s pos=%d depth=%d", msg.UserID, pos, s.qaQueue.Metrics().Depth)
		queueMsg := fmt.Sprintf("收到，前面还有 %d 条消息在处理，请稍候 ⏳", pos)
		if s.redis != nil {
			queueMsg = "收到，当前排队中，请稍候 ⏳"
		}
		_ = adapter.SendReply(ctx, msg, &ReplyMessage{
			Content: queueMsg,
			IsFinal: true,
		})
	} else {
		logger.Infof(ctx, "[IM] Enqueued: user=%s pos=0 (immediate)", msg.UserID)
	}

	return nil
}

func (s *Service) persistIMLastRequestState(ctx context.Context, sessionID, agentID string, customAgent *types.CustomAgent, kbIDs []string) {
	state := buildIMLastRequestState(agentID, customAgent, kbIDs)
	if err := s.sessionService.UpdateSessionLastRequestState(logger.CloneContext(context.WithoutCancel(ctx)), sessionID, state); err != nil {
		logger.Warnf(ctx, "[IM] persist last_request_state failed for session %s: %v", sessionID, err)
	}
}

func (s *Service) executeQARequest(req *qaRequest) {
	ctx := req.ctx
	defer req.cancel()

	entry := &inflightEntry{cancel: req.cancel}
	s.inflight.Store(req.userKey, entry)
	defer s.inflight.Delete(req.userKey)

	if s.checkAndClearStopMarker(ctx, req.userKey) {
		logger.Infof(ctx, "[IM] Request cancelled by remote /stop before execution: %s", req.userKey)
		return
	}

	var kbIDs []string

	streamDisabled := req.channel.OutputMode == "full"

	if !streamDisabled {
		if streamer, ok := req.adapter.(StreamSender); ok {
			if err := s.handleMessageStream(ctx, req.msg, req.session, req.agent, kbIDs, streamer, req.adapter, req.userKey, req.tenant); err != nil {
				logger.Errorf(ctx, "[IM] Stream QA failed: %v", err)
			}
			return
		}
	}

	answer, err := s.runQA(ctx, req.session, req.msg.Content, req.agent, kbIDs, req.userKey, req.msg.Quote)
	if err != nil {
		logger.Errorf(ctx, "[IM] QA failed: %v, sending fallback reply", err)
		answer = "抱歉，处理您的问题时出现了异常，请稍后再试。"
	}

	reply := &ReplyMessage{
		Content: formatIMOutboundAnswer(ctx, answer, req.tenant, s.defaultFileSvc),
		IsFinal: true,
	}
	if err := req.adapter.SendReply(ctx, req.msg, reply); err != nil {
		logger.Errorf(ctx, "[IM] Send reply failed: %v", err)
		return
	}

	logger.Infof(ctx, "[IM] Reply sent: channel=%s platform=%s user=%s answer_len=%d",
		req.channelID, req.msg.Platform, req.msg.UserID, len(answer))
}

func (s *Service) handleCommand(
	ctx context.Context,
	cmd Command,
	args []string,
	msg *IncomingMessage,
	adapter Adapter,
	channel *IMChannel,
	channelSession *ChannelSession,
	customAgent *types.CustomAgent,
) error {
	agentName := ""
	if customAgent != nil {
		agentName = customAgent.Name
	}

	cmdCtx := &CommandContext{
		Incoming:          msg,
		Session:           channelSession,
		TenantID:          channel.TenantID,
		AgentName:         agentName,
		CustomAgent:       customAgent,
		ChannelOutputMode: channel.OutputMode,
	}

	result, err := cmd.Execute(ctx, cmdCtx, args)
	if err != nil {
		logger.Errorf(ctx, "[IM] Command /%s error: %v", cmd.Name(), err)
		_ = adapter.SendReply(ctx, msg, &ReplyMessage{
			Content: "抱歉，执行指令时出现了异常，请稍后再试。",
			IsFinal: true,
		})
		return err
	}

	switch result.Action {
	case ActionClear:
		if err := s.db.Model(&ChannelSession{}).
			Where("id = ?", channelSession.ID).
			Update("deleted_at", time.Now()).Error; err != nil {
			logger.Warnf(ctx, "[IM] Failed to soft-delete channel session: %v", err)
		}
	case ActionStop:
		stopThreadID := ""
		if channel.SessionMode == string(SessionModeThread) {
			stopThreadID = msg.ThreadID
		}
		inflightKey := makeUserKey(channel.ID, msg.UserID, msg.ChatID, stopThreadID)

		var localSessionID, localMessageID string
		localStopped := s.qaQueue.Remove(inflightKey)
		if localStopped {
			logger.Infof(ctx, "[IM] Cancelled queued QA: key=%s", inflightKey)
		} else if raw, loaded := s.inflight.LoadAndDelete(inflightKey); loaded {
			e := raw.(*inflightEntry)
			e.cancel()
			localStopped = true
			localSessionID = e.sessionID
			localMessageID = e.assistantMessageID
			logger.Infof(ctx, "[IM] Cancelled in-flight QA: key=%s", inflightKey)
		}

		sessionID, messageID := localSessionID, localMessageID
		if sessionID == "" || messageID == "" {
			sessionID, messageID, _ = s.loadInflightMapping(ctx, inflightKey)
		}
		if sessionID != "" && messageID != "" {
			s.writeStopEvent(ctx, sessionID, messageID)
			logger.Infof(ctx, "[IM] Wrote stop event to StreamManager: session=%s message=%s", sessionID, messageID)
		}

		if s.redis != nil {
			s.redis.Set(ctx, RedisKeyStop+inflightKey, "1", stopMarkerTTL)
		}

		if !localStopped && sessionID == "" {
			logger.Infof(ctx, "[IM] Set cross-instance stop marker (no inflight found): key=%s", inflightKey)
		}
	}

	sent := false
	if channel.OutputMode != "full" {
		if streamer, ok := adapter.(StreamSender); ok {
			if err := s.sendStreamReply(ctx, msg, streamer, result.Content); err != nil {
				logger.Warnf(ctx, "[IM] Stream reply for command /%s failed, falling back: %v", cmd.Name(), err)
			} else {
				sent = true
			}
		}
	}
	if !sent {
		_ = adapter.SendReply(ctx, msg, &ReplyMessage{
			Content: result.Content,
			IsFinal: true,
		})
	}

	logger.Infof(ctx, "[IM] Command /%s executed: channel=%s user=%s action=%d",
		cmd.Name(), channel.ID, msg.UserID, result.Action)
	return nil
}

func (s *Service) sendStreamReply(ctx context.Context, msg *IncomingMessage, streamer StreamSender, content string) error {
	streamID, err := streamer.StartStream(ctx, msg)
	if err != nil {
		return fmt.Errorf("start stream: %w", err)
	}
	if err := streamer.UpdateStreamContent(ctx, msg, streamID, content); err != nil {
		return fmt.Errorf("update stream content: %w", err)
	}
	if err := streamer.FinalizeStream(ctx, msg, streamID, content); err != nil {
		return fmt.Errorf("finalize stream: %w", err)
	}
	if err := streamer.EndStream(ctx, msg, streamID); err != nil {
		return fmt.Errorf("end stream: %w", err)
	}
	return nil
}

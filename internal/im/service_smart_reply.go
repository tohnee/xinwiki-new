package im

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/types"
)

const smartReplySystemPrompt = "你是一个专业的 IM 机器人助手。请根据以下事件情况，生成一条简洁、清晰的通知消息。" +
	"要求：1) 可适当使用 emoji 但不要过多；2) 语气专业平等，像同事之间对话，不要谄媚讨好，不要用「啦」「哦」「呢」「哟」等撒娇语气词；" +
	"3) 直接输出消息内容，不要加任何额外解释；" +
	"4) 如果事件中包含摘要或详细内容，请用 Markdown 格式结构化展示（使用标题、列表、加粗等），完整呈现，不要删减或概括；如果是简单通知，则控制在 2-3 句话以内。"

func (s *Service) sendFileResult(ctx context.Context, adapter Adapter, msg *IncomingMessage, fileName string, success bool, errDetail string, channel *IMChannel) {
	typeName := fileTypeName(fileName)

	var fallback string
	if success {
		fallback = fmt.Sprintf("✅ %s已保存到知识库，正在解析中，完成后会通知你～", typeName)
	} else {
		fallback = fmt.Sprintf("❌ %s处理失败：%s", typeName, errDetail)
	}

	var situation string
	if success {
		situation = fmt.Sprintf("用户上传的%s已成功保存到知识库，但还需要后台解析文档内容（这需要一些时间）。请告知用户文件已收到，正在解析处理中，解析完成后会自动推送结果。", typeName)
	} else {
		situation = fmt.Sprintf("用户上传的%s处理失败，原因：%s。", typeName, errDetail)
	}

	if err := s.sendSmartReply(ctx, adapter, msg, channel, situation, fallback); err != nil {
		logger.Warnf(ctx, "[IM] Failed to send file result notification: %v", err)
	}
}

func (s *Service) sendSmartReply(ctx context.Context, adapter Adapter, msg *IncomingMessage, channel *IMChannel, situation string, fallback string) error {
	chatModel := s.getChatModelForChannel(ctx, channel)
	if chatModel == nil {
		return adapter.SendReply(ctx, msg, &ReplyMessage{Content: fallback, IsFinal: true})
	}

	if streamer, ok := adapter.(StreamSender); ok {
		if err := s.streamSmartReply(ctx, chatModel, streamer, msg, situation); err == nil {
			return nil
		}
		logger.Warnf(ctx, "[IM] Stream smart reply failed, falling back to non-streaming")
	}

	content := s.generateSmartReply(ctx, chatModel, situation, fallback)
	return adapter.SendReply(ctx, msg, &ReplyMessage{Content: content, IsFinal: true})
}

func (s *Service) streamSmartReply(ctx context.Context, chatModel chat.Chat, streamer StreamSender, msg *IncomingMessage, situation string) error {
	messages := []chat.Message{
		{Role: "system", Content: smartReplySystemPrompt},
		{Role: "user", Content: situation},
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	streamCh, err := chatModel.ChatStream(timeoutCtx, messages, &chat.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})
	if err != nil {
		logger.Warnf(ctx, "[IM] ChatStream failed for smart reply: %v", err)
		return err
	}

	streamID, err := streamer.StartStream(ctx, msg)
	if err != nil {
		logger.Warnf(ctx, "[IM] StartStream failed for smart reply: %v", err)
		return err
	}

	var (
		bufMu     sync.Mutex
		streamRaw strings.Builder
		done      = make(chan struct{})
	)

	go func() {
		defer close(done)
		for resp := range streamCh {
			if resp.Content != "" {
				bufMu.Lock()
				streamRaw.WriteString(resp.Content)
				bufMu.Unlock()
			}
		}
	}()

	ticker := time.NewTicker(streamFlushInterval)
	defer ticker.Stop()

	pushStream := func(phase StreamDisplayPhase) {
		bufMu.Lock()
		raw := streamRaw.String()
		bufMu.Unlock()
		if raw == "" {
			return
		}
		display := FormatIMDisplayContent(raw, phase)
		if err := streamer.UpdateStreamContent(ctx, msg, streamID, display); err != nil {
			logger.Warnf(ctx, "[IM] UpdateStreamContent failed for smart reply: %v", err)
		}
	}

loop:
	for {
		select {
		case <-ticker.C:
			pushStream(StreamDisplayIntermediate)
		case <-done:
			break loop
		case <-timeoutCtx.Done():
			break loop
		}
	}

	bufMu.Lock()
	finalRaw := streamRaw.String()
	bufMu.Unlock()
	finalDisplay := FormatIMDisplayContent(finalRaw, StreamDisplayFinal)
	if finalDisplay == "" {
		finalDisplay = finalRaw
	}
	if err := streamer.FinalizeStream(ctx, msg, streamID, finalDisplay); err != nil {
		logger.Warnf(ctx, "[IM] FinalizeStream failed for smart reply: %v", err)
	}

	if err := streamer.EndStream(ctx, msg, streamID); err != nil {
		logger.Warnf(ctx, "[IM] EndStream failed for smart reply: %v", err)
	}

	return nil
}

func (s *Service) generateSmartReply(ctx context.Context, chatModel chat.Chat, situation string, fallback string) string {
	messages := []chat.Message{
		{Role: "system", Content: smartReplySystemPrompt},
		{Role: "user", Content: situation},
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := chatModel.Chat(timeoutCtx, messages, &chat.ChatOptions{
		Temperature: 0.7,
		MaxTokens:   800,
	})
	if err != nil {
		logger.Warnf(ctx, "[IM] Smart reply generation failed, using fallback: %v", err)
		return fallback
	}

	reply := strings.TrimSpace(resp.Content)
	if reply == "" {
		return fallback
	}
	return reply
}

func (s *Service) getChatModelForChannel(ctx context.Context, channel *IMChannel) chat.Chat {
	if channel == nil || channel.AgentID == "" {
		return nil
	}

	if _, ok := types.TenantIDFromContext(ctx); !ok && channel.TenantID != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, channel.TenantID)
	}

	agent, err := s.agentService.GetAgentByID(ctx, channel.AgentID)
	if err != nil || agent == nil {
		logger.Debugf(ctx, "[IM] Cannot get agent %s for smart reply: %v", channel.AgentID, err)
		return nil
	}

	modelID := agent.Config.ModelID
	if modelID == "" {
		return nil
	}

	chatModel, err := s.modelService.GetChatModel(ctx, modelID)
	if err != nil {
		logger.Debugf(ctx, "[IM] Cannot get chat model %s for smart reply: %v", modelID, err)
		return nil
	}
	return chatModel
}

package im

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
	"time"

	agenttools "github.com/Tencent/XinWiki/internal/agent/tools"
	"github.com/Tencent/XinWiki/internal/config"
	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

func makeUserKey(channelID, userID, chatID, threadID string) string {
	if threadID != "" {
		return fmt.Sprintf("%s:%s:%s:%s", channelID, userID, chatID, threadID)
	}
	return fmt.Sprintf("%s:%s:%s", channelID, userID, chatID)
}

var nonTextTypeLabel = map[string]string{
	"image": "图片",
	"file":  "文件",
	"video": "视频",
	"voice": "语音",
}

func formatQuotedContext(quote *QuotedMessage) string {
	if quote == nil {
		return ""
	}
	if quote.NonTextType != "" {
		label := nonTextTypeLabel[quote.NonTextType]
		if label == "" {
			label = "该类型的"
		}
		return "用户引用了一条" + label + "消息，但你无法查看该内容。请直接告知用户你目前无法处理" + label + "消息，建议用户用文字描述问题。不要猜测该消息的内容。"
	}
	if quote.Content == "" {
		return ""
	}
	content := quote.Content
	runes := []rune(content)
	if len(runes) > maxQuoteContentLength {
		content = string(runes[:maxQuoteContentLength]) + "..."
	}
	content = strings.ReplaceAll(content, "</quoted_message>", "")
	label := "以下是用户引用的一条历史消息，仅作为上下文参考："
	if quote.IsBotMessage {
		label = "以下是用户引用的你（机器人）之前的回复，仅作为上下文参考："
	}
	return label + "\n<quoted_message>\n" + content + "\n</quoted_message>"
}

func withIMIdentity(ctx context.Context, tenantID uint64) context.Context {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, fmt.Sprintf("system-%d", tenantID))
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
	return ctx
}

func buildIMQARequest(
	session *types.Session,
	query string,
	assistantMessageID string,
	userMessageID string,
	customAgent *types.CustomAgent,
	kbIDs []string,
	quote *QuotedMessage,
) *types.QARequest {
	webSearchEnabled := customAgent != nil && customAgent.Config.WebSearchEnabled
	quotedContext := formatQuotedContext(quote)
	return &types.QARequest{
		Session:            session,
		Query:              query,
		AssistantMessageID: assistantMessageID,
		CustomAgent:        customAgent,
		KnowledgeBaseIDs:   kbIDs,
		UserMessageID:      userMessageID,
		WebSearchEnabled:   webSearchEnabled,
		QuotedContext:      quotedContext,
	}
}

func buildIMLastRequestState(agentID string, customAgent *types.CustomAgent, kbIDs []string) *types.SessionLastRequestState {
	state := &types.SessionLastRequestState{
		AgentID:          agentID,
		KnowledgeBaseIDs: append([]string(nil), kbIDs...),
	}
	if customAgent == nil {
		return state
	}
	if state.AgentID == "" {
		state.AgentID = customAgent.ID
	}
	state.AgentEnabled = customAgent.IsAgentMode()
	state.ModelID = customAgent.Config.ModelID
	state.WebSearchEnabled = customAgent.Config.WebSearchEnabled
	if len(state.KnowledgeBaseIDs) == 0 && len(customAgent.Config.KnowledgeBases) > 0 {
		state.KnowledgeBaseIDs = append([]string(nil), customAgent.Config.KnowledgeBases...)
	}
	return state
}

func createIMUserMessagePayload(sessionID, content, requestID string) *types.Message {
	return &types.Message{
		SessionID:   sessionID,
		Role:        "user",
		Content:     content,
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: true,
		Channel:     "im",
	}
}

func createIMAssistantMessagePayload(sessionID, requestID string) *types.Message {
	return &types.Message{
		SessionID:   sessionID,
		Role:        "assistant",
		RequestID:   requestID,
		CreatedAt:   time.Now(),
		IsCompleted: false,
		Channel:     "im",
	}
}

func collectIMKnowledgeReferences(dst *[]*types.SearchResult, refs interface{}) {
	switch v := refs.(type) {
	case []*types.SearchResult:
		*dst = append(*dst, v...)
	case []interface{}:
		for _, ref := range v {
			if sr, ok := ref.(*types.SearchResult); ok {
				*dst = append(*dst, sr)
			}
		}
	}
}

func sanitizeIMAgentSteps(raw interface{}) types.AgentSteps {
	switch steps := raw.(type) {
	case []types.AgentStep:
		return types.AgentSteps(agenttools.SanitizeAgentStepsForStorage(steps))
	case types.AgentSteps:
		return types.AgentSteps(agenttools.SanitizeAgentStepsForStorage([]types.AgentStep(steps)))
	default:
		return nil
	}
}

func applyIMCompleteDataToMessage(msg *types.Message, data event.AgentCompleteData) {
	if msg == nil {
		return
	}
	if data.MessageID != "" && data.MessageID != msg.ID {
		return
	}
	msg.IsCompleted = true
	msg.AgentDurationMs = data.TotalDurationMs
	if len(data.KnowledgeRefs) > 0 {
		refs := make([]*types.SearchResult, 0, len(data.KnowledgeRefs))
		collectIMKnowledgeReferences(&refs, data.KnowledgeRefs)
		if len(refs) > 0 {
			msg.KnowledgeReferences = types.References(refs)
		}
	}
	if steps := sanitizeIMAgentSteps(data.AgentSteps); len(steps) > 0 {
		msg.AgentSteps = steps
	}
}

func waitForIMAgentComplete(ctx context.Context, completeDone <-chan struct{}, sessionID string) {
	timer := time.NewTimer(agentCompleteWaitTimeout)
	defer timer.Stop()
	select {
	case <-completeDone:
	case <-ctx.Done():
		logger.Warnf(ctx, "[IM] QA context ended before agent complete event: session=%s", sessionID)
	case <-timer.C:
		logger.Warnf(ctx, "[IM] Timed out waiting for agent complete event: session=%s", sessionID)
	}
}

func pickIMStoredAnswer(candidates ...string) string {
	for _, s := range candidates {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func mergeIMAgentAnswerBuffers(answerBuilder, answerOuter, agentLiveAnswer *strings.Builder, completeFinal string) {
	if answerBuilder.Len() > 0 {
		return
	}
	switch {
	case agentLiveAnswer.Len() > 0:
		live := agentLiveAnswer.String()
		answerBuilder.WriteString(live)
		if answerOuter.Len() == 0 {
			answerOuter.WriteString(live)
		}
	case answerOuter.Len() > 0:
		answerBuilder.WriteString(answerOuter.String())
	case strings.TrimSpace(completeFinal) != "":
		answerBuilder.WriteString(completeFinal)
		answerOuter.WriteString(completeFinal)
	}
}

func resolveIMConfig(appCfg *config.Config) (workers, maxQueue, maxPerUser, globalMaxWorkers int, rlWindow time.Duration, rlMax int) {
	workers = defaultWorkers
	maxQueue = defaultMaxQueueSize
	maxPerUser = defaultMaxPerUser
	rlWindow = defaultRateLimitWindow
	rlMax = defaultRateLimitMaxRequests

	if appCfg == nil || appCfg.IM == nil {
		return
	}
	im := appCfg.IM
	if im.Workers > 0 {
		workers = im.Workers
	}
	if im.MaxQueueSize > 0 {
		maxQueue = im.MaxQueueSize
	}
	if im.MaxPerUser > 0 {
		maxPerUser = im.MaxPerUser
	}
	if im.GlobalMaxWorkers > 0 {
		globalMaxWorkers = im.GlobalMaxWorkers
	}
	if im.RateLimitWindow > 0 {
		rlWindow = im.RateLimitWindow
	}
	if im.RateLimitMax > 0 {
		rlMax = im.RateLimitMax
	}
	return
}

func fileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToLower(parts[len(parts)-1])
}

func imPlatformToChannel(platform string) string {
	switch strings.ToLower(platform) {
	case "wechat":
		return types.ChannelWechat
	case "wecom", "wxwork":
		return types.ChannelWecom
	case "feishu", "lark":
		return types.ChannelFeishu
	case "dingtalk":
		return types.ChannelDingtalk
	case "slack":
		return types.ChannelSlack
	default:
		return types.ChannelIM
	}
}

func fileTypeName(filename string) string {
	switch fileExtension(filename) {
	case "pdf":
		return "PDF 文档"
	case "doc", "docx":
		return "Word 文档"
	case "txt":
		return "文本文件"
	case "md", "markdown":
		return "Markdown 文档"
	case "png", "jpg", "jpeg", "gif":
		return "图片"
	case "csv":
		return "CSV 表格"
	case "xls", "xlsx":
		return "Excel 表格"
	case "ppt", "pptx":
		return "PPT 演示文稿"
	default:
		return "文件"
	}
}

func newInMemoryFileHeader(filename string, data []byte) *multipart.FileHeader {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	h.Set("Content-Type", "application/octet-stream")

	part, err := writer.CreatePart(h)
	if err != nil {
		return &multipart.FileHeader{Filename: filename, Size: int64(len(data))}
	}
	_, _ = part.Write(data)
	_ = writer.Close()

	reader := multipart.NewReader(body, writer.Boundary())
	form, err := reader.ReadForm(int64(len(data)) + 1024)
	if err != nil || form == nil {
		return &multipart.FileHeader{Filename: filename, Size: int64(len(data))}
	}
	files := form.File["file"]
	if len(files) == 0 {
		return &multipart.FileHeader{Filename: filename, Size: int64(len(data))}
	}
	return files[0]
}

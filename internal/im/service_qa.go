package im

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/google/uuid"
)

var internalToolNames = map[string]bool{
	"thinking":   true,
	"todo_write": true,
}

func isToolVisibleToUser(toolName string) bool {
	return !internalToolNames[toolName]
}

func briefToolSummary(output string) string {
	const maxRunes = 40
	if output == "" {
		return ""
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	if output[0] == '{' || output[0] == '[' || output[0] == '<' {
		return ""
	}
	if idx := strings.IndexByte(output, '\n'); idx >= 0 {
		output = strings.TrimSpace(output[:idx])
	}
	if output == "" {
		return ""
	}
	runes := []rune(output)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return output
}

func (s *Service) handleMessageStream(ctx context.Context, msg *IncomingMessage, session *types.Session, customAgent *types.CustomAgent, kbIDs []string, streamer StreamSender, adapter Adapter, userKey string, tenant *types.Tenant) error {
	streamID, err := streamer.StartStream(ctx, msg)
	if err != nil {
		logger.Warnf(ctx, "[IM] StartStream failed, falling back to non-streaming: %v", err)
		return s.fallbackNonStream(ctx, msg, session, customAgent, kbIDs, adapter, userKey, tenant)
	}

	qaCtx, qaCancel := context.WithCancel(ctx)
	defer qaCancel()

	useAgent := customAgent != nil && customAgent.IsAgentMode()
	eventBus := event.NewEventBus()

	var (
		bufMu           sync.Mutex
		reasoningInner  streamSection
		agentInner      streamSection
		agentLiveAnswer strings.Builder
		answerOuter     strings.Builder
		answerBuilder   strings.Builder
		qaErr           error
		done            = make(chan struct{})
		completeDone    = make(chan struct{})
		closeOnce       sync.Once
		completeOnce    sync.Once
		agentDone       bool
		assistantMsg    *types.Message

		seenToolCalls = make(map[string]bool)
		agentToolIdx  = make(map[string]int)
		pipelineIdx   = make(map[string]int)

		agentToolSteps    []IMToolStep
		pipelineToolSteps []IMToolStep

		agentCompleteFinalAnswer string
		streamedAny              bool
	)
	closeDone := func() { closeOnce.Do(func() { close(done) }) }
	closeComplete := func() { completeOnce.Do(func() { close(completeDone) }) }

	agentWrite := func(s string) {
		if s == "" {
			return
		}
		agentInner.write(s)
		streamedAny = true
	}
	reasoningWrite := func(s string) {
		if s == "" {
			return
		}
		reasoningInner.write(s)
		streamedAny = true
	}

	retractAgentLiveAnswer := func() {
		if agentLiveAnswer.Len() == 0 {
			return
		}
		if agentInner.text.Len() > 0 {
			agentInner.ensureNewlineBefore()
		}
		agentInner.write(agentLiveAnswer.String())
		agentLiveAnswer.Reset()
	}

	getStreamParts := func() IMStreamParts {
		mode := IMStreamModeQuickQA
		if useAgent {
			mode = IMStreamModeAgent
		}
		return IMStreamParts{
			Mode:              mode,
			PipelineToolSteps: pipelineToolSteps,
			ReasoningInner:    reasoningInner.text.String(),
			AgentInner:        agentInner.text.String(),
			AgentToolSteps:    agentToolSteps,
			LiveAnswer:        agentLiveAnswer.String(),
			Answer:            answerOuter.String(),
		}
	}

	eventBus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}

		bufMu.Lock()
		if useAgent && !agentDone {
			if data.Content != "" {
				agentLiveAnswer.WriteString(data.Content)
				streamedAny = true
			}
		} else {
			answerOuter.WriteString(data.Content)
			answerBuilder.WriteString(data.Content)
			streamedAny = true
		}
		bufMu.Unlock()

		if data.Done {
			closeDone()
		}
		return nil
	})

	eventBus.On(event.EventError, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.ErrorData)
		if !ok {
			return nil
		}
		logger.Errorf(ctx, "[IM] QA stream error: %s", data.Error)
		bufMu.Lock()
		qaErr = fmt.Errorf("QA pipeline error: %s", data.Error)
		bufMu.Unlock()
		closeDone()
		closeComplete()
		return nil
	})

	eventBus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentReferencesData)
		if !ok {
			return nil
		}
		bufMu.Lock()
		if assistantMsg != nil {
			refs := []*types.SearchResult(assistantMsg.KnowledgeReferences)
			collectIMKnowledgeReferences(&refs, data.References)
			assistantMsg.KnowledgeReferences = types.References(refs)
		}
		bufMu.Unlock()
		return nil
	})

	eventBus.On(event.EventAgentComplete, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentCompleteData)
		if !ok {
			return nil
		}
		bufMu.Lock()
		agentDone = true
		agentCompleteFinalAnswer = data.FinalAnswer
		applyIMCompleteDataToMessage(assistantMsg, data)
		mergeIMAgentAnswerBuffers(&answerBuilder, &answerOuter, &agentLiveAnswer, data.FinalAnswer)
		bufMu.Unlock()
		closeComplete()
		return nil
	})

	eventBus.On(event.EventAgentThought, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentThoughtData)
		if !ok {
			return nil
		}
		bufMu.Lock()
		if useAgent {
			agentWrite(data.Content)
		} else {
			reasoningWrite(data.Content)
		}
		bufMu.Unlock()
		return nil
	})

	eventBus.On(event.EventAgentToolCall, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentToolCallData)
		if !ok {
			return nil
		}
		if !isToolVisibleToUser(data.ToolName) {
			return nil
		}
		bufMu.Lock()
		if seenToolCalls[data.ToolCallID] {
			bufMu.Unlock()
			return nil
		}
		seenToolCalls[data.ToolCallID] = true
		if !useAgent && IsRAGPipelineToolName(data.ToolName) {
			upsertIMToolStep(&pipelineToolSteps, pipelineIdx, data.ToolCallID, func(step *IMToolStep) {
				step.ToolName = data.ToolName
				step.Pending = true
				step.Arguments = data.Arguments
			})
			streamedAny = true
		} else if useAgent {
			retractAgentLiveAnswer()
			upsertIMToolStep(&agentToolSteps, agentToolIdx, data.ToolCallID, func(step *IMToolStep) {
				step.ToolName = data.ToolName
				step.Pending = true
				step.Arguments = data.Arguments
			})
			streamedAny = true
		}
		bufMu.Unlock()
		logger.Debugf(ctx, "[IM] Tool call streamed to IM: tool=%s id=%s", data.ToolName, data.ToolCallID)
		return nil
	})

	eventBus.On(event.EventAgentToolResult, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentToolResultData)
		if !ok {
			return nil
		}
		if !isToolVisibleToUser(data.ToolName) {
			return nil
		}
		bufMu.Lock()
		if !useAgent && IsRAGPipelineToolName(data.ToolName) {
			upsertIMToolStep(&pipelineToolSteps, pipelineIdx, data.ToolCallID, func(step *IMToolStep) {
				step.ToolName = data.ToolName
				step.Pending = false
				step.Success = data.Success
				step.Data = data.Data
				step.Output = data.Output
			})
			streamedAny = true
		} else if useAgent {
			upsertIMToolStep(&agentToolSteps, agentToolIdx, data.ToolCallID, func(step *IMToolStep) {
				step.ToolName = data.ToolName
				step.Pending = false
				step.Success = data.Success
				step.Data = data.Data
				step.Output = data.Output
			})
			streamedAny = true
		}
		bufMu.Unlock()
		logger.Debugf(ctx, "[IM] Tool result streamed to IM: tool=%s success=%v duration=%dms",
			data.ToolName, data.Success, data.Duration)
		return nil
	})

	requestID := uuid.New().String()

	userMsg, err := s.messageService.CreateMessage(qaCtx, createIMUserMessagePayload(session.ID, msg.Content, requestID))
	if err != nil {
		return fmt.Errorf("create user message: %w", err)
	}

	assistantMsg, err = s.messageService.CreateMessage(qaCtx, createIMAssistantMessagePayload(session.ID, requestID))
	if err != nil {
		return fmt.Errorf("create assistant message: %w", err)
	}

	if raw, ok := s.inflight.Load(userKey); ok {
		e := raw.(*inflightEntry)
		e.sessionID = session.ID
		e.assistantMessageID = assistantMsg.ID
	}
	s.storeInflightMapping(qaCtx, userKey, session.ID, assistantMsg.ID)
	defer s.clearInflightMapping(ctx, userKey)

	go s.watchStreamManagerStop(qaCtx, session.ID, assistantMsg.ID, qaCancel)

	go func() {
		var err error
		req := buildIMQARequest(session, msg.Content, assistantMsg.ID, userMsg.ID, customAgent, kbIDs, msg.Quote)
		if req.QuotedContext != "" {
			logger.Debugf(qaCtx, "[IM] QuotedContext set: length=%d", len(req.QuotedContext))
		}
		if useAgent {
			err = s.sessionService.AgentQA(qaCtx, req, eventBus)
		} else {
			err = s.sessionService.KnowledgeQA(qaCtx, req, eventBus)
		}
		if err != nil {
			logger.Errorf(ctx, "[IM] QA stream execution error: %v", err)
			bufMu.Lock()
			qaErr = fmt.Errorf("QA execution error: %w", err)
			bufMu.Unlock()
			closeDone()
			closeComplete()
		}
	}()

	ticker := time.NewTicker(streamFlushInterval)
	defer ticker.Stop()

	flush := func() {
		bufMu.Lock()
		parts := getStreamParts()
		agentRunning := useAgent && !agentDone
		bufMu.Unlock()

		displaySource := FormatIMIntermediateFromParts(parts, agentRunning)
		if displaySource == "" {
			return
		}

		if cut := holdbackCutoff(displaySource); cut < len(displaySource) {
			displaySource = displaySource[:cut]
		}

		display := cleanIMContent(ctx, displaySource, tenant, s.defaultFileSvc)
		if err := streamer.UpdateStreamContent(ctx, msg, streamID, display); err != nil {
			logger.Warnf(ctx, "[IM] UpdateStreamContent failed: %v", err)
		}
	}

loop:
	for {
		select {
		case <-ticker.C:
			flush()
		case <-done:
			break loop
		case <-qaCtx.Done():
			break loop
		}
	}

	if useAgent {
		waitForIMAgentComplete(qaCtx, completeDone, session.ID)
	}

	bufMu.Lock()
	parts := getStreamParts()
	resolvedAnswer := pickIMStoredAnswer(
		answerBuilder.String(),
		answerOuter.String(),
		agentLiveAnswer.String(),
		agentCompleteFinalAnswer,
	)
	if parts.Answer == "" {
		parts.Answer = resolvedAnswer
	}
	answer := resolvedAnswer
	finalErr := qaErr
	noVisibleContent := !streamedAny && strings.TrimSpace(resolvedAnswer) == ""
	bufMu.Unlock()

	finalDisplay := cleanIMContent(ctx, FormatIMFinalFromParts(parts), tenant, s.defaultFileSvc)
	if noVisibleContent || finalDisplay == "" {
		fallback := "抱歉，我暂时无法回答这个问题。"
		if finalErr != nil {
			fallback = "抱歉，处理您的问题时出现了异常，请稍后再试。"
		}
		finalDisplay = fallback
		if answer == "" {
			answer = fallback
		}
	}

	if err := streamer.FinalizeStream(ctx, msg, streamID, finalDisplay); err != nil {
		logger.Warnf(ctx, "[IM] FinalizeStream failed: %v", err)
	}

	if err := streamer.EndStream(ctx, msg, streamID); err != nil {
		logger.Warnf(ctx, "[IM] EndStream failed: %v", err)
	}

	if answer == "" {
		answer = "抱歉，我暂时无法回答这个问题。"
	}

	assistantMsg.Content = answer
	assistantMsg.IsCompleted = true
	if err := s.messageService.UpdateMessage(ctx, assistantMsg); err != nil {
		logger.Warnf(ctx, "[IM] Failed to update assistant message: %v", err)
	}

	logger.Infof(ctx, "[IM] Stream reply sent: platform=%s user=%s answer_len=%d", msg.Platform, msg.UserID, len(answer))
	return nil
}

func (s *Service) fallbackNonStream(ctx context.Context, msg *IncomingMessage, session *types.Session, customAgent *types.CustomAgent, kbIDs []string, adapter Adapter, userKey string, tenant *types.Tenant) error {
	answer, err := s.runQA(ctx, session, msg.Content, customAgent, kbIDs, userKey, msg.Quote)
	if err != nil {
		logger.Errorf(ctx, "[IM] QA fallback failed: %v", err)
		answer = "抱歉，处理您的问题时出现了异常，请稍后再试。"
	}

	return adapter.SendReply(ctx, msg, &ReplyMessage{Content: formatIMOutboundAnswer(ctx, answer, tenant, s.defaultFileSvc), IsFinal: true})
}

func (s *Service) runQA(ctx context.Context, session *types.Session, query string, customAgent *types.CustomAgent, kbIDs []string, userKey string, quote *QuotedMessage) (string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventBus := event.NewEventBus()

	var answerMu sync.Mutex
	var answerBuilder strings.Builder
	var qaErr error
	done := make(chan struct{})
	completeDone := make(chan struct{})
	var closeOnce sync.Once
	var completeOnce sync.Once
	closeDone := func() { closeOnce.Do(func() { close(done) }) }
	closeComplete := func() { completeOnce.Do(func() { close(completeDone) }) }

	eventBus.On(event.EventAgentFinalAnswer, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentFinalAnswerData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		answerBuilder.WriteString(data.Content)
		answerMu.Unlock()
		if data.Done {
			closeDone()
		}
		return nil
	})

	eventBus.On(event.EventError, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.ErrorData)
		if !ok {
			return nil
		}
		logger.Errorf(ctx, "[IM] QA error: %s", data.Error)
		answerMu.Lock()
		qaErr = fmt.Errorf("QA pipeline error: %s", data.Error)
		answerMu.Unlock()
		closeDone()
		closeComplete()
		return nil
	})

	useAgent := customAgent != nil && customAgent.IsAgentMode()

	requestID := uuid.New().String()

	userMsg, err := s.messageService.CreateMessage(ctx, createIMUserMessagePayload(session.ID, query, requestID))
	if err != nil {
		return "", fmt.Errorf("create user message: %w", err)
	}

	assistantMsg, err := s.messageService.CreateMessage(ctx, createIMAssistantMessagePayload(session.ID, requestID))
	if err != nil {
		return "", fmt.Errorf("create assistant message: %w", err)
	}

	eventBus.On(event.EventAgentReferences, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentReferencesData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		refs := []*types.SearchResult(assistantMsg.KnowledgeReferences)
		collectIMKnowledgeReferences(&refs, data.References)
		assistantMsg.KnowledgeReferences = types.References(refs)
		answerMu.Unlock()
		return nil
	})

	eventBus.On(event.EventAgentComplete, func(ctx context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentCompleteData)
		if !ok {
			return nil
		}
		answerMu.Lock()
		applyIMCompleteDataToMessage(assistantMsg, data)
		if answerBuilder.Len() == 0 && strings.TrimSpace(data.FinalAnswer) != "" {
			answerBuilder.WriteString(data.FinalAnswer)
		}
		answerMu.Unlock()
		closeComplete()
		return nil
	})

	if raw, ok := s.inflight.Load(userKey); ok {
		e := raw.(*inflightEntry)
		e.sessionID = session.ID
		e.assistantMessageID = assistantMsg.ID
	}
	s.storeInflightMapping(ctx, userKey, session.ID, assistantMsg.ID)
	defer s.clearInflightMapping(ctx, userKey)

	go s.watchStreamManagerStop(ctx, session.ID, assistantMsg.ID, cancel)

	go func() {
		var err error
		req := buildIMQARequest(session, query, assistantMsg.ID, userMsg.ID, customAgent, kbIDs, quote)
		if req.QuotedContext != "" {
			logger.Debugf(ctx, "[IM] QuotedContext set: length=%d", len(req.QuotedContext))
		}
		if useAgent {
			err = s.sessionService.AgentQA(ctx, req, eventBus)
		} else {
			err = s.sessionService.KnowledgeQA(ctx, req, eventBus)
		}
		if err != nil {
			logger.Errorf(ctx, "[IM] QA execution error: %v", err)
			answerMu.Lock()
			qaErr = fmt.Errorf("QA execution error: %w", err)
			answerMu.Unlock()
			closeDone()
			closeComplete()
		}
	}()

	select {
	case <-done:
		if useAgent {
			waitForIMAgentComplete(ctx, completeDone, session.ID)
		}
	case <-ctx.Done():
		assistantMsg.Content = "抱歉，回答已被取消。"
		assistantMsg.IsCompleted = true
		if updateErr := s.messageService.UpdateMessage(context.WithoutCancel(ctx), assistantMsg); updateErr != nil {
			logger.Warnf(ctx, "[IM] Failed to update cancelled assistant message: %v", updateErr)
		}
		return "", fmt.Errorf("QA cancelled: %w", ctx.Err())
	}

	answerMu.Lock()
	answer := answerBuilder.String()
	qaError := qaErr
	answerMu.Unlock()

	if answer == "" && qaError != nil {
		return "", qaError
	}
	if answer == "" {
		answer = "抱歉，我暂时无法回答这个问题。"
	}

	assistantMsg.Content = answer
	assistantMsg.IsCompleted = true
	if err := s.messageService.UpdateMessage(ctx, assistantMsg); err != nil {
		logger.Warnf(ctx, "[IM] Failed to update assistant message: %v", err)
	}

	return answer, nil
}

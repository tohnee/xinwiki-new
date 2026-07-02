package agent

import (
	"context"
	"strings"

	"github.com/Tencent/XinWiki/internal/agent/runtime"
	agentmemory "github.com/Tencent/XinWiki/internal/agent/memory"
	"github.com/Tencent/XinWiki/internal/agent/skills"
	"github.com/Tencent/XinWiki/internal/agent/thinking"
	agenttoken "github.com/Tencent/XinWiki/internal/agent/token"
	agenttools "github.com/Tencent/XinWiki/internal/agent/tools"
	"github.com/Tencent/XinWiki/internal/common"
	appconfig "github.com/Tencent/XinWiki/internal/config"
	"github.com/Tencent/XinWiki/internal/event"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/tracing/langfuse"
	"github.com/Tencent/XinWiki/internal/types"
)

// AgentEngine is the core engine for running ReAct agents.
//
// History persistence note: the engine is stateless across turns. Conversation
// history is rebuilt from the DB once per turn by the caller
// (see service.LoadAgentHistory) and passed into Execute as llmContext. The
// engine therefore does not maintain its own cache, system-prompt store, or
// cross-turn buffer.
type AgentEngine struct {
	config               *types.AgentConfig
	toolRegistry         *agenttools.ToolRegistry
	chatModel            chat.Chat
	rt                   runtime.Runtime
	eventBus             *event.EventBus
	knowledgeBasesInfo   []*KnowledgeBaseInfo      // Detailed knowledge base information for prompt
	selectedDocs         []*SelectedDocumentInfo   // User-selected documents (via @ mention)
	sessionID            string                    // Session ID for logging and event emission
	systemPromptTemplate string                    // System prompt template (optional, uses default if empty)
	skillsManager        *skills.Manager           // Skills manager for Progressive Disclosure (optional)
	appConfig            *appconfig.Config         // Application config for prompt template resolution (optional)
	imageDescriber       ImageDescriberFunc        // VLM function for describing images in tool results (optional)
	tokenEstimator       *agenttoken.Estimator     // Token estimator for context window management
	memoryConsolidator   *agentmemory.Consolidator // Memory consolidator for LLM-powered summarization (optional)
	lastUsage            types.TokenUsage          // Token usage from the most recent LLM call
	lastSentMsgCount     int                       // Number of messages sent in the most recent LLM call
	thinkingTracker      *thinking.Tracker         // Thinking chain tracker for monitoring
}

// SetRuntime sets an explicit agent runtime (e.g. Anthropic native SDK,
// OpenCode SDK). When set, the runtime is preferred over chatModel for the
// main ReAct LLM calls. chatModel is retained as a fallback and used by
// sub-components (memory consolidator, tool-side summarization) that still
// expect a chat.Chat.
func (e *AgentEngine) SetRuntime(rt runtime.Runtime) {
	e.rt = rt
}

// ImageDescriberFunc generates a text description of an image.
// Signature matches vlm.VLM.Predict so it can be injected without importing the vlm package.
type ImageDescriberFunc func(ctx context.Context, imgBytes []byte, prompt string) (string, error)

// NewAgentEngine creates a new agent engine
func NewAgentEngine(
	config *types.AgentConfig,
	chatModel chat.Chat,
	toolRegistry *agenttools.ToolRegistry,
	eventBus *event.EventBus,
	knowledgeBasesInfo []*KnowledgeBaseInfo,
	selectedDocs []*SelectedDocumentInfo,
	sessionID string,
	systemPromptTemplate string,
) *AgentEngine {
	if eventBus == nil {
		eventBus = event.NewEventBus()
	}
	tokenEst, err := agenttoken.NewEstimator()
	if err != nil {
		return nil
	}
	engine := &AgentEngine{
		config:               config,
		toolRegistry:         toolRegistry,
		chatModel:            chatModel,
		eventBus:             eventBus,
		knowledgeBasesInfo:   knowledgeBasesInfo,
		selectedDocs:         selectedDocs,
		sessionID:            sessionID,
		systemPromptTemplate: systemPromptTemplate,
		tokenEstimator:       tokenEst,
	}

	// Initialize memory consolidator if context window management is configured
	if config.MaxContextTokens > 0 {
		engine.memoryConsolidator = agentmemory.NewConsolidator(
			chatModel, tokenEst, config.MaxContextTokens, 0,
		)
	}

	return engine
}

// NewAgentEngineWithSkills creates a new agent engine with skills support
func NewAgentEngineWithSkills(
	config *types.AgentConfig,
	chatModel chat.Chat,
	toolRegistry *agenttools.ToolRegistry,
	eventBus *event.EventBus,
	knowledgeBasesInfo []*KnowledgeBaseInfo,
	selectedDocs []*SelectedDocumentInfo,
	sessionID string,
	systemPromptTemplate string,
	skillsManager *skills.Manager,
) *AgentEngine {
	engine := NewAgentEngine(
		config,
		chatModel,
		toolRegistry,
		eventBus,
		knowledgeBasesInfo,
		selectedDocs,
		sessionID,
		systemPromptTemplate,
	)
	engine.skillsManager = skillsManager
	return engine
}

// SetAppConfig sets the application config for prompt template resolution.
// This allows the engine to read default prompts from config/prompt_templates/ YAML files.
func (e *AgentEngine) SetAppConfig(cfg *appconfig.Config) {
	e.appConfig = cfg
}

// SetImageDescriber sets the VLM function for generating text descriptions of images
// in tool results. When set, MCP tool result images are automatically analyzed and
// their descriptions are appended to the tool message content.
// This follows the same pattern as Handler.analyzeImageAttachments() in the handler layer.
func (e *AgentEngine) SetImageDescriber(fn ImageDescriberFunc) {
	e.imageDescriber = fn
}

// SetSkillsManager sets the skills manager for the engine
func (e *AgentEngine) SetSkillsManager(manager *skills.Manager) {
	e.skillsManager = manager
}

// GetSkillsManager returns the skills manager
func (e *AgentEngine) GetSkillsManager() *skills.Manager {
	return e.skillsManager
}

// Execute executes the agent with conversation history and streaming output
// All events are emitted to EventBus and handled by subscribers (like Handler layer)
func (e *AgentEngine) Execute(
	ctx context.Context,
	sessionID, messageID, query string,
	llmContext []chat.Message,
	imageURLs ...[]string,
) (*types.AgentState, error) {
	logger.Infof(ctx, "[Agent] Starting execution: session=%s, message=%s, query_len=%d, context_msgs=%d",
		sessionID, messageID, len(query), len(llmContext))
	// Ensure tools are cleaned up after execution
	defer e.toolRegistry.Cleanup(ctx)

	common.PipelineInfo(ctx, "Agent", "execute_start", map[string]interface{}{
		"session_id":   sessionID,
		"message_id":   messageID,
		"query":        query,
		"context_msgs": len(llmContext),
	})

	// Open a top-level Langfuse span so the agent run — including every
	// round's LLM call and every tool execution — groups under a single
	// node in the Langfuse UI instead of being flat children of the HTTP
	// trace. No-op when Langfuse is disabled.
	imgCount := 0
	if len(imageURLs) > 0 {
		imgCount = len(imageURLs[0])
	}
	kbIDs := make([]string, 0, len(e.knowledgeBasesInfo))
	for _, kb := range e.knowledgeBasesInfo {
		if kb != nil {
			kbIDs = append(kbIDs, kb.ID)
		}
	}
	spanCtx, agentSpan := langfuse.GetManager().StartSpan(ctx, langfuse.SpanOptions{
		Name: "agent.execute",
		Input: map[string]interface{}{
			"query":        truncateRunes(query, langfuseQueryPreview),
			"query_len":    len(query),
			"context_msgs": len(llmContext),
			"image_count":  imgCount,
		},
		Metadata: map[string]interface{}{
			"session_id":          sessionID,
			"message_id":          messageID,
			"max_iterations":      e.config.MaxIterations,
			"parallel_tool_calls": e.config.ParallelToolCalls,
			"web_search":          e.config.WebSearchEnabled,
			"multi_turn":          e.config.MultiTurnEnabled,
			"knowledge_base_ids":  kbIDs,
			"allowed_tools":       e.config.AllowedTools,
		},
	})
	ctx = spanCtx

	// Initialize state
	state := &types.AgentState{
		RoundSteps:    []types.AgentStep{},
		KnowledgeRefs: []*types.SearchResult{},
		IsComplete:    false,
		CurrentRound:  0,
	}

	// Initialize thinking chain tracker
	e.thinkingTracker = thinking.NewTracker(sessionID, true)
	if e.chatModel != nil {
		if mid := e.chatModel.GetModelID(); mid != "" {
			e.thinkingTracker.SetModelID(mid)
		}
	}

	// Build system prompt using progressive RAG prompt
	// If skills are enabled, include skills metadata (Level 1 - Progressive Disclosure)
	// Extract user language from context for prompt placeholder
	language := types.LanguageNameFromContext(ctx)
	var systemPrompt string
	if e.skillsManager != nil && e.skillsManager.IsEnabled() {
		skillsMetadata := e.skillsManager.GetAllMetadata()
		systemPrompt = BuildSystemPromptWithOptions(
			e.knowledgeBasesInfo,
			e.config.WebSearchEnabled,
			e.selectedDocs,
			&BuildSystemPromptOptions{
				SkillsMetadata: skillsMetadata,
				Language:       language,
				Config:         e.appConfig,
			},
			e.systemPromptTemplate,
		)
	} else {
		systemPrompt = BuildSystemPromptWithOptions(
			e.knowledgeBasesInfo,
			e.config.WebSearchEnabled,
			e.selectedDocs,
			&BuildSystemPromptOptions{
				Language: language,
				Config:   e.appConfig,
			},
			e.systemPromptTemplate,
		)
	}
	logger.Debugf(ctx, "[Agent] SystemPrompt: %d chars", len(systemPrompt))

	// Initialize messages with history
	var imgs []string
	if len(imageURLs) > 0 {
		imgs = imageURLs[0]
	}
	messages := e.buildMessagesWithLLMContext(systemPrompt, query, sessionID, llmContext, imgs)

	// Get tool definitions for function calling
	tools := e.buildToolsForLLM()
	toolListStr := strings.Join(listToolNames(tools), ", ")
	logger.Infof(ctx, "[Agent] Ready: %d messages, %d tools [%s], %d images",
		len(messages), len(tools), toolListStr, len(imgs))
	common.PipelineInfo(ctx, "Agent", "tools_ready", map[string]interface{}{
		"session_id": sessionID,
		"tool_count": len(tools),
		"tools":      toolListStr,
	})

	_, err := e.executeLoop(ctx, state, query, messages, tools, sessionID, messageID)
	if err != nil {
		logger.Errorf(ctx, "[Agent] Execution failed: %v", err)
		e.eventBus.Emit(ctx, event.Event{
			ID:        generateEventID("error"),
			Type:      event.EventError,
			SessionID: sessionID,
			Data: event.ErrorData{
				Error:     err.Error(),
				Stage:     "agent_execution",
				SessionID: sessionID,
			},
		})
		finishAgentSpan(agentSpan, state, err)
		return nil, err
	}

	logger.Infof(ctx, "[Agent] Completed: %d rounds, %d steps, complete=%v",
		state.CurrentRound, len(state.RoundSteps), state.IsComplete)
	common.PipelineInfo(ctx, "Agent", "execute_complete", map[string]interface{}{
		"session_id": sessionID,
		"rounds":     state.CurrentRound,
		"steps":      len(state.RoundSteps),
		"complete":   state.IsComplete,
	})
	finishAgentSpan(agentSpan, state, nil)
	return state, nil
}

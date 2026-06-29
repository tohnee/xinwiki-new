package types

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"time"
)

// DefaultMaxContextTokens is the default context window budget for agent conversations (200k).
const DefaultMaxContextTokens = 200000

// AgentConfig represents the full agent configuration (used at tenant level and runtime)
// This includes all configuration parameters for agent execution
type AgentConfig struct {
	MaxIterations  int      `json:"max_iterations"`          // Maximum number of ReAct iterations
	AllowedTools   []string `json:"allowed_tools"`           // List of allowed tool names
	Temperature    float64  `json:"temperature"`             // LLM temperature for agent
	KnowledgeBases []string `json:"knowledge_bases"`         // Accessible knowledge base IDs
	KnowledgeIDs   []string `json:"knowledge_ids"`           // Accessible knowledge IDs (individual documents)
	SystemPrompt   string   `json:"system_prompt,omitempty"` // Unified system prompt (uses web_search_status placeholder for dynamic behavior)
	// Deprecated: Use SystemPrompt instead. Kept for backward compatibility during migration.
	SystemPromptWebEnabled  string        `json:"system_prompt_web_enabled,omitempty"`  // Deprecated: Custom prompt when web search is enabled
	SystemPromptWebDisabled string        `json:"system_prompt_web_disabled,omitempty"` // Deprecated: Custom prompt when web search is disabled
	UseCustomSystemPrompt   bool          `json:"use_custom_system_prompt"`             // Whether to use custom system prompt instead of default
	WebSearchEnabled        bool          `json:"web_search_enabled"`                   // Whether web search tool is enabled
	WebSearchMaxResults     int           `json:"web_search_max_results"`               // Maximum number of web search results (default: 5)
	WebSearchProviderID     string        `json:"web_search_provider_id,omitempty"`     // WebSearchProviderEntity ID (resolved from agent config)
	MultiTurnEnabled        bool          `json:"multi_turn_enabled"`                   // Whether multi-turn conversation is enabled
	HistoryTurns            int           `json:"history_turns"`                        // Number of history turns to keep in context
	SearchTargets           SearchTargets `json:"-"`                                    // Pre-computed unified search targets (runtime only)
	// MCP service selection
	MCPSelectionMode string   `json:"mcp_selection_mode"` // MCP selection mode: "all", "selected", "none"
	MCPServices      []string `json:"mcp_services"`       // Selected MCP service IDs (when mode is "selected")
	// Whether to enable thinking mode (for models that support extended thinking)
	Thinking *bool `json:"thinking"`
	// Whether to retrieve knowledge base only when explicitly mentioned with @ (default: false)
	RetrieveKBOnlyWhenMentioned bool `json:"retrieve_kb_only_when_mentioned"`

	// Whether to retain retrieval history (like wiki_read_page results) across turns (default: false)
	RetainRetrievalHistory bool `json:"retain_retrieval_history"`

	// Skills configuration (Progressive Disclosure pattern)
	SkillsEnabled bool     `json:"skills_enabled"` // Whether skills are enabled (default: false)
	SkillDirs     []string `json:"skill_dirs"`     // Directories to search for skills
	AllowedSkills []string `json:"allowed_skills"` // Skill names whitelist (empty = allow all)

	// Runtime-only fields (not persisted)
	VLMModelID string `json:"-"` // VLM model ID for tool result image analysis (set from CustomAgent config)
	// LLM call timeout in seconds (default: 120). Controls the maximum time for a single LLM call.
	LLMCallTimeout int `json:"llm_call_timeout,omitempty"`

	// Maximum character length for tool output (default: 16000).
	// Outputs exceeding this limit are truncated with head + tail preservation.
	MaxToolOutputChars int `json:"max_tool_output_chars,omitempty"`

	// Maximum context window tokens for the agent (default: 200000).
	// The agent compresses older messages to stay within this limit,
	// preserving tool_call/tool_result pairs.
	MaxContextTokens int `json:"max_context_tokens,omitempty"`

	// Whether to execute independent tool calls in parallel (default: false).
	// When enabled and the LLM returns multiple tool calls, they run concurrently via errgroup.
	ParallelToolCalls bool `json:"parallel_tool_calls,omitempty"`
}

// SessionAgentConfig represents session-level agent configuration
// Sessions only store Enabled and KnowledgeBases; other configs are read from Tenant at runtime
type SessionAgentConfig struct {
	AgentModeEnabled bool     `json:"agent_mode_enabled"` // Whether agent mode is enabled for this session
	WebSearchEnabled bool     `json:"web_search_enabled"` // Whether web search is enabled for this session
	KnowledgeBases   []string `json:"knowledge_bases"`    // Accessible knowledge base IDs for this session
	KnowledgeIDs     []string `json:"knowledge_ids"`      // Accessible knowledge IDs (individual documents) for this session
}

// Value implements driver.Valuer interface for AgentConfig
func (c AgentConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner interface for AgentConfig
func (c *AgentConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return nil
	}
	return json.Unmarshal(b, c)
}

// Value implements driver.Valuer interface for SessionAgentConfig
func (c SessionAgentConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements sql.Scanner interface for SessionAgentConfig
func (c *SessionAgentConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return nil
	}
	return json.Unmarshal(b, c)
}

// ResolveSystemPrompt returns the prompt template for the given web search state.
// It uses the unified SystemPrompt field, falling back to deprecated fields for backward compatibility.
func (c *AgentConfig) ResolveSystemPrompt(webSearchEnabled bool) string {
	if c == nil {
		return ""
	}

	// First, try the new unified SystemPrompt field
	if c.SystemPrompt != "" {
		return c.SystemPrompt
	}

	// Fallback to deprecated fields for backward compatibility
	if webSearchEnabled {
		if c.SystemPromptWebEnabled != "" {
			return c.SystemPromptWebEnabled
		}
	} else {
		if c.SystemPromptWebDisabled != "" {
			return c.SystemPromptWebDisabled
		}
	}

	return ""
}

// Tool defines the interface that all agent tools must implement
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// Parameters returns the JSON Schema for the tool's parameters
	Parameters() json.RawMessage

	// Execute runs the tool with the given arguments
	Execute(ctx context.Context, args json.RawMessage) (*ToolResult, error)
}

// Cleanable is an optional interface that tools can implement to release resources.
// Tools implementing this interface will have their Cleanup method called during
// registry cleanup (e.g., at the end of an agent session).
type Cleanable interface {
	Cleanup(ctx context.Context)
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool                   `json:"success"`          // Whether the tool executed successfully
	Output  string                 `json:"output"`           // Human-readable output
	Data    map[string]interface{} `json:"data,omitempty"`   // Structured data for programmatic use
	Error   string                 `json:"error,omitempty"`  // Error message if execution failed
	Images  []string               `json:"images,omitempty"` // Base64 data URIs from tool (e.g. MCP image content)
}

// ToolCall represents a single tool invocation within an agent step
type ToolCall struct {
	ID               string                 `json:"id"`                          // Function call ID from LLM
	Name             string                 `json:"name"`                        // Tool name
	Args             map[string]interface{} `json:"args"`                        // Tool arguments
	Result           *ToolResult            `json:"result"`                      // Execution result (contains Output)
	Reflection       string                 `json:"reflection,omitempty"`        // Agent's reflection on this tool call result (if enabled)
	Duration         int64                  `json:"duration"`                    // Execution time in milliseconds
	ProviderMetadata ToolCallMetadata       `json:"provider_metadata,omitempty"` // Provider-specific tool-call state for replay
}

// ThinkingType represents the type of thinking step in the chain
type ThinkingType string

const (
	ThinkingTypeThought    ThinkingType = "thought"     // LLM思考过程
	ThinkingTypeToolCall   ThinkingType = "tool_call"   // 工具调用
	ThinkingTypeToolResult ThinkingType = "tool_result" // 工具返回结果
	ThinkingTypeObservation ThinkingType = "observation" // 观察结果
	ThinkingTypeFinalAnswer ThinkingType = "final_answer" // 最终答案
)

// TokenUsageDetail represents detailed token usage breakdown
type TokenUsageDetail struct {
	InputTokens         int     `json:"input_tokens"`          // 输入Token数
	OutputTokens        int     `json:"output_tokens"`         // 输出Token数
	CacheCreationTokens int     `json:"cache_creation_tokens"` // 缓存创建Token数
	CacheReadTokens     int     `json:"cache_read_tokens"`     // 缓存读取Token数
	TotalTokens         int     `json:"total_tokens"`          // 总Token数
	Cost                float64 `json:"cost"`                  // 成本（元）
	ModelID             string  `json:"model_id"`              // 使用的模型ID
}

// ThinkingStep represents a single step in the model's thinking chain
type ThinkingStep struct {
	ID        string            `json:"id"`         // 步骤唯一ID
	Type      ThinkingType      `json:"type"`       // 步骤类型
	Content   string            `json:"content"`    // 步骤内容
	Timestamp time.Time         `json:"timestamp"`  // 发生时间
	Duration  int64             `json:"duration_ms"` // 耗时（毫秒）
	Tokens    TokenUsageDetail  `json:"tokens"`     // 本步骤Token使用
	ParentID  string            `json:"parent_id,omitempty"` // 父步骤ID（用于嵌套结构）
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // 扩展元数据
}

// AgentStep represents one iteration of the ReAct loop
type AgentStep struct {
	Iteration int    `json:"iteration"` // Iteration number (0-indexed)
	Thought   string `json:"thought"`   // LLM's reasoning/thinking (Think phase)
	// ReasoningContent stores the OpenAI-protocol reasoning_content emitted by the
	// model in this round. Persisted on AgentStep so cross-turn replay can put it
	// back on the assistant message — required by MiMo / DeepSeek V3.2+ thinking
	// mode, ignored by providers that don't recognize the field.
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls"` // Tools called in this step (Act phase)
	Timestamp        time.Time  `json:"timestamp"`  // When this step occurred
	DurationMs       int64      `json:"duration_ms"` // 本步骤总耗时（毫秒）
	Tokens           TokenUsageDetail `json:"tokens"` // 本步骤Token使用统计
	ThinkingSteps    []ThinkingStep `json:"thinking_steps,omitempty"` // 详细思维链步骤
}

// GetObservations returns observations from all tool calls in this step
// This is a convenience method to maintain backward compatibility
func (s *AgentStep) GetObservations() []string {
	observations := make([]string, 0, len(s.ToolCalls))
	for _, tc := range s.ToolCalls {
		if tc.Result != nil && tc.Result.Output != "" {
			observations = append(observations, tc.Result.Output)
		}
		if tc.Reflection != "" {
			observations = append(observations, "Reflection: "+tc.Reflection)
		}
	}
	return observations
}

// AgentState tracks the execution state of an agent across iterations
type AgentState struct {
	CurrentRound  int             `json:"current_round"`  // Current round number
	RoundSteps    []AgentStep     `json:"round_steps"`    // All steps taken so far in the current round
	IsComplete    bool            `json:"is_complete"`    // Whether agent has finished
	FinalAnswer   string          `json:"final_answer"`   // The final answer to the query
	KnowledgeRefs []*SearchResult `json:"knowledge_refs"` // Collected knowledge references
	// 思维链和统计
	ThinkingChain []ThinkingStep  `json:"thinking_chain"` // 完整思维链（扁平结构，用于前端时间轴展示）
	TotalTokens   TokenUsageDetail `json:"total_tokens"`  // 整个执行过程的总Token和成本
	StartTime     time.Time       `json:"start_time"`    // 执行开始时间
	EndTime       time.Time       `json:"end_time"`      // 执行结束时间
	TotalDuration int64           `json:"total_duration_ms"` // 总耗时（毫秒）
}

// GetTotalCost returns the total cost of agent execution
func (s *AgentState) GetTotalCost() float64 {
	return s.TotalTokens.Cost
}

// AddThinkingStep adds a thinking step to both the step-specific chain and the global chain
func (s *AgentState) AddThinkingStep(step ThinkingStep, roundIndex int) {
	s.ThinkingChain = append(s.ThinkingChain, step)
	if roundIndex >= 0 && roundIndex < len(s.RoundSteps) {
		s.RoundSteps[roundIndex].ThinkingSteps = append(s.RoundSteps[roundIndex].ThinkingSteps, step)
	}
}

// AccumulateTokens accumulates token usage into the total
func (s *AgentState) AccumulateTokens(usage TokenUsageDetail) {
	s.TotalTokens.InputTokens += usage.InputTokens
	s.TotalTokens.OutputTokens += usage.OutputTokens
	s.TotalTokens.CacheCreationTokens += usage.CacheCreationTokens
	s.TotalTokens.CacheReadTokens += usage.CacheReadTokens
	s.TotalTokens.TotalTokens += usage.TotalTokens
	s.TotalTokens.Cost += usage.Cost
	if usage.ModelID != "" {
		s.TotalTokens.ModelID = usage.ModelID
	}
}

// FunctionDefinition represents a function definition for LLM function calling
type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

package thinking

import (
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/google/uuid"
)

// Tracker 思维链追踪器，负责记录和管理Agent执行过程中的所有思维步骤
type Tracker struct {
	mu            sync.RWMutex
	sessionID     string
	steps         []types.ThinkingStep
	currentStep   *types.ThinkingStep
	startTime     time.Time
	modelID       string
	enableTracing bool
}

// NewTracker 创建新的思维链追踪器
func NewTracker(sessionID string, enableTracing bool) *Tracker {
	return &Tracker{
		sessionID:     sessionID,
		steps:         make([]types.ThinkingStep, 0),
		enableTracing: enableTracing,
		startTime:     time.Now(),
	}
}

// SetModelID 设置当前使用的模型ID
func (t *Tracker) SetModelID(modelID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.modelID = modelID
}

// StartStep 开始一个新的思维步骤
func (t *Tracker) StartStep(stepType types.ThinkingType, content string) *types.ThinkingStep {
	if !t.enableTracing {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	step := types.ThinkingStep{
		ID:        uuid.New().String(),
		Type:      stepType,
		Content:   content,
		Timestamp: time.Now(),
		Tokens: types.TokenUsageDetail{
			ModelID: t.modelID,
		},
	}

	if t.currentStep != nil {
		step.ParentID = t.currentStep.ID
	}

	t.steps = append(t.steps, step)
	t.currentStep = &t.steps[len(t.steps)-1]
	return t.currentStep
}

// EndStep 结束当前思维步骤，记录耗时和Token使用
func (t *Tracker) EndStep(tokens *types.TokenUsageDetail) {
	if !t.enableTracing || t.currentStep == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	t.currentStep.Duration = now.Sub(t.currentStep.Timestamp).Milliseconds()

	if tokens != nil {
		t.currentStep.Tokens = *tokens
	}

	t.currentStep = nil
}

// AddThought 添加思考步骤
func (t *Tracker) AddThought(content string, tokens *types.TokenUsageDetail) *types.ThinkingStep {
	step := t.StartStep(types.ThinkingTypeThought, content)
	t.EndStep(tokens)
	return step
}

// AddToolCall 添加工具调用步骤
func (t *Tracker) AddToolCall(toolName string, args interface{}, tokens *types.TokenUsageDetail) *types.ThinkingStep {
	content := "调用工具: " + toolName
	step := t.StartStep(types.ThinkingTypeToolCall, content)
	if step != nil {
		step.Metadata = map[string]interface{}{
			"tool_name": toolName,
			"args":      args,
		}
	}
	t.EndStep(tokens)
	return step
}

// AddToolResult 添加工具返回结果步骤
func (t *Tracker) AddToolResult(toolName string, result string, success bool, tokens *types.TokenUsageDetail) *types.ThinkingStep {
	content := "工具返回结果: " + toolName
	step := t.StartStep(types.ThinkingTypeToolResult, content)
	if step != nil {
		step.Metadata = map[string]interface{}{
			"tool_name": toolName,
			"success":   success,
			"result":    result,
		}
	}
	t.EndStep(tokens)
	return step
}

// AddObservation 添加观察步骤
func (t *Tracker) AddObservation(content string, tokens *types.TokenUsageDetail) *types.ThinkingStep {
	step := t.StartStep(types.ThinkingTypeObservation, content)
	t.EndStep(tokens)
	return step
}

// AddFinalAnswer 添加最终答案步骤
func (t *Tracker) AddFinalAnswer(content string, tokens *types.TokenUsageDetail) *types.ThinkingStep {
	step := t.StartStep(types.ThinkingTypeFinalAnswer, content)
	t.EndStep(tokens)
	return step
}

// GetSteps 获取所有思维步骤
func (t *Tracker) GetSteps() []types.ThinkingStep {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]types.ThinkingStep, len(t.steps))
	copy(result, t.steps)
	return result
}

// GetStepByID 根据ID获取思维步骤
func (t *Tracker) GetStepByID(id string) *types.ThinkingStep {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, step := range t.steps {
		if step.ID == id {
			stepCopy := step
			return &stepCopy
		}
	}
	return nil
}

// GetTotalDuration 获取总耗时
func (t *Tracker) GetTotalDuration() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return time.Since(t.startTime).Milliseconds()
}

// GetTotalTokens 获取总Token使用统计
func (t *Tracker) GetTotalTokens() types.TokenUsageDetail {
	t.mu.RLock()
	defer t.mu.RUnlock()

	total := types.TokenUsageDetail{
		ModelID: t.modelID,
	}

	for _, step := range t.steps {
		total.InputTokens += step.Tokens.InputTokens
		total.OutputTokens += step.Tokens.OutputTokens
		total.CacheCreationTokens += step.Tokens.CacheCreationTokens
		total.CacheReadTokens += step.Tokens.CacheReadTokens
		total.TotalTokens += step.Tokens.TotalTokens
		total.Cost += step.Tokens.Cost
	}

	return total
}

// Reset 重置追踪器，清空所有记录
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.steps = make([]types.ThinkingStep, 0)
	t.currentStep = nil
	t.startTime = time.Now()
}

// IsTracingEnabled 返回是否启用追踪
func (t *Tracker) IsTracingEnabled() bool {
	return t.enableTracing
}

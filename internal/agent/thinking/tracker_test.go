package thinking

import (
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracker_StartStep_ReturnsValidPointer(t *testing.T) {
	tracker := NewTracker("test-session", true)

	step := tracker.StartStep(types.ThinkingTypeThought, "test thought")
	require.NotNil(t, step)
	assert.Equal(t, "test thought", step.Content)
	assert.Equal(t, types.ThinkingTypeThought, step.Type)
}

func TestTracker_EndStep_PersistsDurationAndTokens(t *testing.T) {
	tracker := NewTracker("test-session", true)

	step := tracker.StartStep(types.ThinkingTypeThought, "test thought")
	require.NotNil(t, step)

	time.Sleep(10 * time.Millisecond)

	tokens := &types.TokenUsageDetail{
		ModelID:       "test-model",
		InputTokens:   100,
		OutputTokens:  50,
		TotalTokens:   150,
	}
	tracker.EndStep(tokens)

	steps := tracker.GetSteps()
	require.Len(t, steps, 1)

	assert.Greater(t, steps[0].Duration, int64(0), "duration should be persisted in steps slice")
	assert.Equal(t, 100, steps[0].Tokens.InputTokens, "input tokens should be persisted")
	assert.Equal(t, 50, steps[0].Tokens.OutputTokens, "output tokens should be persisted")
	assert.Equal(t, "test-model", steps[0].Tokens.ModelID, "model ID should be persisted")
}

func TestTracker_AddToolCall_PersistsMetadata(t *testing.T) {
	tracker := NewTracker("test-session", true)

	tracker.AddToolCall("search", map[string]string{"query": "test"}, nil)

	steps := tracker.GetSteps()
	require.Len(t, steps, 1)

	require.NotNil(t, steps[0].Metadata, "metadata should be persisted")
	assert.Equal(t, "search", steps[0].Metadata["tool_name"], "tool_name should be persisted in metadata")
}

func TestTracker_MultipleSteps_ParentChild(t *testing.T) {
	tracker := NewTracker("test-session", true)

	parent := tracker.StartStep(types.ThinkingTypeThought, "parent")
	require.NotNil(t, parent)
	parentID := parent.ID

	child := tracker.StartStep(types.ThinkingTypeToolCall, "child")
	require.NotNil(t, child)
	assert.Equal(t, parentID, child.ParentID, "child should have parent ID set correctly")

	tracker.EndStep(nil) // end child
	tracker.EndStep(nil) // end parent

	steps := tracker.GetSteps()
	require.Len(t, steps, 2)
	assert.Equal(t, "parent", steps[0].Content)
	assert.Equal(t, "child", steps[1].Content)
	assert.Equal(t, parentID, steps[1].ParentID)
}

func TestTracker_TracingDisabled(t *testing.T) {
	tracker := NewTracker("test-session", false)

	step := tracker.StartStep(types.ThinkingTypeThought, "test")
	assert.Nil(t, step, "should return nil when tracing is disabled")

	tracker.EndStep(nil) // should not panic
	assert.Empty(t, tracker.GetSteps())
}

func TestTracker_GetStepByID(t *testing.T) {
	tracker := NewTracker("test-session", true)

	step := tracker.AddThought("test thought", nil)
	require.NotNil(t, step)

	found := tracker.GetStepByID(step.ID)
	require.NotNil(t, found)
	assert.Equal(t, "test thought", found.Content)
}

func TestTracker_SetModelID(t *testing.T) {
	tracker := NewTracker("test-session", true)
	tracker.SetModelID("my-model")

	step := tracker.StartStep(types.ThinkingTypeThought, "test")
	require.NotNil(t, step)
	assert.Equal(t, "my-model", step.Tokens.ModelID)
	tracker.EndStep(nil)

	total := tracker.GetTotalTokens()
	assert.Equal(t, "my-model", total.ModelID)
}

func TestTracker_Reset(t *testing.T) {
	tracker := NewTracker("test-session", true)
	tracker.AddThought("thought 1", nil)
	tracker.AddThought("thought 2", nil)
	assert.Len(t, tracker.GetSteps(), 2)

	tracker.Reset()
	assert.Empty(t, tracker.GetSteps())
}

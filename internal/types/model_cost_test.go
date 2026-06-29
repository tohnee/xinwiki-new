package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModel_CalculateCost(t *testing.T) {
	t.Run("nil_usage_returns_zero", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
		}
		cost := m.CalculateCost(nil)
		assert.Equal(t, 0.0, cost)
	})

	t.Run("basic_cost_calculation", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
		}
		usage := &TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		}
		expected := (1000.0/1_000_000)*3.0 + (500.0/1_000_000)*15.0
		cost := m.CalculateCost(usage)
		assert.InDelta(t, expected, cost, 0.000001)
	})

	t.Run("with_cached_tokens_special_price", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:        3.0,
			OutputPricePerMillion:       15.0,
			CachedInputPricePerMillion:  0.30,
		}
		usage := &TokenUsage{
			PromptTokens:     2000,
			CompletionTokens: 500,
			CachedTokens:     1500,
			TotalTokens:      2500,
		}
		nonCachedInput := 500.0
		expected := (nonCachedInput/1_000_000)*3.0 + (1500.0/1_000_000)*0.30 + (500.0/1_000_000)*15.0
		cost := m.CalculateCost(usage)
		assert.InDelta(t, expected, cost, 0.000001)
	})

	t.Run("cached_price_falls_back_to_input_price_when_zero", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:        3.0,
			OutputPricePerMillion:       15.0,
			CachedInputPricePerMillion:  0,
		}
		usage := &TokenUsage{
			PromptTokens:     2000,
			CompletionTokens: 500,
			CachedTokens:     1500,
			TotalTokens:      2500,
		}
		expected := (2000.0/1_000_000)*3.0 + (500.0/1_000_000)*15.0
		cost := m.CalculateCost(usage)
		assert.InDelta(t, expected, cost, 0.000001)
	})

	t.Run("zero_pricing_returns_zero", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:        0,
			OutputPricePerMillion:       0,
			CachedInputPricePerMillion:  0,
		}
		usage := &TokenUsage{
			PromptTokens:     1000000,
			CompletionTokens: 1000000,
			CachedTokens:     500000,
		}
		cost := m.CalculateCost(usage)
		assert.Equal(t, 0.0, cost)
	})

	t.Run("cached_tokens_exceeds_prompt_clamped", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:       3.0,
			OutputPricePerMillion:      15.0,
			CachedInputPricePerMillion: 0.30,
		}
		usage := &TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 200,
			CachedTokens:     1500,
		}
		expected := (1000.0/1_000_000)*0.30 + (200.0/1_000_000)*15.0
		cost := m.CalculateCost(usage)
		assert.InDelta(t, expected, cost, 0.000001)
	})
}

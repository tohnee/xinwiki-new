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

	// Anthropic uses ADDITIVE cache billing: Anthropic's input_tokens EXCLUDES
	// cache_read_input_tokens and cache_creation_input_tokens, and each is
	// billed ADDITIVELY at distinct published rates (~0.1× for read,
	// ~1.25× for creation). The prior arithmetic (treated cache as a subset
	// of prompt) under-billed ~90% on cached retries -- this test locks in
	// the additive semantics.
	t.Run("anthropic_additive_cache_billing_default_ratios", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
			Parameters: ModelParameters{Provider: "anthropic"},
			// CachedInputPricePerMillion intentionally zero -- must fall
			// back to the 0.1× Anthropic published ratio.
		}
		usage := &TokenUsage{
			PromptTokens:        1000, // Anthropic input_tokens (NON-cached)
			CompletionTokens:    500,
			CacheReadTokens:     800,
			CacheCreationTokens: 200,
		}
		// 1000 input billed at full rate, 800 cache_read at 0.1×=0.30,
		// 200 cache_creation at 1.25×=3.75, 500 completion at 15.
		expected := (1000.0/1_000_000)*3.0 +
			(800.0/1_000_000)*0.30 +
			(200.0/1_000_000)*3.75 +
			(500.0/1_000_000)*15.0
		cost := m.CalculateCost(usage)
		assert.InDelta(t, expected, cost, 0.000001)
	})

	// When the operator has filled CachedInputPricePerMillion (e.g. for an
	// Anthropic model with a different per-call discount), that value is
	// honored for cache_read; cache_creation is still priced at Anthropic's
	// documented 1.25× input price (no separate field yet).
	t.Run("anthropic_honors_operator_set_cache_read_price", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:       3.0,
			OutputPricePerMillion:      15.0,
			CachedInputPricePerMillion: 0.15,
			Parameters:                 ModelParameters{Provider: "anthropic"},
		}
		usage := &TokenUsage{
			PromptTokens:        1000,
			CompletionTokens:    500,
			CacheReadTokens:      800,
			CacheCreationTokens:  200,
		}
		expected := (1000.0/1_000_000)*3.0 +
			(800.0/1_000_000)*0.15 +
			(200.0/1_000_000)*(1.25*3.0) +
			(500.0/1_000_000)*15.0
		cost := m.CalculateCost(usage)
		assert.InDelta(t, expected, cost, 0.000001)
	})

	// Anthropic without any cache activity must produce the same number as
	// the OpenAI-style path (just prompt + completion) so a non-cached
	// retry doesn't regress on cost.
	t.Run("anthropic_no_cache_matches_basic_path", func(t *testing.T) {
		m := &Model{
			InputPricePerMillion:  3.0,
			OutputPricePerMillion: 15.0,
			Parameters:            ModelParameters{Provider: "anthropic"},
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
}

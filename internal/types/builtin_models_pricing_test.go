package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadBuiltinModelsConfig_PricingPassthroughAndUpsertReconcile verifies
// three properties of the YAML builtin-models loader's pricing support that
// all worked AGAINST each other in the prior "silent $0" bug:
//
//  1. New entry: YAML `input_price_per_million` / `output_price_per_million`
//     / `cached_input_price_per_million` round-trip into the corresponding
//     Model fields. Previously the BuiltinModelEntry struct did not expose
//     these fields at all, so YAML pricing silently dropped to the GORM
//     column default of 0.0 — the cost dashboard then showed blank totals
//     even when the operator had filled in YAML.
//  2. Edit: a price change in YAML actually REPLACES the column on the
//     existing row (the OnConflict DoUpdates assignment list must include
//     the price columns; without them an UPDATE silently keeps the prior
//     value). This is the regression most likely to recur if someone
//     shortens the column list later.
//  3. Zero-price default still round-trips (an operator leaving the
//     fields unset continues to see $0 — the documented "cheapest safe
//     default" for embeddings).
func TestLoadBuiltinModelsConfig_PricingPassthroughAndUpsertReconcile(t *testing.T) {
	db := setupBuiltinModelsDB(t)

	// 1. Initial price injection.
	dir := writeYAML(t, `builtin_models:
  - id: builtin-priced-llm
    name: claude-sonnet
    type: KnowledgeQA
    input_price_per_million: 3.0
    output_price_per_million: 15.0
    cached_input_price_per_million: 0.3
`)
	require.NoError(t, LoadBuiltinModelsConfig(context.Background(), db, dir))

	var m Model
	require.NoError(t, db.First(&m, "id = ?", "builtin-priced-llm").Error)
	assert.InDelta(t, 3.0, m.InputPricePerMillion, 1e-9, "input price must round-trip YAML->Model")
	assert.InDelta(t, 15.0, m.OutputPricePerMillion, 1e-9, "output price must round-trip YAML->Model")
	assert.InDelta(t, 0.3, m.CachedInputPricePerMillion, 1e-9, "cached input price must round-trip YAML->Model")

	// 2. Reconcile: operator edits the YAML. The price columns must be
	// REPLACED on the existing row so going from 3.0 -> 0.5 actually takes
	// effect, rather than the OnConflict path silently keeping 3.0.
	dir2 := writeYAML(t, `builtin_models:
  - id: builtin-priced-llm
    name: claude-sonnet
    type: KnowledgeQA
    input_price_per_million: 0.5
    output_price_per_million: 7.5
    cached_input_price_per_million: 0.1
`)
	require.NoError(t, LoadBuiltinModelsConfig(context.Background(), db, dir2))
	require.NoError(t, db.First(&m, "id = ?", "builtin-priced-llm").Error)
	assert.InDelta(t, 0.5, m.InputPricePerMillion, 1e-9, "reconcile must REPLACE input price column on the existing row")
	assert.InDelta(t, 7.5, m.OutputPricePerMillion, 1e-9, "reconcile must REPLACE output price column on the existing row")
	assert.InDelta(t, 0.1, m.CachedInputPricePerMillion, 1e-9, "reconcile must REPLACE cached input price column on the existing row")

	// 3. An entry with no price fields at all must round-trip as $0 across
	// all three columns (no accidental NaN, no NULL-in-database surprises).
	// This is the default for an operator who didn't bother with pricing —
	// cost dashboard shows blank, but the row still loads.
	dir3 := writeYAML(t, `builtin_models:
  - id: builtin-embedding-unpriced
    name: embedding-default
    type: Embedding
`)
	require.NoError(t, LoadBuiltinModelsConfig(context.Background(), db, dir3))
	var e Model
	require.NoError(t, db.First(&e, "id = ?", "builtin-embedding-unpriced").Error)
	assert.InDelta(t, 0.0, e.InputPricePerMillion, 1e-9, "missing YAML pricing must zero-default rather than NaN")
	assert.InDelta(t, 0.0, e.OutputPricePerMillion, 1e-9, "missing YAML pricing must zero-default rather than NaN")
	assert.InDelta(t, 0.0, e.CachedInputPricePerMillion, 1e-9, "missing YAML pricing must zero-default rather than NaN")
}
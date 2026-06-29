-- Migration: 000065_llm_cost_tracking (down)
-- Description: Remove LLM cost tracking tables

DO $$ BEGIN RAISE NOTICE '[Migration 000065] Rolling back LLM cost tracking...'; END $$;

DROP TABLE IF EXISTS llm_call_logs;

ALTER TABLE models
    DROP COLUMN IF EXISTS input_price_per_million,
    DROP COLUMN IF EXISTS output_price_per_million,
    DROP COLUMN IF EXISTS cached_input_price_per_million;

DO $$ BEGIN RAISE NOTICE '[Migration 000065] Rollback complete'; END $$;

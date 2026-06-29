-- Migration: 000065_llm_cost_tracking
-- Description: Add LLM cost tracking - model pricing and call logs

DO $$ BEGIN RAISE NOTICE '[Migration 000065] Adding LLM cost tracking tables...'; END $$;

-- Add pricing fields to models table
DO $$ BEGIN RAISE NOTICE '[Migration 000065] Adding pricing columns to models table...'; END $$;
ALTER TABLE models
    ADD COLUMN IF NOT EXISTS input_price_per_million DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_price_per_million DECIMAL(20, 10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cached_input_price_per_million DECIMAL(20, 10) NOT NULL DEFAULT 0;

COMMENT ON COLUMN models.input_price_per_million IS 'Price per million input tokens in USD';
COMMENT ON COLUMN models.output_price_per_million IS 'Price per million output tokens in USD';
COMMENT ON COLUMN models.cached_input_price_per_million IS 'Price per million cached input tokens in USD (0 = use input price)';

-- Create llm_call_logs table for detailed LLM call tracking
DO $$ BEGIN RAISE NOTICE '[Migration 000065] Creating table: llm_call_logs'; END $$;
CREATE TABLE IF NOT EXISTS llm_call_logs (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(36),
    session_id VARCHAR(36),
    kb_id VARCHAR(36),
    model_id VARCHAR(64) NOT NULL,
    model_type VARCHAR(50) NOT NULL DEFAULT 'chat',
    request_type VARCHAR(50) NOT NULL DEFAULT 'chat_completion',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    estimated_cost DECIMAL(20, 10) NOT NULL DEFAULT 0,
    latency_ms INTEGER,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    error_message TEXT,
    trace_id VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient aggregation queries
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_tenant_id ON llm_call_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_user_id ON llm_call_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_model_id ON llm_call_logs(model_id);
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_session_id ON llm_call_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_created_at ON llm_call_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_tenant_created ON llm_call_logs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_call_logs_tenant_model_created ON llm_call_logs(tenant_id, model_id, created_at);

DO $$ BEGIN RAISE NOTICE '[Migration 000065] LLM cost tracking tables created successfully'; END $$;

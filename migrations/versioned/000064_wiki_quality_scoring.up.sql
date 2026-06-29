-- Migration: 000064_wiki_quality_scoring
-- Description: Add confidence score, quality score, freshness tracking fields to wiki_pages, and wiki_supersessions table

DO $$ BEGIN RAISE NOTICE '[Migration 000064] Adding wiki quality scoring fields and supersessions table'; END $$;

-- Create wiki_supersessions table for B milestone
CREATE TABLE IF NOT EXISTS wiki_supersessions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL DEFAULT 0,
    knowledge_base_id VARCHAR(36) NOT NULL,
    old_page_id VARCHAR(36) NOT NULL,
    old_page_slug VARCHAR(255) NOT NULL,
    new_page_id VARCHAR(36) NOT NULL,
    new_page_slug VARCHAR(255) NOT NULL,
    reason TEXT,
    created_by VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_wiki_supersessions_tenant_kb
    ON wiki_supersessions (tenant_id, knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_wiki_supersessions_old_page
    ON wiki_supersessions (old_page_id);

CREATE INDEX IF NOT EXISTS idx_wiki_supersessions_new_page
    ON wiki_supersessions (new_page_id);

CREATE INDEX IF NOT EXISTS idx_wiki_supersessions_old_slug
    ON wiki_supersessions (knowledge_base_id, old_page_slug);

CREATE INDEX IF NOT EXISTS idx_wiki_supersessions_new_slug
    ON wiki_supersessions (knowledge_base_id, new_page_slug);

CREATE INDEX IF NOT EXISTS idx_wiki_supersessions_deleted_at
    ON wiki_supersessions (deleted_at);

-- Add quality scoring fields to wiki_pages
ALTER TABLE wiki_pages
    -- Criticality and access tracking
    ADD COLUMN IF NOT EXISTS criticality_level VARCHAR(8) NOT NULL DEFAULT 'P3',
    ADD COLUMN IF NOT EXISTS last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS view_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS positive_feedback BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS negative_feedback BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS expert_validated BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS expert_validated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS contradiction_count INTEGER NOT NULL DEFAULT 0,

    -- Confidence score components (0.0 - 1.0)
    ADD COLUMN IF NOT EXISTS confidence_score DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS source_authority DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS evidence_support DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS recency_score DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS consistency_score DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS expert_validation DECIMAL(5,4) NOT NULL DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS usage_feedback DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS contradiction_penalty DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS staleness_penalty DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS final_score DECIMAL(5,4) NOT NULL DEFAULT 0.5,

    -- Quality score components (0.0 - 1.0)
    ADD COLUMN IF NOT EXISTS quality_score DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS content_completeness DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS source_reliability DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS timeliness_score DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS readability_score DECIMAL(5,4) NOT NULL DEFAULT 0.5,
    ADD COLUMN IF NOT EXISTS citation_sufficiency DECIMAL(5,4) NOT NULL DEFAULT 0.5,

    -- Freshness state and retrieval boost
    ADD COLUMN IF NOT EXISTS freshness_state VARCHAR(16) NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS retrieval_boost DECIMAL(5,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS score_last_calculated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Create indexes for scoring fields
CREATE INDEX IF NOT EXISTS idx_wiki_pages_criticality_level
    ON wiki_pages (tenant_id, knowledge_base_id, criticality_level);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_final_score
    ON wiki_pages (tenant_id, knowledge_base_id, final_score DESC);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_freshness_state
    ON wiki_pages (tenant_id, knowledge_base_id, freshness_state);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_retrieval_boost
    ON wiki_pages (tenant_id, knowledge_base_id, retrieval_boost DESC);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_last_accessed_at
    ON wiki_pages (tenant_id, knowledge_base_id, last_accessed_at);

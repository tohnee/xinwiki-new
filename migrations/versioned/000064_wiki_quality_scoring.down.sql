-- Migration: 000064_wiki_quality_scoring (Down)
-- Description: Remove wiki quality scoring fields from wiki_pages

DO $$ BEGIN RAISE NOTICE '[Migration 000064] Rolling back wiki quality scoring fields'; END $$;

-- Drop indexes first
DROP INDEX IF EXISTS idx_wiki_pages_criticality_level;
DROP INDEX IF EXISTS idx_wiki_pages_final_score;
DROP INDEX IF EXISTS idx_wiki_pages_freshness_state;
DROP INDEX IF EXISTS idx_wiki_pages_retrieval_boost;
DROP INDEX IF EXISTS idx_wiki_pages_last_accessed_at;

-- Drop columns
ALTER TABLE wiki_pages
    DROP COLUMN IF EXISTS criticality_level,
    DROP COLUMN IF EXISTS last_accessed_at,
    DROP COLUMN IF EXISTS view_count,
    DROP COLUMN IF EXISTS positive_feedback,
    DROP COLUMN IF EXISTS negative_feedback,
    DROP COLUMN IF EXISTS expert_validated,
    DROP COLUMN IF EXISTS expert_validated_at,
    DROP COLUMN IF EXISTS contradiction_count,
    DROP COLUMN IF EXISTS confidence_score,
    DROP COLUMN IF EXISTS source_authority,
    DROP COLUMN IF EXISTS evidence_support,
    DROP COLUMN IF EXISTS recency_score,
    DROP COLUMN IF EXISTS consistency_score,
    DROP COLUMN IF EXISTS expert_validation,
    DROP COLUMN IF EXISTS usage_feedback,
    DROP COLUMN IF EXISTS contradiction_penalty,
    DROP COLUMN IF EXISTS staleness_penalty,
    DROP COLUMN IF EXISTS final_score,
    DROP COLUMN IF EXISTS quality_score,
    DROP COLUMN IF EXISTS content_completeness,
    DROP COLUMN IF EXISTS source_reliability,
    DROP COLUMN IF EXISTS timeliness_score,
    DROP COLUMN IF EXISTS readability_score,
    DROP COLUMN IF EXISTS citation_sufficiency,
    DROP COLUMN IF EXISTS freshness_state,
    DROP COLUMN IF EXISTS retrieval_boost,
    DROP COLUMN IF EXISTS score_last_calculated_at;

-- Migration: 000068_prompt_templates (down)
-- Description: Drop prompt_templates table (rolling back DB-backed prompt template versioning).
-- WARNING: dropping the table loses user-edited prompt templates; only run when
-- you have copied any production overrides back to your config/prompt_templates
-- file or otherwise preserved them.

DO $$ BEGIN RAISE NOTICE '[Migration 000068] Rolling back prompt_templates...'; END $$;

DROP TABLE IF EXISTS prompt_templates;

DO $$ BEGIN RAISE NOTICE '[Migration 000068] Rollback complete'; END $$;
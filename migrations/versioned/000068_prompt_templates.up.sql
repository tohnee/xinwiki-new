-- Migration: 000068_prompt_templates
-- Description: Create prompt_templates table for DB-backed prompt template versioning.
-- Enables multi-replica HA: previously prompt templates lived in an in-memory
-- process singleton (internal/application/service/prompt_template.go) so any
-- edit via the UI/API was lost on restart and not visible to other replicas.

DO $$ BEGIN RAISE NOTICE '[Migration 000068] Creating table: prompt_templates'; END $$;

CREATE TABLE IF NOT EXISTS prompt_templates (
    id           VARCHAR(36) PRIMARY KEY,
    template_key VARCHAR(64) NOT NULL,
    tenant_id    BIGINT     NOT NULL DEFAULT 0,
    version      VARCHAR(32) NOT NULL,
    content      TEXT       NOT NULL,
    description  VARCHAR(256),
    is_active    BOOLEAN    NOT NULL DEFAULT TRUE,
    created_by   VARCHAR(36),
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- One active version per (tenant, key) is enforced by the code path that
-- swaps is_active; a partial unique index would additionally give a DB
-- guard, but the service-layer logic handles the mutual-exclusion today
-- and adding the partial index now could fail on pre-existing inconsistent
-- datasets. The composite UNIQUE on (template_key, tenant_id, version)
-- below (mirrors the GORM unique-index tag) gives the storage-level
-- uniqueness for versions.
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_key_version
    ON prompt_templates (template_key, tenant_id, version);

CREATE INDEX IF NOT EXISTS idx_prompt_templates_tenant_id ON prompt_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_template_key ON prompt_templates(template_key);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_active
    ON prompt_templates(template_key, tenant_id, is_active)
    WHERE is_active = TRUE;

DO $$ BEGIN RAISE NOTICE '[Migration 000068] prompt_templates table created successfully'; END $$;
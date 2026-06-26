-- Migration: 000063_derived_acl_metadata
-- Description: Add security classification and optional ACL metadata to chunks and wiki pages.

DO $$ BEGIN RAISE NOTICE '[Migration 000063] Adding derived ACL metadata to chunks and wiki_pages'; END $$;

ALTER TABLE chunks
    ADD COLUMN IF NOT EXISTS security_level VARCHAR(16) NOT NULL DEFAULT 'L1',
    ADD COLUMN IF NOT EXISTS allowed_user_ids JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS allowed_group_ids JSONB NOT NULL DEFAULT '[]'::JSONB;

ALTER TABLE wiki_pages
    ADD COLUMN IF NOT EXISTS security_level VARCHAR(16) NOT NULL DEFAULT 'L1',
    ADD COLUMN IF NOT EXISTS allowed_user_ids JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS allowed_group_ids JSONB NOT NULL DEFAULT '[]'::JSONB;

CREATE INDEX IF NOT EXISTS idx_chunks_security_level
    ON chunks (tenant_id, knowledge_base_id, security_level);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_security_level
    ON wiki_pages (tenant_id, knowledge_base_id, security_level);

CREATE INDEX IF NOT EXISTS idx_chunks_allowed_user_ids
    ON chunks USING GIN (allowed_user_ids jsonb_path_ops);

CREATE INDEX IF NOT EXISTS idx_chunks_allowed_group_ids
    ON chunks USING GIN (allowed_group_ids jsonb_path_ops);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_allowed_user_ids
    ON wiki_pages USING GIN (allowed_user_ids jsonb_path_ops);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_allowed_group_ids
    ON wiki_pages USING GIN (allowed_group_ids jsonb_path_ops);

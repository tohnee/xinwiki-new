-- Rollback: 000063_derived_acl_metadata

DO $$ BEGIN RAISE NOTICE '[Migration 000063] Removing derived ACL metadata from chunks and wiki_pages'; END $$;

DROP INDEX IF EXISTS idx_wiki_pages_allowed_group_ids;
DROP INDEX IF EXISTS idx_wiki_pages_allowed_user_ids;
DROP INDEX IF EXISTS idx_chunks_allowed_group_ids;
DROP INDEX IF EXISTS idx_chunks_allowed_user_ids;
DROP INDEX IF EXISTS idx_wiki_pages_security_level;
DROP INDEX IF EXISTS idx_chunks_security_level;

ALTER TABLE wiki_pages
    DROP COLUMN IF EXISTS allowed_group_ids,
    DROP COLUMN IF EXISTS allowed_user_ids,
    DROP COLUMN IF EXISTS security_level;

ALTER TABLE chunks
    DROP COLUMN IF EXISTS allowed_group_ids,
    DROP COLUMN IF EXISTS allowed_user_ids,
    DROP COLUMN IF EXISTS security_level;

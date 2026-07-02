-- Migration: 000069_user_notes
-- Per-(user, tenant) notes that pin a saved excerpt from chat citations
-- or a free-form note the user writes. This is the backend for the
-- NotebookLM-style "Notes" surface in the Workspace.
--
-- Design mirrors user_resource_favorites (000047): tenant-scoped, no FK
-- to chat sessions / knowledge chunks so a note survives session expiry
-- or chunk re-ingestion. The source_* columns denormalise the citation
-- metadata at save time so the note stays readable even if the upstream
-- reference is later deleted.
DO $$ BEGIN RAISE NOTICE '[Migration 000069] Creating table: user_notes'; END $$;

CREATE TABLE IF NOT EXISTS user_notes (
    id             VARCHAR(36) NOT NULL,
    user_id        VARCHAR(36) NOT NULL,
    tenant_id      BIGINT      NOT NULL,
    session_id     VARCHAR(36),                 -- optional, for "open in chat"溯源
    title          VARCHAR(255) NOT NULL,
    content        TEXT         NOT NULL DEFAULT '',
    source_excerpt TEXT,                       -- the cited snippet the user saved
    source_ref_id  VARCHAR(64),                -- chunk id / wiki page id
    source_title   VARCHAR(512),               -- denormalised citation title
    source_url     TEXT,                       -- denormalised citation url
    created_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
);

-- Primary read path: "list this user's notes in this tenant, newest first".
CREATE INDEX IF NOT EXISTS idx_user_notes_user_tenant_created_at
    ON user_notes (user_id, tenant_id, created_at DESC);

-- Tenant cleanup path.
CREATE INDEX IF NOT EXISTS idx_user_notes_tenant_id
    ON user_notes (tenant_id);

-- Optional session-scoped listing ("notes saved from this chat").
CREATE INDEX IF NOT EXISTS idx_user_notes_session_id
    ON user_notes (user_id, tenant_id, session_id);

DO $$ BEGIN RAISE NOTICE '[Migration 000069] user_notes table ready'; END $$;

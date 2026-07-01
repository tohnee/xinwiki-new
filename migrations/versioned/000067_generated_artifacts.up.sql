-- Generated artifacts (review 4.2 risk 2).
--
-- The right-panel generation surface (PPT / PDF / report / chart / diagram /
-- markdown) needs a real data model with tenant / user / session / source
-- provenance and an ACL, otherwise generated content becomes a cross-tenant /
-- cross-user leak channel. Every row is scoped to a tenant, a creator, the
-- session that produced it, and the source KB / document / page refs it was
-- derived from; sharing_policy + allowed_user_ids enforce who may read it
-- (evaluated by types.CanAccessArtifact).
--
-- storage_uri is an opaque object-store key / path resolved by the file
-- service at download time; the artifact file itself is governed by the same
-- tenant/owner ACL as the row.
CREATE TABLE IF NOT EXISTS generated_artifacts (
    id                  TEXT PRIMARY KEY,
    tenant_id           BIGINT      NOT NULL,
    user_id             TEXT        NOT NULL,
    session_id          TEXT        NOT NULL DEFAULT '',
    type                TEXT        NOT NULL,
    status              TEXT        NOT NULL DEFAULT 'pending',
    title               TEXT        NOT NULL DEFAULT '',
    source_kb_id        TEXT        NOT NULL DEFAULT '',
    source_knowledge_id TEXT        NOT NULL DEFAULT '',
    source_wiki_page_id TEXT        NOT NULL DEFAULT '',
    source_refs         JSONB       NOT NULL DEFAULT '[]'::jsonb,
    storage_uri         TEXT        NOT NULL DEFAULT '',
    storage_type        TEXT        NOT NULL DEFAULT '',
    mime_type           TEXT        NOT NULL DEFAULT '',
    size_bytes          BIGINT      NOT NULL DEFAULT 0,
    sharing_policy      TEXT        NOT NULL DEFAULT 'private',
    allowed_user_ids    JSONB       NOT NULL DEFAULT '[]'::jsonb,
    metadata            JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ
);

-- Tenant-scoped list (the hot path: "show this tenant's artifacts").
CREATE INDEX IF NOT EXISTS idx_generated_artifacts_tenant_id
    ON generated_artifacts (tenant_id, created_at DESC) WHERE deleted_at IS NULL;
-- Creator's own artifacts ("my artifacts" view).
CREATE INDEX IF NOT EXISTS idx_generated_artifacts_user_id
    ON generated_artifacts (tenant_id, user_id, created_at DESC) WHERE deleted_at IS NULL;
-- Session-scoped artifacts (the chat/agent panel: "what did this session produce").
CREATE INDEX IF NOT EXISTS idx_generated_artifacts_session_id
    ON generated_artifacts (tenant_id, session_id) WHERE deleted_at IS NULL AND session_id <> '';
-- Status filter (e.g. pending generation jobs).
CREATE INDEX IF NOT EXISTS idx_generated_artifacts_status
    ON generated_artifacts (tenant_id, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_generated_artifacts_deleted_at
    ON generated_artifacts (deleted_at);

-- Scoped, revocable API keys (review 4.5).
--
-- Replaces the implicit "tenant-wide master key" model (the single
-- tenants.api_key column that authenticated as a synthetic Owner) with a
-- dedicated table where each key carries an explicit Scopes list and optional
-- user binding, so integrations get least-privilege access and individual
-- keys can be rotated/revoked independently.
--
-- key_hash stores a SHA-256 digest (never plaintext); the secret is returned
-- to the caller only once at creation. prefix is a short display fragment.
CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT PRIMARY KEY,
    tenant_id    BIGINT      NOT NULL,
    user_id      TEXT,
    name         TEXT        NOT NULL DEFAULT '',
    key_hash     TEXT        NOT NULL,
    prefix       TEXT        NOT NULL DEFAULT '',
    scopes       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    status       TEXT        NOT NULL DEFAULT 'active',
    expires_at   TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id     ON api_keys (tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash      ON api_keys (key_hash)  WHERE deleted_at IS NULL AND status = 'active';
CREATE INDEX IF NOT EXISTS idx_api_keys_status        ON api_keys (status)    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at    ON api_keys (deleted_at);

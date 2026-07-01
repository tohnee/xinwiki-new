-- Revert scoped API keys (review 4.5).
DROP INDEX IF EXISTS idx_api_keys_deleted_at;
DROP INDEX IF EXISTS idx_api_keys_status;
DROP INDEX IF EXISTS idx_api_keys_key_hash;
DROP INDEX IF EXISTS idx_api_keys_tenant_id;
DROP TABLE IF EXISTS api_keys;

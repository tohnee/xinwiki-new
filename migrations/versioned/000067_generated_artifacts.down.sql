-- Roll back the generated_artifacts table (review 4.2).
DROP INDEX IF EXISTS idx_generated_artifacts_deleted_at;
DROP INDEX IF EXISTS idx_generated_artifacts_status;
DROP INDEX IF EXISTS idx_generated_artifacts_session_id;
DROP INDEX IF EXISTS idx_generated_artifacts_user_id;
DROP INDEX IF EXISTS idx_generated_artifacts_tenant_id;
DROP TABLE IF EXISTS generated_artifacts;

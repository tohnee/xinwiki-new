-- Reverse of 000069_user_notes.
DO $$ BEGIN RAISE NOTICE '[Migration 000069] Dropping table: user_notes'; END $$;

DROP INDEX IF EXISTS idx_user_notes_session_id;
DROP INDEX IF EXISTS idx_user_notes_tenant_id;
DROP INDEX IF EXISTS idx_user_notes_user_tenant_created_at;
DROP TABLE IF EXISTS user_notes;

DO $$ BEGIN RAISE NOTICE '[Migration 000069] user_notes table dropped'; END $$;

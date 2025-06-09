-- Migration: Drop chat_sessions table
-- Rollback for chat session management migration

-- Drop trigger
DROP TRIGGER IF EXISTS update_chat_sessions_updated_at ON chat_sessions;

-- Drop indexes
DROP INDEX IF EXISTS idx_chat_sessions_tags;
DROP INDEX IF EXISTS idx_chat_sessions_parent;
DROP INDEX IF EXISTS idx_chat_sessions_status;
DROP INDEX IF EXISTS idx_chat_sessions_last_interacted;
DROP INDEX IF EXISTS idx_chat_sessions_user_updated;
DROP INDEX IF EXISTS idx_chat_sessions_updated_at;
DROP INDEX IF EXISTS idx_chat_sessions_user_id;

-- Drop table
DROP TABLE IF EXISTS chat_sessions; 
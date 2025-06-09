-- Migration: Drop messages table
-- Rollback for message management migration

-- Drop indexes
DROP INDEX IF EXISTS idx_messages_chat_created;
DROP INDEX IF EXISTS idx_messages_created_at;
DROP INDEX IF EXISTS idx_messages_parent;
DROP INDEX IF EXISTS idx_messages_chat_session;

-- Drop table
DROP TABLE IF EXISTS messages; 
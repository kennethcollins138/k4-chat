-- Migration: Drop shared_chats table
-- Rollback for shared chats migration

-- Drop indexes
DROP INDEX IF EXISTS idx_shared_chats_public;
DROP INDEX IF EXISTS idx_shared_chats_token;

-- Drop table
DROP TABLE IF EXISTS shared_chats; 
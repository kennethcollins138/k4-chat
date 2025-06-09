-- Migration: Drop user_sessions table
-- Rollback for user sessions migration

-- Drop indexes
DROP INDEX IF EXISTS idx_user_sessions_user;
DROP INDEX IF EXISTS idx_user_sessions_token;

-- Drop table
DROP TABLE IF EXISTS user_sessions; 
-- Migration: Rollback tier column from users table
-- Rollback for user tier support

-- Drop index
DROP INDEX IF EXISTS idx_users_tier;

-- Drop tier column
ALTER TABLE users DROP COLUMN IF EXISTS tier; 
-- Migration: Drop users table
-- Rollback for initial user management migration

-- Drop trigger
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- Drop indexes
DROP INDEX IF EXISTS idx_users_active;
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;

-- Drop table
DROP TABLE IF EXISTS users;

-- Note: We don't drop the update_updated_at_column function here 
-- as it might be used by other tables 
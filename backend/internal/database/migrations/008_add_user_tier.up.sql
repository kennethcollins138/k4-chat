-- Migration: Add tier column to users table
-- Created: Add user tier support for auth system

-- Add tier column to users table
ALTER TABLE users ADD COLUMN tier VARCHAR(20) DEFAULT 'free' NOT NULL;

-- Add index for tier queries (performance optimization)
CREATE INDEX idx_users_tier ON users(tier);

-- Update existing users to have 'free' tier (if any exist)
UPDATE users SET tier = 'free' WHERE tier IS NULL; 
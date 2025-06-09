-- Migration: Drop message_attachments table
-- Rollback for message attachments migration

-- Drop indexes
DROP INDEX IF EXISTS idx_message_attachments_message;

-- Drop table
DROP TABLE IF EXISTS message_attachments; 
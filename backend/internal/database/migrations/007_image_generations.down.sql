-- Migration: Drop image_generations table
-- Rollback for image generations migration

-- Drop indexes
DROP INDEX IF EXISTS idx_image_generations_message;

-- Drop table
DROP TABLE IF EXISTS image_generations; 
-- Migration: Create image_generations table
-- Created: AI image generation support

-- Image generations table
CREATE TABLE image_generations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    model_name VARCHAR(100) NOT NULL, -- 'dall-e-3', 'midjourney', etc.
    image_url TEXT NOT NULL,
    thumbnail_url TEXT,
    width INTEGER,
    height INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for image_generations table
CREATE INDEX idx_image_generations_message ON image_generations(message_id); 
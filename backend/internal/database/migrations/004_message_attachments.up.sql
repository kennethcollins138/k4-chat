-- Migration: Create message_attachments table
-- Created: File attachment support for messages

-- Message attachments table
CREATE TABLE message_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    storage_path TEXT NOT NULL, -- S3 key or local path
    upload_url TEXT, -- Pre-signed URL for direct upload
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for message_attachments table
CREATE INDEX idx_message_attachments_message ON message_attachments(message_id); 
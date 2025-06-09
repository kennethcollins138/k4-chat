-- Migration: Create shared_chats table
-- Created: Chat sharing functionality

-- Shared chats table
CREATE TABLE shared_chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    share_token VARCHAR(64) UNIQUE NOT NULL,
    title VARCHAR(200),
    description TEXT,
    is_public BOOLEAN DEFAULT false,
    password_hash VARCHAR(255), -- Optional password protection
    expires_at TIMESTAMP WITH TIME ZONE,
    view_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for shared_chats table
CREATE INDEX idx_shared_chats_token ON shared_chats(share_token);
CREATE INDEX idx_shared_chats_public ON shared_chats(is_public, created_at DESC); 
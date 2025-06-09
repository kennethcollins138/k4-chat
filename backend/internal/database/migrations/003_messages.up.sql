-- Migration: Create messages table
-- Created: Message management with branching support

-- Messages table - supports branching
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_session_id UUID NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    parent_message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content TEXT NOT NULL,
    token_count INTEGER,
    model_name VARCHAR(100), -- For assistant messages
    finish_reason VARCHAR(50), -- 'stop', 'length', 'tool_calls', etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    branch_index INTEGER DEFAULT 0, -- For handling multiple responses
    is_selected BOOLEAN DEFAULT true -- Which branch is currently selected
);

-- Indexes for messages table
CREATE INDEX idx_messages_chat_session ON messages(chat_session_id);
CREATE INDEX idx_messages_parent ON messages(parent_message_id);
CREATE INDEX idx_messages_created_at ON messages(created_at);
CREATE INDEX idx_messages_chat_created ON messages(chat_session_id, created_at); 
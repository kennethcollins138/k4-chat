-- Migration: Create chat_sessions table
-- Created: Chat session management

-- Chat sessions table
CREATE TABLE chat_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL DEFAULT 'New Chat',
    model_name VARCHAR(100) NOT NULL, -- e.g., 'gpt-4', 'claude-3'
    system_prompt TEXT,
    temperature DECIMAL(3,2) DEFAULT 0.7,
    max_tokens INTEGER DEFAULT 4000,
    model_config JSONB, -- Advanced config (e.g. top_p, stop sequences, etc.)
    parent_session_id UUID REFERENCES chat_sessions(id) ON DELETE SET NULL, -- For forked/branched chats
    branch_label VARCHAR(100), -- Label for a branch (e.g. "Plan B")
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'archived', 'deleted', 'shared')), -- Lifecycle status
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_interacted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(), -- Last message/interaction
    archived_at TIMESTAMP WITH TIME ZONE,
    is_pinned BOOLEAN DEFAULT false,
    is_favorite BOOLEAN DEFAULT false, -- Marked favorite
    tags TEXT[], -- User-defined tags for filtering
    extensions JSONB -- Dynamic or plugin-based metadata
);

-- Indexes for chat_sessions table
CREATE INDEX idx_chat_sessions_user_id ON chat_sessions(user_id);
CREATE INDEX idx_chat_sessions_updated_at ON chat_sessions(updated_at DESC);
CREATE INDEX idx_chat_sessions_user_updated ON chat_sessions(user_id, updated_at DESC);
CREATE INDEX idx_chat_sessions_last_interacted ON chat_sessions(last_interacted_at DESC);
CREATE INDEX idx_chat_sessions_status ON chat_sessions(status);
CREATE INDEX idx_chat_sessions_parent ON chat_sessions(parent_session_id);
CREATE INDEX idx_chat_sessions_tags ON chat_sessions USING GIN(tags);

-- Trigger for chat_sessions table
CREATE TRIGGER update_chat_sessions_updated_at BEFORE UPDATE ON chat_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column(); 
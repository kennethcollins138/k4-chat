package models

import (
	"time"

	"github.com/google/uuid"
)

/*
Message represents a single entry in a chat session, whether from the user, assistant, or system.

This model is designed to support **branchable conversations**, **rich content**, and **auditing** of
LLM behavior. While simple in structure, it accommodates advanced features like:

- Message Trees (threaded/branching dialogues)
- Response tracking (e.g. token usage, finish reasons)
- File attachments per message
- Model-specific metadata

Key Design Notes:

- ParentMessageID:
    Enables *conversation forking* or threaded replies, important for interactive LLM use cases
    (e.g., exploring alternate responses, backtracking logic).

- Role:
    Tracks the sender identity: 'user', 'assistant', or 'system'. Can be extended to support
    future roles like 'tool', 'plugin', or 'moderator'.

- ModelName / FinishReason:
    Critical for LLM auditing, evaluation, and debugging. Especially helpful when using multiple providers.

- TokenCount:
    Used for cost tracking, throttling, and performance analysis.

- BranchIndex / IsSelected:
    When a message has siblings (i.e. multiple children from the same parent), these fields help
    determine which branch is "active" in the UI or logic. Supports GPT-like streaming with multi-choice outputs.

- Attachments / Children:
    These relationships are not directly stored in the DB (denoted with `db:"-"`), but are
    loaded and resolved at runtime to support rendering or export.

Planned extensions:

- `Status` (e.g., "in_progress", "streaming", "completed", "failed")
- `Reactions` or Feedback metadata
- `ToolCalls` for plugin or tool execution
*/

type Message struct {
	ID              uuid.UUID  `json:"id" db:"id"`                                         // Unique message ID
	ChatSessionID   uuid.UUID  `json:"chat_session_id" db:"chat_session_id"`               // Parent chat session
	ParentMessageID *uuid.UUID `json:"parent_message_id,omitempty" db:"parent_message_id"` // Optional parent message (for threading)

	Role         string  `json:"role" db:"role"`                             // 'user', 'assistant', 'system'
	Content      string  `json:"content" db:"content"`                       // Message text content
	TokenCount   *int    `json:"token_count,omitempty" db:"token_count"`     // Optional: LLM token usage
	ModelName    *string `json:"model_name,omitempty" db:"model_name"`       // Optional: LLM model used
	FinishReason *string `json:"finish_reason,omitempty" db:"finish_reason"` // Optional: why the LLM stopped generating

	CreatedAt   time.Time `json:"created_at" db:"created_at"`     // Time of message creation
	BranchIndex int       `json:"branch_index" db:"branch_index"` // Order of child within its parent
	IsSelected  bool      `json:"is_selected" db:"is_selected"`   // Whether this message is the selected branch (in forks)

	// Relationships (runtime only, not stored directly in DB)
	Attachments []MessageAttachment `json:"attachments,omitempty" db:"-"` // Files attached to the message
	Children    []Message           `json:"children,omitempty" db:"-"`    // Child messages (replies or forks)
}

/*
MessageAttachment represents a file attached to a specific message.

Supports uploading and storing user-provided or generated content. This design assumes the
files are stored in a persistent store (e.g. S3, local disk), and optionally provide an
upload URL if the client needs to send the file directly (pre-signed URL pattern).
*/

type MessageAttachment struct {
	ID          uuid.UUID `json:"id" db:"id"`                           // Unique ID for the attachment
	MessageID   uuid.UUID `json:"message_id" db:"message_id"`           // Linked message
	FileName    string    `json:"file_name" db:"file_name"`             // Original file name
	FileSize    int64     `json:"file_size" db:"file_size"`             // Size in bytes
	MimeType    string    `json:"mime_type" db:"mime_type"`             // MIME type (e.g. image/png)
	StoragePath string    `json:"storage_path" db:"storage_path"`       // Internal path or key (e.g. S3 bucket key)
	UploadURL   *string   `json:"upload_url,omitempty" db:"upload_url"` // Optional: pre-signed client upload URL
	CreatedAt   time.Time `json:"created_at" db:"created_at"`           // Time of upload
}

/*
CreateMessageRequest represents the request payload to create a new message in a chat session.

Used in API endpoints and internal validation. This structure enforces required fields and
acceptable values for the role field to avoid misuse.
*/

type CreateMessageRequest struct {
	ParentMessageID *uuid.UUID `json:"parent_message_id,omitempty"`                          // Optional: ID of parent message
	Role            string     `json:"role" validate:"required,oneof=user assistant system"` // Sender identity
	Content         string     `json:"content" validate:"required"`                          // Message body
}

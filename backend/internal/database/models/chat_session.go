package models

import (
	"time"

	"github.com/google/uuid"
)

/*
ChatSession represents a single chat conversation session between a user and an LLM.

This model is designed with **future scalability and extensibility** in mind. While some fields
may not be immediately active in the current implementation, they are included to support planned
features and ensure smooth evolution of the system without requiring disruptive migrations.

Key Design Notes:

- ParentSessionID, BranchLabel:
    Supports *chat branching*, allowing users to fork alternative conversation paths
    from any message/session. Enables graph-style navigation of conversation history.

- ModelConfig:
    Generic key-value map for fine-tuning and provider-specific settings beyond temperature
    and max tokens. Future-proofed to support custom sampling strategies (e.g. top_p, stop sequences).

- Status:
    Adds flexibility to manage lifecycle states such as "active", "archived", "deleted", or "shared".
    Preferable to relying solely on ArchivedAt for state-based logic.

- LastInteractedAt:
    Improves UX by enabling better session sorting (e.g. most recently used), independent of creation time.

- Tags, IsFavorite:
    Personalization features for user-side filtering, searching, or flagging important conversations.

- Extensions:
    Reserved for experimental or dynamic metadata. This can include:
        - Tool usage context (e.g. web search enabled)
        - Plugin toggles (e.g. code interpreter)
        - Temporary session-level overrides

Although these fields are not fully utilized at project inception, they reflect a forward-looking
design and reduce the need for disruptive schema changes later.
*/

type ChatSession struct {
	ID               uuid.UUID              `json:"id" db:"id"`                                         // Unique session ID
	UserID           uuid.UUID              `json:"user_id" db:"user_id"`                               // Owning user
	Title            string                 `json:"title" db:"title"`                                   // User-provided title
	ModelName        string                 `json:"model_name" db:"model_name"`                         // LLM provider/model used
	SystemPrompt     *string                `json:"system_prompt,omitempty" db:"system_prompt"`         // Optional override prompt
	Temperature      float64                `json:"temperature" db:"temperature"`                       // Sampling temperature
	MaxTokens        int                    `json:"max_tokens" db:"max_tokens"`                         // Token limit per message
	ModelConfig      map[string]interface{} `json:"model_config,omitempty" db:"model_config"`           // Advanced config (e.g. top_p, stop, etc.) (TODO: WE WILL SEE IF I HAVE TIME FOR THIS)
	ParentSessionID  *uuid.UUID             `json:"parent_session_id,omitempty" db:"parent_session_id"` // For forked/branched chats
	BranchLabel      *string                `json:"branch_label,omitempty" db:"branch_label"`           // Label for a branch (e.g. "Plan B")
	Status           string                 `json:"status" db:"status"`                                 // Lifecycle status: active/archived/deleted
	CreatedAt        time.Time              `json:"created_at" db:"created_at"`                         // Timestamp of creation
	UpdatedAt        time.Time              `json:"updated_at" db:"updated_at"`                         // Last update timestamp
	LastInteractedAt time.Time              `json:"last_interacted_at" db:"last_interacted_at"`         // Last message/interaction
	ArchivedAt       *time.Time             `json:"archived_at,omitempty" db:"archived_at"`             // Soft-delete/archive support
	IsPinned         bool                   `json:"is_pinned" db:"is_pinned"`                           // Pinned by user for quick access
	IsFavorite       bool                   `json:"is_favorite" db:"is_favorite"`                       // Marked favorite
	Tags             []string               `json:"tags,omitempty" db:"tags"`                           // User-defined tags for filtering
	Extensions       map[string]interface{} `json:"extensions,omitempty" db:"extensions"`               // Dynamic or plugin-based metadata
}

// CreateChatSessionRequest represents the request to create a new chat session
type CreateChatSessionRequest struct {
	Title        string   `json:"title" validate:"required,max=200"`                         // Session title
	ModelName    string   `json:"model_name" validate:"required"`                            // Model to use
	SystemPrompt *string  `json:"system_prompt,omitempty"`                                   // Optional custom system prompt
	Temperature  *float64 `json:"temperature,omitempty" validate:"omitempty,min=0,max=2"`    // Sampling temperature
	MaxTokens    *int     `json:"max_tokens,omitempty" validate:"omitempty,min=1,max=32000"` // Max response size
}

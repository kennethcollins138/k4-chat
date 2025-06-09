package models

import (
	"time"

	"github.com/google/uuid"
)

/*
SharedChat represents a publicly accessible or restricted shared view of a chat session.

This model enables use cases such as:
  - Sharing conversations with teammates or externally
  - Embedding chats in blogs or documentation
  - Public archives of AI-assisted sessions

Key Design Elements:

- ShareToken:
    A unique, unguessable string used as the access key in shared URLs
    (e.g., `/share/{share_token}`). Decouples exposure from internal UUIDs.

- PasswordHash:
    Optional hashed password to protect shared chats. Hidden from JSON serialization
    to ensure no sensitive data leaks. Use bcrypt/argon2 for secure storage.

- IsPublic:
    Boolean toggle to allow discovery (e.g., public archive index). This is separate
    from password protection, allowing fine-grained access control.

- ExpiresAt:
    Optional field to auto-expire access after a certain time (e.g., 7 days).
    Useful for temporary collaborations or one-time share links.

- ViewCount:
    Analytics field for tracking popularity or auditing access.

- ChatSession (Relationship):
    Optional preloaded full session object. Not persisted in the DB,
    typically filled via JOIN or hydration logic in the service layer.

Future Scalability Ideas:

- Add `Tags`, `OwnerName`, or `ThumbnailURL` for display in shared indexes.
- Add `AccessLogs []AccessLog` to track IP/time/user agents for auditing.
- Extend with `Permissions` to allow shared edits, annotations, or comments.
*/

type SharedChat struct {
	ID            uuid.UUID  `json:"id" db:"id"`                             // Internal UUID
	ChatSessionID uuid.UUID  `json:"chat_session_id" db:"chat_session_id"`   // Foreign key to original chat
	ShareToken    string     `json:"share_token" db:"share_token"`           // Public-facing access token
	Title         *string    `json:"title,omitempty" db:"title"`             // Optional display title
	Description   *string    `json:"description,omitempty" db:"description"` // Optional summary or notes
	IsPublic      bool       `json:"is_public" db:"is_public"`               // Discoverable in public listings
	PasswordHash  *string    `json:"-" db:"password_hash"`                   // Protected access (never exposed)
	ExpiresAt     *time.Time `json:"expires_at,omitempty" db:"expires_at"`   // Optional expiry date
	ViewCount     int        `json:"view_count" db:"view_count"`             // Analytics field
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`             // Creation timestamp

	// Relationship (hydrated separately)
	ChatSession *ChatSession `json:"chat_session,omitempty" db:"-"` // Embedded if needed for full fetch
}

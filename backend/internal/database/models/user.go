package models

import (
	"time"

	"github.com/google/uuid"
)

/*
User represents a registered account in the system.

Key Design Notes:

- PasswordHash:
    Stored securely using a strong hash algorithm (e.g., bcrypt, argon2).
    **Never serialized** in API responses. This is a critical security practice.

- DisplayName vs. Username:
    - `Username` is unique and used for login or routing (e.g. /u/username).
    - `DisplayName` is optional and meant for personalization (e.g., "John D.").

- LastActiveAt:
    Updated on each authenticated action. Useful for:
        - "Last seen" indicators
        - Inactive user detection
        - Session tracking

- IsActive:
    Soft-delete mechanism for deactivation (e.g. banned, voluntary deactivation).
    Preferable to hard deletion for data retention or moderation purposes.

Scalability/Future-Proofing Suggestions:

- Add `Role` or `PermissionLevel` for admin/moderator distinction.
- Add `EmailVerifiedAt *time.Time` to support email verification workflows.
- Consider adding `AuthProvider` (e.g. "local", "google") for social login support.
- Use `Metadata map[string]interface{}` to extend user data without schema migration (e.g. feature flags).
*/

type User struct {
	ID           uuid.UUID `json:"id" db:"id"`                               // Unique user ID
	Email        string    `json:"email" db:"email"`                         // Login email (must be unique)
	Username     string    `json:"username" db:"username"`                   // Public-facing unique handle
	PasswordHash string    `json:"-" db:"password_hash"`                     // Never exposed in JSON responses
	DisplayName  *string   `json:"display_name,omitempty" db:"display_name"` // Optional display name
	AvatarURL    *string   `json:"avatar_url,omitempty" db:"avatar_url"`     // Optional profile picture
	CreatedAt    time.Time `json:"created_at" db:"created_at"`               // Account creation timestamp
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`               // Updated on profile change
	LastActiveAt time.Time `json:"last_active_at" db:"last_active_at"`       // Updated on login/activity
	IsActive     bool      `json:"is_active" db:"is_active"`                 // Soft deletion flag
}

/*
CreateUserRequest defines the required and optional fields to register a new user.

Notes:
- Validated on the API layer using struct tags.
- Future extensions may include:
    - Invite codes
    - CAPTCHA tokens
    - Email confirmation steps
*/

type CreateUserRequest struct {
	Email       string  `json:"email" validate:"required,email"`                     // Required, must be valid format
	Username    string  `json:"username" validate:"required,min=3,max=50"`           // Unique handle
	Password    string  `json:"password" validate:"required,min=8"`                  // Strong password required
	DisplayName *string `json:"display_name,omitempty" validate:"omitempty,max=100"` // Optional personalization
}

/*
UserProfile exposes a subset of user data for public consumption (e.g., comments, chat ownership).

This structure allows separation of concerns:
- Keeps private info (email, password hash) internal
- Ensures public views use a sanitized, limited format

Scalable for social features or profile pages.

Future Suggestions:
- Add a `Bio`, `Location`, or `Website` field
- Track public post/comment count or reputation score
*/

type UserProfile struct {
	ID          uuid.UUID `json:"id"`                     // Public identifier
	Username    string    `json:"username"`               // Shown in URLs/UI
	DisplayName *string   `json:"display_name,omitempty"` // Friendly name
	AvatarURL   *string   `json:"avatar_url,omitempty"`   // Profile picture
	CreatedAt   time.Time `json:"created_at"`             // Public account age
}

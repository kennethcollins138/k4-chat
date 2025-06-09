package sessions

import (
	"context"
	"time"
)

// Session represents a user's active session
type Session struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	DeviceID     string    `json:"device_id"`
	IPAddress    string    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	LastUsed     time.Time `json:"last_used"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	IsActive     bool      `json:"is_active"`
	TokenVersion int       `json:"token_version"`
	// OAuth and provider support
	Provider   string `json:"provider,omitempty"`    // "local", "google", "facebook", "github", etc.
	ProviderID string `json:"provider_id,omitempty"` // OAuth provider's user ID
	// Security enhancements
	Fingerprint string `json:"fingerprint,omitempty"` // Device fingerprint for additional security
	LoginMethod string `json:"login_method"`          // "password", "oauth", "sso"
	// Session metadata
	Region     string `json:"region,omitempty"`      // Geographic region for sharding
	ClientType string `json:"client_type,omitempty"` // "web", "mobile", "api"
}

// SessionMetadata contains information about the session's context
type SessionMetadata struct {
	DeviceID    string    `json:"device_id"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	LastUsed    time.Time `json:"last_used"`
	CreatedAt   time.Time `json:"created_at"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Region      string    `json:"region,omitempty"`
	ClientType  string    `json:"client_type,omitempty"`
}

// OAuthSessionRequest represents data for creating OAuth sessions
type OAuthSessionRequest struct {
	Provider     string                 `json:"provider"`             // OAuth provider name
	ProviderID   string                 `json:"provider_id"`          // User ID from OAuth provider
	Email        string                 `json:"email"`                // Email from OAuth provider
	Username     string                 `json:"username"`             // Username from OAuth provider
	DisplayName  string                 `json:"display_name"`         // Display name from OAuth provider
	AvatarURL    string                 `json:"avatar_url"`           // Avatar URL from OAuth provider
	AccessToken  string                 `json:"access_token"`         // OAuth access token
	RefreshToken string                 `json:"refresh_token"`        // OAuth refresh token (if available)
	ExpiresAt    *time.Time             `json:"expires_at"`           // OAuth token expiration
	Scopes       []string               `json:"scopes"`               // Granted OAuth scopes
	Metadata     *SessionMetadata       `json:"metadata"`             // Session metadata
	ExtraData    map[string]interface{} `json:"extra_data,omitempty"` // Additional provider-specific data
}

// SessionCreateRequest contains all data needed to create a session
type SessionCreateRequest struct {
	UserID       string           `json:"user_id"`
	TokenVersion int              `json:"token_version"`
	Metadata     *SessionMetadata `json:"metadata"`
	LoginMethod  string           `json:"login_method"`
	Provider     string           `json:"provider,omitempty"`
	ProviderID   string           `json:"provider_id,omitempty"`
}

// SessionValidationResult contains the result of session validation
type SessionValidationResult struct {
	Valid        bool      `json:"valid"`
	Session      *Session  `json:"session,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	TokenVersion int       `json:"token_version"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SessionManager defines the interface for session management
type SessionManager interface {
	// CreateSession creates a new session for a user
	CreateSession(ctx context.Context, req *SessionCreateRequest) (*Session, error)

	// CreateOAuthSession creates a session from OAuth flow
	CreateOAuthSession(ctx context.Context, req *OAuthSessionRequest) (*Session, error)

	// GetSession retrieves a session by its ID
	GetSession(ctx context.Context, sessionID string) (*Session, error)

	// ListSessions retrieves all active sessions for a user
	ListSessions(ctx context.Context, userID string) ([]*Session, error)

	// UpdateSession updates session metadata
	UpdateSession(ctx context.Context, sessionID string, metadata *SessionMetadata) error

	// ValidateSession checks if a session is valid and returns detailed result
	ValidateSession(ctx context.Context, sessionID string) (*SessionValidationResult, error)

	// RevokeSession invalidates a specific session
	RevokeSession(ctx context.Context, sessionID string) error

	// RevokeAllSessions invalidates all sessions for a user
	RevokeAllSessions(ctx context.Context, userID string) error

	// RevokeAllSessionsExcept invalidates all sessions for a user except the specified one
	RevokeAllSessionsExcept(ctx context.Context, userID string, keepSessionID string) error

	// RefreshSession extends session lifetime and updates metadata
	RefreshSession(ctx context.Context, sessionID string, metadata *SessionMetadata) (*Session, error)

	// GetSessionsByProvider gets sessions filtered by OAuth provider
	GetSessionsByProvider(ctx context.Context, userID string, provider string) ([]*Session, error)

	// CleanupExpiredSessions removes expired sessions
	CleanupExpiredSessions(ctx context.Context) (int64, error)
}

package tokens

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UserTier represents user subscription/permission tiers
type UserTier string

const (
	TierFree      UserTier = "free"
	TierPremium   UserTier = "premium"
	TierAdmin     UserTier = "admin"
	TierModerator UserTier = "moderator"
)

// TokenPair represents a pair of access and refresh tokens
type TokenPair struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	ExpiresIn       int64  `json:"expires_in"`        // Access token expiration time in seconds
	RefreshTokenTTL int64  `json:"refresh_token_ttl"` // Refresh token expiration time in seconds
	TokenType       string `json:"token_type"`        // "Bearer"
	Scope           string `json:"scope,omitempty"`   // OAuth scopes
}

// TokenClaims represents the claims in a JWT token
type TokenClaims struct {
	UserID       string   `json:"user_id"`
	SessionID    string   `json:"session_id"`
	Tier         UserTier `json:"tier"` // Simple user tier instead of roles array
	TokenVersion int      `json:"token_version"`
	// OAuth and provider support
	Provider   string   `json:"provider,omitempty"`    // OAuth provider
	ProviderID string   `json:"provider_id,omitempty"` // OAuth provider user ID
	Scopes     []string `json:"scopes,omitempty"`      // OAuth scopes
	// Security and binding
	DeviceID    string `json:"device_id,omitempty"`   // Device binding
	Fingerprint string `json:"fingerprint,omitempty"` // Browser/client fingerprint
	// Metadata
	LoginMethod string `json:"login_method,omitempty"` // "password", "oauth", "sso"
	Region      string `json:"region,omitempty"`       // Geographic region
	ClientType  string `json:"client_type,omitempty"`  // "web", "mobile", "api"
	jwt.RegisteredClaims
}

// TokenConfig contains configuration for token generation
type TokenConfig struct {
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	JWTSecret       string
	// Security settings
	EnableTokenBinding bool // Whether to bind tokens to device/fingerprint
	EnableRotation     bool // Whether to rotate refresh tokens
	// OAuth settings
	OAuthEnabled   bool
	OAuthProviders []string // Supported OAuth providers
}

// RefreshTokenData represents the data stored for a refresh token
type RefreshTokenData struct {
	UserID       string   `json:"user_id"`
	Username     string   `json:"username"`
	SessionID    string   `json:"session_id"`
	Tier         UserTier `json:"tier"` // Simple user tier instead of roles array
	TokenVersion int      `json:"token_version"`
	CreatedAt    int64    `json:"created_at"`
	LastUsedAt   int64    `json:"last_used_at"`
	FamilyID     string   `json:"family_id"` // For token rotation tracking
	// OAuth support
	Provider   string   `json:"provider,omitempty"`
	ProviderID string   `json:"provider_id,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	// Security binding
	DeviceID    string `json:"device_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	// Metadata
	LoginMethod string `json:"login_method,omitempty"`
	Region      string `json:"region,omitempty"`
	ClientType  string `json:"client_type,omitempty"`
}

// TokenGenerationRequest contains all data needed to generate tokens
type TokenGenerationRequest struct {
	UserID       string   `json:"user_id"`
	SessionID    string   `json:"session_id"`
	Tier         UserTier `json:"tier"` // Simple user tier instead of roles array
	TokenVersion int      `json:"token_version"`
	// OAuth support
	Provider   string   `json:"provider,omitempty"`
	ProviderID string   `json:"provider_id,omitempty"`
	Scopes     []string `json:"scopes,omitempty"`
	// Security binding
	DeviceID    string `json:"device_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	// Metadata
	LoginMethod string `json:"login_method,omitempty"`
	Region      string `json:"region,omitempty"`
	ClientType  string `json:"client_type,omitempty"`
}

// TokenValidationRequest contains data for token validation
type TokenValidationRequest struct {
	Token       string `json:"token"`
	DeviceID    string `json:"device_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	UserAgent   string `json:"user_agent,omitempty"`
}

// TokenValidationResult contains the result of token validation
type TokenValidationResult struct {
	Valid        bool         `json:"valid"`
	Claims       *TokenClaims `json:"claims,omitempty"`
	Reason       string       `json:"reason,omitempty"`
	ExpiresAt    time.Time    `json:"expires_at,omitempty"`
	RefreshAfter time.Time    `json:"refresh_after,omitempty"`
}

// SessionTokenContext represents the coordination between session and token
type SessionTokenContext struct {
	SessionID    string    `json:"session_id"`
	TokenVersion int       `json:"token_version"`
	UserID       string    `json:"user_id"`
	ValidUntil   time.Time `json:"valid_until"`
	LastActivity time.Time `json:"last_activity"`
}

// HasTier checks if the user has the required tier or higher
func (t UserTier) HasTier(required UserTier) bool {
	tierLevels := map[UserTier]int{
		TierFree:      0,
		TierPremium:   1,
		TierModerator: 2,
		TierAdmin:     3,
	}

	currentLevel, exists := tierLevels[t]
	if !exists {
		return false
	}

	requiredLevel, exists := tierLevels[required]
	if !exists {
		return false
	}

	return currentLevel >= requiredLevel
}

// TokenStore defines the interface for token management
type TokenStore interface {
	// GenerateTokenPair generates a new pair of access and refresh tokens
	GenerateTokenPair(ctx context.Context, req *TokenGenerationRequest) (*TokenPair, error)

	// ValidateToken validates a token and returns its claims
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)

	// ValidateTokenWithBinding validates a token with device/fingerprint binding
	ValidateTokenWithBinding(ctx context.Context, req *TokenValidationRequest) (*TokenValidationResult, error)

	// ValidateTokenWithSession validates a token and ensures it's bound to a specific session and device
	ValidateTokenWithSession(ctx context.Context, token string, sessionID string, deviceID string) (*TokenClaims, error)

	// RefreshToken generates a new token pair using a refresh token
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)

	// RefreshTokenWithBinding refreshes token with binding validation
	RefreshTokenWithBinding(ctx context.Context, req *TokenValidationRequest) (*TokenPair, error)

	// RevokeToken invalidates a token
	RevokeToken(ctx context.Context, token string) error

	// RevokeAllTokens invalidates all tokens for a user
	RevokeAllTokens(ctx context.Context, userID string) error

	// RevokeSessionTokens invalidates all tokens for a specific session
	RevokeSessionTokens(ctx context.Context, sessionID string) error

	// StoreRefreshToken stores a refresh token and its associated data
	StoreRefreshToken(ctx context.Context, tokenID string, data *RefreshTokenData, ttl time.Duration) error
	// GetRefreshToken retrieves the data associated with a refresh token
	GetRefreshToken(ctx context.Context, tokenID string) (*RefreshTokenData, error)
	// DeleteRefreshToken removes a refresh token
	DeleteRefreshToken(ctx context.Context, tokenID string) error
	// DeleteUserRefreshTokens removes all refresh tokens for a user
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
	// BlacklistToken adds a token to the blacklist
	BlacklistToken(ctx context.Context, tokenID string, ttl time.Duration) error
	// IsTokenBlacklisted checks if a token is blacklisted
	IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error)
	// RotateRefreshToken rotates a refresh token
	RotateRefreshToken(ctx context.Context, oldTokenID string, newTokenID string, data *RefreshTokenData, ttl time.Duration) error

	// OAuth specific methods
	// StoreOAuthTokens stores OAuth provider tokens
	StoreOAuthTokens(ctx context.Context, userID string, provider string, accessToken string, refreshToken string, expiresAt time.Time) error
	// GetOAuthTokens retrieves OAuth provider tokens
	GetOAuthTokens(ctx context.Context, userID string, provider string) (accessToken string, refreshToken string, expiresAt time.Time, err error)
	// RefreshOAuthToken refreshes OAuth provider token
	RefreshOAuthToken(ctx context.Context, userID string, provider string) (*TokenPair, error)

	// Session coordination methods
	// ValidateSessionTokenSync validates that session and token are synchronized
	ValidateSessionTokenSync(ctx context.Context, sessionID string, tokenVersion int) (*SessionTokenContext, error)
	// UpdateSessionTokenSync updates session-token coordination data
	UpdateSessionTokenSync(ctx context.Context, sessionID string, tokenVersion int) error
}

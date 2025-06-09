package tokens

import "errors"

// Token-related errors
var (
	ErrTokenExpired         = errors.New("token has expired")
	ErrTokenInvalid         = errors.New("token is invalid")
	ErrTokenBlacklisted     = errors.New("token has been blacklisted")
	ErrTokenRotationFailed  = errors.New("token rotation failed")
	ErrTokenNotFound        = errors.New("token not found")
	ErrTokenVersionMismatch = errors.New("token version mismatch")
	ErrTokenReuse           = errors.New("token reuse detected")
)

// Token-related constants
const (
	// Token types
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"

	// Redis key prefixes
	RedisKeyRefreshToken = "refresh_token:"
	RedisKeyBlacklist    = "blacklist:"
	RedisKeyUserTokens   = "user_refresh_tokens:"

	// Default TTLs (in seconds)
	DefaultAccessTokenTTL  = 900    // 15 minutes
	DefaultRefreshTokenTTL = 604800 // 7 days
	DefaultBlacklistTTL    = 86400  // 24 hours
)

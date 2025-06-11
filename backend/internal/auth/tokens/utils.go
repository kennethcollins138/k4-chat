package tokens

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// GenerateRefreshTokenID generates a unique ID for a refresh token
func GenerateRefreshTokenID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate refresh token ID: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

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

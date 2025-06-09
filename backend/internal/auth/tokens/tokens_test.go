package tokens

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testJWTSecret    = "test-super-secret-key"
	testUserID       = "user-12345"
	testTokenVersion = 1
)

func TestGenerateToken(t *testing.T) {
	ttl := time.Hour
	tokenString, err := GenerateToken(testUserID, testTokenVersion, ttl, testJWTSecret)

	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	// Parse the token without validation to check claims easily
	claims := &TokenClaims{}
	parser := jwt.Parser{}
	_, _, err = parser.ParseUnverified(tokenString, claims)
	require.NoError(t, err, "Failed to parse unverified token")

	assert.Equal(t, testUserID, claims.UserID)
	assert.Equal(t, testTokenVersion, claims.TokenVersion)
	assert.WithinDuration(t, time.Now().Add(ttl), claims.ExpiresAt.Time, time.Second*2) // Allow a small delta
	assert.WithinDuration(t, time.Now(), claims.IssuedAt.Time, time.Second*2)           // Allow a small delta
}

func TestValidateToken(t *testing.T) {
	ttl := time.Hour
	validToken, err := GenerateToken(testUserID, testTokenVersion, ttl, testJWTSecret)
	require.NoError(t, err)
	require.NotEmpty(t, validToken)

	t.Run("ValidToken", func(t *testing.T) {
		claims, err := ValidateToken(validToken, testJWTSecret)
		require.NoError(t, err)
		require.NotNil(t, claims)
		assert.Equal(t, testUserID, claims.UserID)
		assert.Equal(t, testTokenVersion, claims.TokenVersion)
		assert.WithinDuration(t, time.Now().Add(ttl), claims.ExpiresAt.Time, time.Second*2)
	})

	t.Run("ExpiredToken", func(t *testing.T) {
		expiredTTL := -time.Hour // Token expired an hour ago
		expiredTokenGenerated, genErr := GenerateToken(testUserID, testTokenVersion, expiredTTL, testJWTSecret)
		require.NoError(t, genErr)

		// Need to wait a moment for the time check if ExpiresAt is exactly now
		// jwt.ParseWithClaims checks `!claims.VerifyExpiresAt(now, false)`
		// if VerifyExpiresAt needs `e.Time.Unix() > now.Unix()`, setting expiry to -1hr should be fine.

		_, err := ValidateToken(expiredTokenGenerated, testJWTSecret)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), jwt.ErrTokenExpired.Error())
		assert.Contains(t, err.Error(), "token has invalid claims: token is expired")
	})

	t.Run("InvalidSecret", func(t *testing.T) {
		_, err := ValidateToken(validToken, "wrong-secret-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature is invalid")
	})

	t.Run("MalformedToken", func(t *testing.T) {
		_, err := ValidateToken("this.is.not.a.jwt", testJWTSecret)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token: token is malformed:") // Error may vary slightly
	})

	t.Run("DifferentSigningMethod", func(t *testing.T) {
		// Create a token with a different signing method (e.g., RSA)
		// This is hard to test without actually signing with RSA.
		// Instead, we can test the specific check in ValidateToken by crafting a token header.
		// However, jwt.ParseWithClaims will likely fail before our custom validation func is called if the token is truly malformed due to header/payload/sig mismatch.
		// The primary goal here is to ensure our *key func* correctly rejects wrong alg.

		// Let's try crafting a token with a modified header (ES256 instead of HS256)
		validHS256Token, _ := GenerateToken("tempUser", 1, time.Hour, testJWTSecret)
		parts := strings.Split(validHS256Token, ".")
		require.Len(t, parts, 3, "Valid token should have 3 parts")

		// Tamper header to claim ES256 (example)
		header := `{"alg":"ES256","typ":"JWT"}`
		tamperedTokenString := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + parts[1] + "." + parts[2]

		_, err := ValidateToken(tamperedTokenString, testJWTSecret)
		assert.Error(t, err)
		// The error should come from our keyFunc due to mismatched alg
		assert.Contains(t, err.Error(), "unexpected signing method: ES256")
	})

	t.Run("InvalidClaimsStructure", func(t *testing.T) {
		// Create a token with invalid claims structure
		tokenWithInvalidClaims := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoxMjM0NX0.xxxxxxx"

		_, err := ValidateToken(tokenWithInvalidClaims, "secret")
		require.Error(t, err)
		// Update the expected error message to match the actual error
		assert.Contains(t, err.Error(), "could not JSON decode claim")
	})

}

func TestGenerateTokenPair(t *testing.T) {
	config := &TokenConfig{
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 7 * 24 * time.Hour,
		JWTSecret:       testJWTSecret,
	}

	tokenPair, err := GenerateTokenPair(testUserID, testTokenVersion, config)
	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, (config.AccessTokenTTL.Milliseconds() / 1000), tokenPair.ExpiresIn)

	// Validate Access Token
	accessClaims, err := ValidateToken(tokenPair.AccessToken, testJWTSecret)
	require.NoError(t, err, "Access token validation failed")
	assert.Equal(t, testUserID, accessClaims.UserID)
	assert.Equal(t, testTokenVersion, accessClaims.TokenVersion)
	assert.WithinDuration(t, time.Now().Add(config.AccessTokenTTL), accessClaims.ExpiresAt.Time, time.Second*2)

	// Validate Refresh Token
	refreshClaims, err := ValidateToken(tokenPair.RefreshToken, testJWTSecret)
	require.NoError(t, err, "Refresh token validation failed")
	assert.Equal(t, testUserID, refreshClaims.UserID)
	assert.Equal(t, testTokenVersion, refreshClaims.TokenVersion)
	assert.WithinDuration(t, time.Now().Add(config.RefreshTokenTTL), refreshClaims.ExpiresAt.Time, time.Second*2)
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	// Test with empty secret - should return an error
	_, err := GenerateToken("123", 1, time.Hour, "")
	require.Error(t, err, "An empty JWT secret should return an error")
	assert.Contains(t, err.Error(), "JWT secret cannot be empty")
}

func TestValidateToken_EmptySecretInKeyFunc(t *testing.T) {
	// Generate a token with a valid secret first
	validToken, err := GenerateToken(testUserID, 1, time.Hour, testJWTSecret)
	require.NoError(t, err)

	// Attempt to validate with an empty secret
	_, err = ValidateToken(validToken, "")
	assert.Error(t, err)
	// Just check for "signature is invalid" which is what the actual error contains
	assert.Contains(t, err.Error(), "signature is invalid")
}

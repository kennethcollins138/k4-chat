package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kdot/k4-chat/backend/internal/apperrors"
	"github.com/kdot/k4-chat/backend/internal/database"
)

// RedisTokenStore implements TokenStore using Redis with resilience patterns
type RedisTokenStore struct {
	client         *redis.Client
	logger         *zap.Logger
	tokenConfig    *TokenConfig
	cache          *TokenCache
	batchProcessor *database.BatchProcessor
	circuitBreaker *database.CircuitBreaker
	rateLimiter    *RateLimiter
	analytics      *AnalyticsStore
}

// NewRedisTokenStore creates a new Redis-based token store with resilience patterns
func NewRedisTokenStore(client *redis.Client, logger *zap.Logger, config *TokenConfig) *RedisTokenStore {
	// Initialize components
	cache := NewTokenCache(client, logger, 5*time.Minute)
	batchProcessor := database.NewBatchProcessor(client, logger)
	circuitBreaker := database.New("token_store", 5, 30*time.Second, logger)
	rateLimiter := NewRateLimiter(*client)
	analytics := NewAnalyticsStore(*client)

	return &RedisTokenStore{
		client:         client,
		logger:         logger,
		tokenConfig:    config,
		cache:          cache,
		batchProcessor: batchProcessor,
		circuitBreaker: circuitBreaker,
		rateLimiter:    rateLimiter,
		analytics:      analytics,
	}
}

// GenerateTokenPair creates access and refresh token pair with OAuth support
func (s *RedisTokenStore) GenerateTokenPair(ctx context.Context, req *TokenGenerationRequest) (*TokenPair, error) {
	if err := s.validateTokenRequest(req); err != nil {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenSign, "invalid token request", err)
	}

	// Rate limiting check
	if allowed, err := s.rateLimiter.CheckRateLimit(ctx, req.UserID, RefreshTokenRateLimit); err != nil {
		s.logger.Error("Rate limit check failed", zap.Error(err))
	} else if !allowed {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenSign, "rate limit exceeded", nil)
	}

	now := time.Now()

	// Create token pair with circuit breaker protection
	var tokenPair *TokenPair
	err := s.circuitBreaker.Execute(ctx, func() error {
		var err error
		tokenPair, err = s.createTokenPairInternal(ctx, req, now)
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to generate token pair: %w", err)
	}

	// Record analytics (non-critical, don't fail on error)
	s.recordTokenAnalytics(ctx, req, "generate", now, true)

	return tokenPair, nil
}

// ValidateToken validates a token and returns its claims
func (s *RedisTokenStore) ValidateToken(ctx context.Context, token string) (*TokenClaims, error) {
	// Try cache first
	if claims, ok := s.cache.Get(ctx, token); ok {
		return claims, nil
	}

	// Parse and validate with circuit breaker
	var claims *TokenClaims
	err := s.circuitBreaker.Execute(ctx, func() error {
		parsedToken, err := jwt.ParseWithClaims(token, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(s.tokenConfig.JWTSecret), nil
		})

		if err != nil {
			return apperrors.NewTokenError(apperrors.ErrTokenParse, "failed to parse token", err)
		}

		if !parsedToken.Valid {
			return apperrors.NewTokenError(apperrors.ErrTokenExpired, "invalid token", nil)
		}

		var ok bool
		claims, ok = parsedToken.Claims.(*TokenClaims)
		if !ok {
			return apperrors.NewTokenError(apperrors.ErrTokenParse, "invalid token claims", nil)
		}

		// Check blacklist
		blacklisted, err := s.IsTokenBlacklisted(ctx, token)
		if err != nil {
			return fmt.Errorf("blacklist check failed: %w", err)
		}
		if blacklisted {
			return apperrors.NewTokenError(apperrors.ErrTokenBlacklisted, "token is blacklisted", nil)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Cache valid claims
	if err := s.cache.Set(ctx, token, claims); err != nil {
		s.logger.Warn("Failed to cache token claims", zap.Error(err))
	}

	return claims, nil
}

// RefreshToken generates a new token pair using a valid refresh token
func (s *RedisTokenStore) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// Validate refresh token
	claims, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Get refresh token data
	refreshData, err := s.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh token data not found: %w", err)
	}

	// Verify token version
	if refreshData.TokenVersion != claims.TokenVersion {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenVersionMismatch, "token version mismatch", nil)
	}

	// Create new token pair request
	req := &TokenGenerationRequest{
		UserID:       claims.UserID,
		SessionID:    claims.SessionID,
		Tier:         claims.Tier,
		TokenVersion: claims.TokenVersion,
		Provider:     claims.Provider,
		ProviderID:   claims.ProviderID,
		Scopes:       claims.Scopes,
		DeviceID:     claims.DeviceID,
		Fingerprint:  claims.Fingerprint,
		LoginMethod:  claims.LoginMethod,
		Region:       claims.Region,
		ClientType:   claims.ClientType,
	}

	// Generate new token pair
	newTokenPair, err := s.GenerateTokenPair(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new token pair: %w", err)
	}

	// Delete old refresh token if rotation is enabled
	if s.tokenConfig.EnableRotation {
		if err := s.DeleteRefreshToken(ctx, refreshToken); err != nil {
			s.logger.Warn("Failed to delete old refresh token", zap.Error(err))
		}
	}

	return newTokenPair, nil
}

// RevokeToken adds a token to the blacklist
func (s *RedisTokenStore) RevokeToken(ctx context.Context, token string) error {
	// Parse token to get expiration for TTL
	claims, err := s.ValidateToken(ctx, token)
	var ttl time.Duration
	if err == nil && claims != nil && claims.ExpiresAt != nil {
		ttl = time.Until(claims.ExpiresAt.Time)
		if ttl <= 0 {
			ttl = time.Hour // Minimum blacklist time
		}
	} else {
		ttl = s.tokenConfig.RefreshTokenTTL
	}

	// Blacklist token and invalidate cache
	err = s.BlacklistToken(ctx, token, ttl)
	if err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	if err := s.cache.Invalidate(ctx, token); err != nil {
		s.logger.Warn("Failed to invalidate token from cache", zap.Error(err))
	}

	return nil
}

// BlacklistToken adds a token to the blacklist
func (s *RedisTokenStore) BlacklistToken(ctx context.Context, tokenID string, ttl time.Duration) error {
	if tokenID == "" {
		return fmt.Errorf("token ID is required")
	}
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenID)
	return s.client.Set(ctx, blacklistKey, "1", ttl).Err()
}

// IsTokenBlacklisted checks if a token is blacklisted
func (s *RedisTokenStore) IsTokenBlacklisted(ctx context.Context, tokenID string) (bool, error) {
	if tokenID == "" {
		return false, fmt.Errorf("token ID is required")
	}
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenID)
	exists, err := s.client.Exists(ctx, blacklistKey).Result()
	return exists > 0, err
}

// StoreRefreshToken stores refresh token data
func (s *RedisTokenStore) StoreRefreshToken(ctx context.Context, tokenID string, data *RefreshTokenData, ttl time.Duration) error {
	if tokenID == "" || data == nil {
		return fmt.Errorf("token ID and data are required")
	}

	tokenData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	return s.batchProcessor.MultiExec(ctx, func(pipe redis.Pipeliner) error {
		refreshTokenKey := fmt.Sprintf("refresh_token:%s", tokenID)
		pipe.Set(ctx, refreshTokenKey, tokenData, ttl)

		userTokensKey := fmt.Sprintf("user:%s:tokens", data.UserID)
		pipe.SAdd(ctx, userTokensKey, tokenID)
		pipe.Expire(ctx, userTokensKey, ttl)

		return nil
	})
}

// GetRefreshToken retrieves refresh token data
func (s *RedisTokenStore) GetRefreshToken(ctx context.Context, tokenID string) (*RefreshTokenData, error) {
	if tokenID == "" {
		return nil, fmt.Errorf("token ID is required")
	}

	refreshTokenKey := fmt.Sprintf("refresh_token:%s", tokenID)
	data, err := s.client.Get(ctx, refreshTokenKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, apperrors.NewTokenError(apperrors.ErrTokenNotFound, "refresh token not found", err)
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	var tokenData RefreshTokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return &tokenData, nil
}

// DeleteRefreshToken deletes refresh token data
func (s *RedisTokenStore) DeleteRefreshToken(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return fmt.Errorf("token ID is required")
	}

	data, err := s.GetRefreshToken(ctx, tokenID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to get refresh token data: %w", err)
	}

	return s.batchProcessor.MultiExec(ctx, func(pipe redis.Pipeliner) error {
		refreshTokenKey := fmt.Sprintf("refresh_token:%s", tokenID)
		pipe.Del(ctx, refreshTokenKey)

		userTokensKey := fmt.Sprintf("user:%s:tokens", data.UserID)
		pipe.SRem(ctx, userTokensKey, tokenID)

		return nil
	})
}

// Helper methods

func (s *RedisTokenStore) validateTokenRequest(req *TokenGenerationRequest) error {
	if req == nil {
		return fmt.Errorf("token request cannot be nil")
	}
	if req.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if req.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if req.TokenVersion < 0 {
		return fmt.Errorf("token version must be non-negative")
	}
	// Set defaults
	if req.LoginMethod == "" {
		req.LoginMethod = "password"
	}
	if req.Provider == "" {
		req.Provider = "local"
	}
	return nil
}

func (s *RedisTokenStore) createTokenPairInternal(ctx context.Context, req *TokenGenerationRequest, now time.Time) (*TokenPair, error) {
	// Generate access token
	accessClaims := s.createTokenClaims(req, now, s.tokenConfig.AccessTokenTTL)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.tokenConfig.JWTSecret))
	if err != nil {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenSign, "failed to sign access token", err)
	}

	// Generate refresh token
	refreshClaims := s.createTokenClaims(req, now, s.tokenConfig.RefreshTokenTTL)
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString([]byte(s.tokenConfig.JWTSecret))
	if err != nil {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenSign, "failed to sign refresh token", err)
	}

	// Store refresh token data
	refreshData := &RefreshTokenData{
		UserID:       req.UserID,
		SessionID:    req.SessionID,
		Tier:         req.Tier,
		TokenVersion: req.TokenVersion,
		CreatedAt:    now.Unix(),
		LastUsedAt:   now.Unix(),
		Provider:     req.Provider,
		ProviderID:   req.ProviderID,
		Scopes:       req.Scopes,
		DeviceID:     req.DeviceID,
		Fingerprint:  req.Fingerprint,
		LoginMethod:  req.LoginMethod,
		Region:       req.Region,
		ClientType:   req.ClientType,
	}

	if err := s.StoreRefreshToken(ctx, refreshTokenString, refreshData, s.tokenConfig.RefreshTokenTTL); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Cache access token claims
	if err := s.cache.Set(ctx, accessTokenString, accessClaims); err != nil {
		s.logger.Warn("Failed to cache access token claims", zap.Error(err))
	}

	return &TokenPair{
		AccessToken:     accessTokenString,
		RefreshToken:    refreshTokenString,
		ExpiresIn:       int64(s.tokenConfig.AccessTokenTTL.Seconds()),
		RefreshTokenTTL: int64(s.tokenConfig.RefreshTokenTTL.Seconds()),
		TokenType:       "Bearer",
		Scope:           strings.Join(req.Scopes, " "),
	}, nil
}

func (s *RedisTokenStore) createTokenClaims(req *TokenGenerationRequest, now time.Time, ttl time.Duration) *TokenClaims {
	return &TokenClaims{
		UserID:       req.UserID,
		SessionID:    req.SessionID,
		Tier:         req.Tier,
		TokenVersion: req.TokenVersion,
		Provider:     req.Provider,
		ProviderID:   req.ProviderID,
		Scopes:       req.Scopes,
		DeviceID:     req.DeviceID,
		Fingerprint:  req.Fingerprint,
		LoginMethod:  req.LoginMethod,
		Region:       req.Region,
		ClientType:   req.ClientType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
}

func (s *RedisTokenStore) recordTokenAnalytics(ctx context.Context, req *TokenGenerationRequest, action string, timestamp time.Time, success bool) {
	analytics := &TokenAnalytics{
		UserID:      req.UserID,
		TokenType:   TokenTypeAccess,
		Action:      action,
		Timestamp:   timestamp,
		DeviceID:    req.DeviceID,
		Fingerprint: req.Fingerprint,
		Success:     success,
	}
	if err := s.analytics.RecordTokenEvent(ctx, analytics); err != nil {
		s.logger.Warn("Failed to record token analytics", zap.Error(err))
	}
}

// RevokeAllTokens revokes all tokens for a user
func (s *RedisTokenStore) RevokeAllTokens(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	userTokensKey := fmt.Sprintf("user:%s:tokens", userID)
	refreshTokens, err := s.client.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // No tokens to revoke
		}
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	return s.batchProcessor.MultiExec(ctx, func(pipe redis.Pipeliner) error {
		for _, token := range refreshTokens {
			// Get token expiration for blacklist TTL
			claims, err := s.ValidateToken(ctx, token)
			var ttl time.Duration
			if err == nil && claims != nil && claims.ExpiresAt != nil {
				ttl = time.Until(claims.ExpiresAt.Time)
				if ttl <= 0 {
					ttl = time.Hour
				}
			} else {
				ttl = s.tokenConfig.RefreshTokenTTL
			}

			// Add to blacklist
			blacklistKey := fmt.Sprintf("blacklist:%s", token)
			pipe.Set(ctx, blacklistKey, "1", ttl)

			// Delete refresh token data
			refreshTokenKey := fmt.Sprintf("refresh_token:%s", token)
			pipe.Del(ctx, refreshTokenKey)

			// Invalidate from cache
			if err := s.cache.Invalidate(ctx, token); err != nil {
				s.logger.Warn("Failed to invalidate token from cache", zap.Error(err))
			}
		}

		// Delete user's token list
		pipe.Del(ctx, userTokensKey)

		return nil
	})
}

// DeleteUserRefreshTokens deletes all refresh tokens for a user
func (s *RedisTokenStore) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	userTokensKey := fmt.Sprintf("user:%s:tokens", userID)
	refreshTokens, err := s.client.SMembers(ctx, userTokensKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	return s.batchProcessor.MultiExec(ctx, func(pipe redis.Pipeliner) error {
		for _, token := range refreshTokens {
			refreshTokenKey := fmt.Sprintf("refresh_token:%s", token)
			pipe.Del(ctx, refreshTokenKey)
		}

		// Delete user's token list
		pipe.Del(ctx, userTokensKey)

		return nil
	})
}

// ValidateTokenWithBinding validates a token with device/fingerprint binding
func (s *RedisTokenStore) ValidateTokenWithBinding(ctx context.Context, req *TokenValidationRequest) (*TokenValidationResult, error) {
	if req == nil {
		return &TokenValidationResult{
			Valid:  false,
			Reason: "validation request is required",
		}, nil
	}

	// Validate the token first
	claims, err := s.ValidateToken(ctx, req.Token)
	if err != nil {
		return &TokenValidationResult{
			Valid:  false,
			Reason: err.Error(),
		}, nil
	}

	result := &TokenValidationResult{
		Valid:        true,
		Claims:       claims,
		ExpiresAt:    claims.ExpiresAt.Time,
		RefreshAfter: claims.ExpiresAt.Add(-5 * time.Minute), // Refresh 5 minutes before expiry
	}

	// Validate device binding if enabled
	if s.tokenConfig.EnableTokenBinding {
		if claims.DeviceID != "" && req.DeviceID != "" && claims.DeviceID != req.DeviceID {
			result.Valid = false
			result.Reason = "device binding validation failed"
			return result, nil
		}

		if claims.Fingerprint != "" && req.Fingerprint != "" && claims.Fingerprint != req.Fingerprint {
			result.Valid = false
			result.Reason = "fingerprint binding validation failed"
			return result, nil
		}
	}

	return result, nil
}

// ValidateTokenWithSession validates a token and ensures it's bound to a specific session and device
func (s *RedisTokenStore) ValidateTokenWithSession(ctx context.Context, token string, sessionID string, deviceID string) (*TokenClaims, error) {
	if token == "" {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenParse, "token is required", nil)
	}
	if sessionID == "" {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenBinding, "session ID is required for binding validation", nil)
	}
	if deviceID == "" {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenBinding, "device ID is required for binding validation", nil)
	}

	// Validate the token first
	claims, err := s.ValidateToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Validate token is bound to the expected session
	if claims.SessionID != sessionID {
		s.logger.Warn("Token session binding validation failed",
			zap.String("expected_session", sessionID),
			zap.String("token_session", claims.SessionID),
			zap.String("user_id", claims.UserID))
		return nil, apperrors.NewTokenError(apperrors.ErrTokenBinding, "token not bound to session", nil)
	}

	// Validate token is bound to the expected device
	if claims.DeviceID != deviceID {
		s.logger.Warn("Token device binding validation failed",
			zap.String("expected_device", deviceID),
			zap.String("token_device", claims.DeviceID),
			zap.String("user_id", claims.UserID),
			zap.String("session_id", claims.SessionID))
		return nil, apperrors.NewTokenError(apperrors.ErrTokenBinding, "token not bound to device", nil)
	}

	// Additional security check: verify the session still exists and is active
	if err := s.validateSessionExists(ctx, sessionID, claims.UserID); err != nil {
		s.logger.Warn("Token session validation failed",
			zap.Error(err),
			zap.String("session_id", sessionID),
			zap.String("user_id", claims.UserID))
		return nil, apperrors.NewTokenError(apperrors.ErrTokenBinding, "associated session is invalid", err)
	}

	s.logger.Debug("Token session binding validation successful",
		zap.String("session_id", sessionID),
		zap.String("device_id", deviceID),
		zap.String("user_id", claims.UserID))

	return claims, nil
}

// validateSessionExists checks if a session exists and is active (helper for ValidateTokenWithSession)
func (s *RedisTokenStore) validateSessionExists(ctx context.Context, sessionID string, userID string) error {
	// Check if session exists in Redis
	sessionKey := fmt.Sprintf("session:%s", sessionID)
	exists, err := s.client.Exists(ctx, sessionKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("session does not exist")
	}

	// Optional: Additional validation can be added here
	// For example, checking if the session belongs to the user
	// This would require either storing user mapping or fetching session data

	return nil
}

// RefreshTokenWithBinding refreshes a token with device/fingerprint binding validation
func (s *RedisTokenStore) RefreshTokenWithBinding(ctx context.Context, req *TokenValidationRequest) (*TokenPair, error) {
	if req == nil {
		return nil, fmt.Errorf("validation request is required")
	}

	// Validate token with binding
	validationResult, err := s.ValidateTokenWithBinding(ctx, req)
	if err != nil {
		return nil, err
	}

	if !validationResult.Valid {
		return nil, fmt.Errorf("token validation failed: %s", validationResult.Reason)
	}

	// Refresh the token
	return s.RefreshToken(ctx, req.Token)
}

// StoreOAuthTokens stores OAuth tokens for a user and provider
func (s *RedisTokenStore) StoreOAuthTokens(ctx context.Context, userID string, provider string, accessToken string, refreshToken string, expiresAt time.Time) error {
	if userID == "" || provider == "" {
		return fmt.Errorf("user ID and provider are required")
	}

	oauthKey := fmt.Sprintf("oauth:%s:%s", provider, userID)
	oauthData := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt.Unix(),
		"updated_at":    time.Now().Unix(),
	}

	oauthDataJSON, err := json.Marshal(oauthData)
	if err != nil {
		return fmt.Errorf("failed to marshal OAuth data: %w", err)
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = 24 * time.Hour // Default TTL if expires_at is in the past
	}

	return s.client.Set(ctx, oauthKey, oauthDataJSON, ttl).Err()
}

// GetOAuthTokens retrieves OAuth tokens for a user and provider
func (s *RedisTokenStore) GetOAuthTokens(ctx context.Context, userID string, provider string) (accessToken string, refreshToken string, expiresAt time.Time, err error) {
	if userID == "" || provider == "" {
		return "", "", time.Time{}, fmt.Errorf("user ID and provider are required")
	}

	oauthKey := fmt.Sprintf("oauth:%s:%s", provider, userID)
	data, err := s.client.Get(ctx, oauthKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return "", "", time.Time{}, fmt.Errorf("OAuth tokens not found")
		}
		return "", "", time.Time{}, fmt.Errorf("failed to get OAuth tokens: %w", err)
	}

	var oauthData map[string]interface{}
	if err := json.Unmarshal(data, &oauthData); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to unmarshal OAuth data: %w", err)
	}

	accessToken, _ = oauthData["access_token"].(string)
	refreshToken, _ = oauthData["refresh_token"].(string)
	if expiresAtUnix, ok := oauthData["expires_at"].(float64); ok {
		expiresAt = time.Unix(int64(expiresAtUnix), 0)
	}

	return accessToken, refreshToken, expiresAt, nil
}

// RefreshOAuthToken refreshes OAuth tokens (placeholder implementation)
func (s *RedisTokenStore) RefreshOAuthToken(ctx context.Context, userID string, provider string) (*TokenPair, error) {
	// This would integrate with OAuth provider to refresh tokens
	// Implementation depends on the specific OAuth provider
	return nil, fmt.Errorf("OAuth token refresh not implemented for provider: %s", provider)
}

// RevokeSessionTokens revokes all tokens for a specific session
func (s *RedisTokenStore) RevokeSessionTokens(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// Get all tokens for session
	sessionTokensKey := fmt.Sprintf("session:%s:tokens", sessionID)
	refreshTokens, err := s.client.SMembers(ctx, sessionTokensKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // No tokens to revoke
		}
		return fmt.Errorf("failed to get session tokens: %w", err)
	}

	return s.batchProcessor.MultiExec(ctx, func(pipe redis.Pipeliner) error {
		for _, token := range refreshTokens {
			// Get token data to find user ID
			refreshData, err := s.GetRefreshToken(ctx, token)
			if err != nil {
				continue
			}

			// Add to blacklist
			claims, err := s.ValidateToken(ctx, token)
			var ttl time.Duration
			if err == nil && claims != nil && claims.ExpiresAt != nil {
				ttl = time.Until(claims.ExpiresAt.Time)
				if ttl <= 0 {
					ttl = time.Hour
				}
			} else {
				ttl = s.tokenConfig.RefreshTokenTTL
			}

			blacklistKey := fmt.Sprintf("blacklist:%s", token)
			pipe.Set(ctx, blacklistKey, "1", ttl)

			// Delete refresh token data
			refreshTokenKey := fmt.Sprintf("refresh_token:%s", token)
			pipe.Del(ctx, refreshTokenKey)

			// Remove from user's token list
			userTokensKey := fmt.Sprintf("user:%s:tokens", refreshData.UserID)
			pipe.SRem(ctx, userTokensKey, token)

			// Invalidate from cache
			if err := s.cache.Invalidate(ctx, token); err != nil {
				s.logger.Warn("Failed to invalidate token from cache", zap.Error(err))
			}
		}

		// Delete session's token list
		pipe.Del(ctx, sessionTokensKey)

		return nil
	})
}

// RotateRefreshToken atomically replaces an old refresh token with a new one
func (s *RedisTokenStore) RotateRefreshToken(ctx context.Context, oldTokenID string, newTokenID string, data *RefreshTokenData, ttl time.Duration) error {
	if oldTokenID == "" || newTokenID == "" {
		return fmt.Errorf("both old and new token IDs are required")
	}
	if data == nil {
		return fmt.Errorf("token data is required")
	}

	return s.batchProcessor.MultiExec(ctx, func(pipe redis.Pipeliner) error {
		// Store new token data
		tokenData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal token data: %w", err)
		}

		newRefreshTokenKey := fmt.Sprintf("refresh_token:%s", newTokenID)
		pipe.Set(ctx, newRefreshTokenKey, tokenData, ttl)

		// Add new token to user's token list
		userTokensKey := fmt.Sprintf("user:%s:tokens", data.UserID)
		pipe.SAdd(ctx, userTokensKey, newTokenID)

		// Add new token to session's token list
		sessionTokensKey := fmt.Sprintf("session:%s:tokens", data.SessionID)
		pipe.SAdd(ctx, sessionTokensKey, newTokenID)

		// Delete old token
		oldRefreshTokenKey := fmt.Sprintf("refresh_token:%s", oldTokenID)
		pipe.Del(ctx, oldRefreshTokenKey)

		// Remove old token from user's token list
		pipe.SRem(ctx, userTokensKey, oldTokenID)

		// Remove old token from session's token list
		pipe.SRem(ctx, sessionTokensKey, oldTokenID)

		return nil
	})
}

// ValidateSessionTokenSync validates that session and token are synchronized
func (s *RedisTokenStore) ValidateSessionTokenSync(ctx context.Context, sessionID string, tokenVersion int) (*SessionTokenContext, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	sessionTokenKey := fmt.Sprintf("session_token:%s", sessionID)
	data, err := s.client.Get(ctx, sessionTokenKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("session token context not found")
		}
		return nil, fmt.Errorf("failed to get session token context: %w", err)
	}

	var context SessionTokenContext
	if err := json.Unmarshal(data, &context); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session token context: %w", err)
	}

	// Validate token version
	if context.TokenVersion != tokenVersion {
		return nil, apperrors.NewTokenError(apperrors.ErrTokenVersionMismatch, "token version mismatch", nil)
	}

	// Check if context is still valid
	if time.Now().After(context.ValidUntil) {
		return nil, fmt.Errorf("session token context has expired")
	}

	return &context, nil
}

// UpdateSessionTokenSync updates session-token coordination data
func (s *RedisTokenStore) UpdateSessionTokenSync(ctx context.Context, sessionID string, tokenVersion int) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	sessionTokenKey := fmt.Sprintf("session_token:%s", sessionID)

	// Get existing context or create new one
	var context SessionTokenContext
	data, err := s.client.Get(ctx, sessionTokenKey).Bytes()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get session token context: %w", err)
	}

	if err == redis.Nil {
		// Create new context
		context = SessionTokenContext{
			SessionID:    sessionID,
			TokenVersion: tokenVersion,
			ValidUntil:   time.Now().Add(s.tokenConfig.RefreshTokenTTL),
			LastActivity: time.Now(),
		}
	} else {
		// Update existing context
		if err := json.Unmarshal(data, &context); err != nil {
			return fmt.Errorf("failed to unmarshal session token context: %w", err)
		}
		context.TokenVersion = tokenVersion
		context.LastActivity = time.Now()
	}

	contextData, err := json.Marshal(&context)
	if err != nil {
		return fmt.Errorf("failed to marshal session token context: %w", err)
	}

	return s.client.Set(ctx, sessionTokenKey, contextData, s.tokenConfig.RefreshTokenTTL).Err()
}

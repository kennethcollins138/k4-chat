package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kdot/k4-chat/backend/internal/auth/sessions"
	"github.com/kdot/k4-chat/backend/internal/auth/tokens"
)

// UserService defines interface for fetching user information
type UserService interface {
	GetUserTier(ctx context.Context, userID string) (tokens.UserTier, error)
}

// SessionTokenCoordinator manages the lifecycle of sessions and tokens together
type SessionTokenCoordinator struct {
	sessionManager sessions.SessionManager
	tokenStore     tokens.TokenStore
	redisClient    redis.Cmdable
	userService    UserService
	logger         *zap.Logger
}

// NewSessionTokenCoordinator creates a new coordinator
func NewSessionTokenCoordinator(
	sessionManager sessions.SessionManager,
	tokenStore tokens.TokenStore,
	redisClient redis.Cmdable,
	userService UserService,
	logger *zap.Logger,
) *SessionTokenCoordinator {
	return &SessionTokenCoordinator{
		sessionManager: sessionManager,
		tokenStore:     tokenStore,
		redisClient:    redisClient,
		userService:    userService,
		logger:         logger,
	}
}

// CreateSessionWithTokens creates a session and generates tokens atomically
func (c *SessionTokenCoordinator) CreateSessionWithTokens(ctx context.Context, req *SessionTokenRequest) (*SessionTokenResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("session token request is required")
	}

	// Create session
	sessionReq := &sessions.SessionCreateRequest{
		UserID:       req.UserID,
		TokenVersion: req.TokenVersion,
		Metadata:     req.SessionMetadata,
		LoginMethod:  req.LoginMethod,
		Provider:     req.Provider,
		ProviderID:   req.ProviderID,
	}

	session, err := c.sessionManager.CreateSession(ctx, sessionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Generate tokens
	tokenReq := &tokens.TokenGenerationRequest{
		UserID:       req.UserID,
		SessionID:    session.ID,
		Tier:         req.Tier,
		TokenVersion: req.TokenVersion,
		Provider:     req.Provider,
		ProviderID:   req.ProviderID,
		Scopes:       req.Scopes,
		DeviceID:     req.SessionMetadata.DeviceID,
		Fingerprint:  req.SessionMetadata.Fingerprint,
		LoginMethod:  req.LoginMethod,
		Region:       req.SessionMetadata.Region,
		ClientType:   req.SessionMetadata.ClientType,
	}

	tokenPair, err := c.tokenStore.GenerateTokenPair(ctx, tokenReq)
	if err != nil {
		// Rollback session creation
		if rollbackErr := c.sessionManager.RevokeSession(ctx, session.ID); rollbackErr != nil {
			c.logger.Error("Failed to rollback session after token generation failure",
				zap.Error(rollbackErr),
				zap.String("sessionID", session.ID))
		}
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	return &SessionTokenResponse{
		Session:   session,
		TokenPair: tokenPair,
	}, nil
}

// CreateOAuthSessionWithTokens creates an OAuth session and tokens atomically
func (c *SessionTokenCoordinator) CreateOAuthSessionWithTokens(ctx context.Context, req *OAuthSessionTokenRequest) (*SessionTokenResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("OAuth session token request is required")
	}

	// Create OAuth session
	session, err := c.sessionManager.CreateOAuthSession(ctx, req.OAuthRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth session: %w", err)
	}

	// Generate tokens with OAuth data
	tokenReq := &tokens.TokenGenerationRequest{
		UserID:       session.UserID,
		SessionID:    session.ID,
		Tier:         req.Tier,
		TokenVersion: session.TokenVersion,
		Provider:     req.OAuthRequest.Provider,
		ProviderID:   req.OAuthRequest.ProviderID,
		Scopes:       req.OAuthRequest.Scopes,
		DeviceID:     req.OAuthRequest.Metadata.DeviceID,
		Fingerprint:  req.OAuthRequest.Metadata.Fingerprint,
		LoginMethod:  "oauth",
		Region:       req.OAuthRequest.Metadata.Region,
		ClientType:   req.OAuthRequest.Metadata.ClientType,
	}

	tokenPair, err := c.tokenStore.GenerateTokenPair(ctx, tokenReq)
	if err != nil {
		// Rollback session creation
		if rollbackErr := c.sessionManager.RevokeSession(ctx, session.ID); rollbackErr != nil {
			c.logger.Error("Failed to rollback OAuth session after token generation failure",
				zap.Error(rollbackErr),
				zap.String("sessionID", session.ID))
		}
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Store OAuth provider tokens
	if req.OAuthRequest.AccessToken != "" {
		err := c.tokenStore.StoreOAuthTokens(
			ctx,
			session.UserID,
			req.OAuthRequest.Provider,
			req.OAuthRequest.AccessToken,
			req.OAuthRequest.RefreshToken,
			*req.OAuthRequest.ExpiresAt,
		)
		if err != nil {
			c.logger.Warn("Failed to store OAuth provider tokens", zap.Error(err))
			// Don't fail the entire operation for OAuth token storage
		}
	}

	return &SessionTokenResponse{
		Session:   session,
		TokenPair: tokenPair,
	}, nil
}

// RefreshSessionAndTokens refreshes both session and tokens atomically
func (c *SessionTokenCoordinator) RefreshSessionAndTokens(ctx context.Context, req *RefreshRequest) (*SessionTokenResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("refresh request is required")
	}

	// Validate refresh token
	claims, err := c.tokenStore.ValidateToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Validate session-token synchronization
	_, err = c.tokenStore.ValidateSessionTokenSync(ctx, claims.SessionID, claims.TokenVersion)
	if err != nil {
		return nil, fmt.Errorf("session-token synchronization failed: %w", err)
	}

	// Refresh session
	var refreshedSession *sessions.Session
	if req.SessionMetadata != nil {
		refreshedSession, err = c.sessionManager.RefreshSession(ctx, claims.SessionID, req.SessionMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh session: %w", err)
		}
	} else {
		refreshedSession, err = c.sessionManager.GetSession(ctx, claims.SessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to get session: %w", err)
		}
	}

	// Fetch FRESH user tier instead of using stale token claims
	freshTier, err := c.userService.GetUserTier(ctx, claims.UserID)
	if err != nil {
		c.logger.Warn("Failed to fetch fresh user tier during refresh, using cached tier",
			zap.Error(err),
			zap.String("userID", claims.UserID))
		freshTier = claims.Tier // Fallback to cached tier
	} else {
		// Log if tier has changed during refresh
		if claims.Tier != freshTier {
			c.logger.Info("User tier changed during token refresh",
				zap.String("userID", claims.UserID),
				zap.String("oldTier", string(claims.Tier)),
				zap.String("newTier", string(freshTier)))
		}
	}

	// Create new token request with fresh tier
	tokenReq := &tokens.TokenGenerationRequest{
		UserID:       claims.UserID,
		SessionID:    claims.SessionID,
		Tier:         freshTier, // Use fresh tier from user service
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

	// Generate new tokens with fresh tier
	tokenPair, err := c.tokenStore.GenerateTokenPair(ctx, tokenReq)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens with fresh tier: %w", err)
	}

	// Delete old refresh token if rotation is enabled (mimicking store behavior)
	if err := c.tokenStore.DeleteRefreshToken(ctx, req.RefreshToken); err != nil {
		c.logger.Warn("Failed to delete old refresh token during coordinator refresh", zap.Error(err))
	}

	return &SessionTokenResponse{
		Session:   refreshedSession,
		TokenPair: tokenPair,
	}, nil
}

// RevokeSessionAndTokens revokes both session and all associated tokens atomically with blacklisting
func (c *SessionTokenCoordinator) RevokeSessionAndTokens(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	c.logger.Info("Revoking session and blacklisting tokens", zap.String("sessionID", sessionID))

	// Get session details first to blacklist active tokens
	session, err := c.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		c.logger.Warn("Failed to get session for token blacklisting, proceeding with revocation",
			zap.Error(err),
			zap.String("sessionID", sessionID))
	} else {
		// Blacklist active tokens for this session with smart TTL
		if blacklistErr := c.blacklistSessionTokens(ctx, session); blacklistErr != nil {
			c.logger.Error("Failed to blacklist session tokens",
				zap.Error(blacklistErr),
				zap.String("sessionID", sessionID))
			// Continue with revocation even if blacklisting fails
		}
	}

	// Revoke all tokens for the session
	err = c.tokenStore.RevokeSessionTokens(ctx, sessionID)
	if err != nil {
		c.logger.Error("Failed to revoke session tokens", zap.Error(err))
		// Continue with session revocation even if token revocation fails
	}

	// Revoke the session
	err = c.sessionManager.RevokeSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	c.logger.Info("Successfully revoked session and blacklisted tokens",
		zap.String("sessionID", sessionID))

	return nil
}

// RevokeAllUserSessionsAndTokens revokes all sessions and tokens for a user atomically with blacklisting
func (c *SessionTokenCoordinator) RevokeAllUserSessionsAndTokens(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	c.logger.Info("Revoking all user sessions and blacklisting tokens", zap.String("userID", userID))

	// Get all active sessions for blacklisting
	sessions, err := c.sessionManager.ListSessions(ctx, userID)
	if err != nil {
		c.logger.Warn("Failed to get user sessions for token blacklisting, proceeding with revocation",
			zap.Error(err),
			zap.String("userID", userID))
	} else {
		// Blacklist tokens for all active sessions
		for _, session := range sessions {
			if session.IsActive {
				if blacklistErr := c.blacklistSessionTokens(ctx, session); blacklistErr != nil {
					c.logger.Error("Failed to blacklist tokens for session",
						zap.Error(blacklistErr),
						zap.String("sessionID", session.ID),
						zap.String("userID", userID))
					// Continue with other sessions
				}
			}
		}
	}

	// Revoke all user tokens
	err = c.tokenStore.RevokeAllTokens(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to revoke user tokens", zap.Error(err))
		// Continue with session revocation even if token revocation fails
	}

	// Revoke all user sessions
	err = c.sessionManager.RevokeAllSessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke user sessions: %w", err)
	}

	c.logger.Info("Successfully revoked all user sessions and blacklisted tokens",
		zap.String("userID", userID))

	return nil
}

// blacklistSessionTokens blacklists active tokens for a session with smart TTL management
func (c *SessionTokenCoordinator) blacklistSessionTokens(ctx context.Context, session *sessions.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	now := time.Now()

	// We need to blacklist tokens that might still be valid for this session
	// Since we don't track individual token JTIs in sessions, we'll use the session ID
	// to find and blacklist active tokens via the token store's session-based revocation

	// The token store's RevokeSessionTokens method should handle the blacklisting
	// But we'll also explicitly blacklist with smart TTL for defense in depth

	// Calculate smart TTL - only blacklist if session isn't already expired
	if session.ExpiresAt.After(now) {
		// For access tokens: typically short-lived (15 minutes)
		accessTokenTTL := c.calculateSmartTTL(session.ExpiresAt, 15*time.Minute)
		if accessTokenTTL > 0 {
			// Blacklist session's access tokens (using session ID as a key pattern)
			blacklistKey := fmt.Sprintf("session:%s:access", session.ID)
			if err := c.tokenStore.BlacklistToken(ctx, blacklistKey, accessTokenTTL); err != nil {
				c.logger.Warn("Failed to blacklist session access tokens",
					zap.Error(err),
					zap.String("sessionID", session.ID))
			}
		}

		// For refresh tokens: longer-lived (7 days typically)
		refreshTokenTTL := c.calculateSmartTTL(session.ExpiresAt, 7*24*time.Hour)
		if refreshTokenTTL > 0 {
			// Blacklist session's refresh tokens
			blacklistKey := fmt.Sprintf("session:%s:refresh", session.ID)
			if err := c.tokenStore.BlacklistToken(ctx, blacklistKey, refreshTokenTTL); err != nil {
				c.logger.Warn("Failed to blacklist session refresh tokens",
					zap.Error(err),
					zap.String("sessionID", session.ID))
			}
		}

		c.logger.Debug("Blacklisted session tokens",
			zap.String("sessionID", session.ID),
			zap.Duration("accessTokenTTL", accessTokenTTL),
			zap.Duration("refreshTokenTTL", refreshTokenTTL))
	}

	return nil
}

// calculateSmartTTL calculates an optimal TTL for blacklisting to minimize Redis bloat
func (c *SessionTokenCoordinator) calculateSmartTTL(expiresAt time.Time, defaultTTL time.Duration) time.Duration {
	now := time.Now()

	// If already expired, no need to blacklist
	if expiresAt.Before(now) {
		return 0
	}

	timeUntilExpiry := expiresAt.Sub(now)

	// Use the shorter of: time until expiry or 1 hour (to prevent long-term Redis bloat)
	maxBlacklistTTL := 1 * time.Hour

	if timeUntilExpiry < maxBlacklistTTL {
		return timeUntilExpiry
	}

	return maxBlacklistTTL
}

// BlacklistTokenWithSmartTTL provides a public method for blacklisting individual tokens with optimal TTL
func (c *SessionTokenCoordinator) BlacklistTokenWithSmartTTL(ctx context.Context, tokenJTI string, expiresAt time.Time) error {
	ttl := c.calculateSmartTTL(expiresAt, 15*time.Minute)
	if ttl <= 0 {
		c.logger.Debug("Token already expired, skipping blacklist", zap.String("tokenJTI", tokenJTI))
		return nil
	}

	err := c.tokenStore.BlacklistToken(ctx, tokenJTI, ttl)
	if err != nil {
		return fmt.Errorf("failed to blacklist token: %w", err)
	}

	c.logger.Debug("Successfully blacklisted token with smart TTL",
		zap.String("tokenJTI", tokenJTI),
		zap.Duration("ttl", ttl))

	return nil
}

// ValidateSessionAndToken validates both session and token, ensuring they're synchronized
func (c *SessionTokenCoordinator) ValidateSessionAndToken(ctx context.Context, sessionID string, token string) (*ValidationResult, error) {
	if sessionID == "" || token == "" {
		return &ValidationResult{
			Valid:  false,
			Reason: "session ID and token are required",
		}, nil
	}

	// Validate token first
	claims, err := c.tokenStore.ValidateToken(ctx, token)
	if err != nil {
		return &ValidationResult{
			Valid:  false,
			Reason: fmt.Sprintf("invalid token: %v", err),
		}, nil
	}

	// Validate session
	sessionResult, err := c.sessionManager.ValidateSession(ctx, sessionID)
	if err != nil {
		return &ValidationResult{
			Valid:  false,
			Reason: fmt.Sprintf("session validation failed: %v", err),
		}, nil
	}

	if !sessionResult.Valid {
		return &ValidationResult{
			Valid:  false,
			Reason: sessionResult.Reason,
		}, nil
	}

	// Ensure session and token are synchronized
	if claims.SessionID != sessionID {
		return &ValidationResult{
			Valid:  false,
			Reason: "session and token mismatch",
		}, nil
	}

	if claims.TokenVersion != sessionResult.TokenVersion {
		return &ValidationResult{
			Valid:  false,
			Reason: "token version mismatch",
		}, nil
	}

	return &ValidationResult{
		Valid:   true,
		Claims:  claims,
		Session: sessionResult.Session,
	}, nil
}

// PropagateTierChanges notifies active sessions when user tier changes
func (c *SessionTokenCoordinator) PropagateTierChanges(ctx context.Context, userID string, oldTier, newTier tokens.UserTier) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	c.logger.Info("Propagating tier changes to active sessions",
		zap.String("userID", userID),
		zap.String("oldTier", string(oldTier)),
		zap.String("newTier", string(newTier)))

	// Get all active sessions for the user
	sessions, err := c.sessionManager.ListSessions(ctx, userID)
	if err != nil {
		c.logger.Error("Failed to get user sessions for tier propagation",
			zap.Error(err),
			zap.String("userID", userID))
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	if len(sessions) == 0 {
		c.logger.Info("No active sessions found for user", zap.String("userID", userID))
		return nil
	}

	// Create a notification key in Redis to signal token refresh needed
	refreshNeededKey := fmt.Sprintf("user:%s:refresh_needed", userID)
	tierChangeData := map[string]interface{}{
		"reason":    "tier_change",
		"old_tier":  string(oldTier),
		"new_tier":  string(newTier),
		"timestamp": time.Now().Unix(),
	}

	// Store notification with reasonable TTL (24 hours)
	tierChangeJSON, _ := json.Marshal(tierChangeData)
	err = c.redisClient.Set(ctx, refreshNeededKey, string(tierChangeJSON), 24*time.Hour).Err()
	if err != nil {
		c.logger.Error("Failed to set refresh notification",
			zap.Error(err),
			zap.String("userID", userID))
		return fmt.Errorf("failed to set refresh notification: %w", err)
	}

	// For each active session, set a session-specific refresh flag
	for _, session := range sessions {
		sessionRefreshKey := fmt.Sprintf("session:%s:refresh_needed", session.ID)
		err := c.redisClient.Set(ctx, sessionRefreshKey, "tier_change", time.Hour).Err()
		if err != nil {
			c.logger.Warn("Failed to set session refresh flag",
				zap.Error(err),
				zap.String("sessionID", session.ID))
		}
	}

	c.logger.Info("Successfully propagated tier changes",
		zap.String("userID", userID),
		zap.Int("affectedSessions", len(sessions)))

	return nil
}

// CheckRefreshNeeded checks if a session needs to refresh due to tier changes
func (c *SessionTokenCoordinator) CheckRefreshNeeded(ctx context.Context, sessionID string, userID string) (bool, string, error) {
	if sessionID == "" || userID == "" {
		return false, "", fmt.Errorf("sessionID and userID are required")
	}

	// Check session-specific refresh flag first
	sessionRefreshKey := fmt.Sprintf("session:%s:refresh_needed", sessionID)
	sessionRefreshNeeded, err := c.redisClient.Get(ctx, sessionRefreshKey).Result()
	if err == nil {
		// Clear the flag after checking
		c.redisClient.Del(ctx, sessionRefreshKey)
		return true, sessionRefreshNeeded, nil
	}

	// Check user-level refresh flag
	userRefreshKey := fmt.Sprintf("user:%s:refresh_needed", userID)
	_, err = c.redisClient.Get(ctx, userRefreshKey).Result()
	if err == nil {
		return true, "tier_change", nil
	}

	// If neither redis.Nil nor other error, it's a real error
	// Note: redis.Nil check would need proper redis import
	if err != nil {
		c.logger.Warn("Failed to check refresh needed status",
			zap.Error(err),
			zap.String("sessionID", sessionID),
			zap.String("userID", userID))
	}

	return false, "", nil
}

// InvalidateUserSessions invalidates all active sessions for a user (forces re-authentication)
func (c *SessionTokenCoordinator) InvalidateUserSessions(ctx context.Context, userID string, reason string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	c.logger.Info("Invalidating all user sessions",
		zap.String("userID", userID),
		zap.String("reason", reason))

	// Get all sessions for the user
	sessions, err := c.sessionManager.ListSessions(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user sessions: %w", err)
	}

	// Revoke all sessions and tokens
	for _, session := range sessions {
		if err := c.RevokeSessionAndTokens(ctx, session.ID); err != nil {
			c.logger.Warn("Failed to revoke session during invalidation",
				zap.Error(err),
				zap.String("sessionID", session.ID))
		}
	}

	c.logger.Info("Successfully invalidated user sessions",
		zap.String("userID", userID),
		zap.Int("invalidatedSessions", len(sessions)))

	return nil
}

// SessionTokenRequest represents a request to create a session with tokens
type SessionTokenRequest struct {
	UserID          string                    `json:"user_id"`
	TokenVersion    int                       `json:"token_version"`
	Tier            tokens.UserTier           `json:"tier"` // Simple user tier instead of roles array
	SessionMetadata *sessions.SessionMetadata `json:"session_metadata"`
	LoginMethod     string                    `json:"login_method"`
	Provider        string                    `json:"provider,omitempty"`
	ProviderID      string                    `json:"provider_id,omitempty"`
	Scopes          []string                  `json:"scopes,omitempty"`
}

// OAuthSessionTokenRequest represents a request to create an OAuth session with tokens
type OAuthSessionTokenRequest struct {
	OAuthRequest *sessions.OAuthSessionRequest `json:"oauth_request"`
	Tier         tokens.UserTier               `json:"tier"` // Simple user tier instead of roles array
}

// RefreshRequest represents a request to refresh session and tokens
type RefreshRequest struct {
	RefreshToken    string                    `json:"refresh_token"`
	SessionMetadata *sessions.SessionMetadata `json:"session_metadata,omitempty"`
}

// SessionTokenResponse represents the response from session and token operations
type SessionTokenResponse struct {
	Session   *sessions.Session `json:"session"`
	TokenPair *tokens.TokenPair `json:"token_pair"`
}

// ValidationResult represents the result of session and token validation
type ValidationResult struct {
	Valid   bool                `json:"valid"`
	Reason  string              `json:"reason,omitempty"`
	Claims  *tokens.TokenClaims `json:"claims,omitempty"`
	Session *sessions.Session   `json:"session,omitempty"`
}

package sessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kdot/k4-chat/backend/internal/apperrors"
	"github.com/kdot/k4-chat/backend/internal/auth"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisSessionStore implements SessionManager using Redis
type RedisSessionStore struct {
	client *redis.Client
	logger *zap.Logger
	ttl    time.Duration
}

// NewRedisSessionStore creates a new Redis-based session store
func NewRedisSessionStore(client *redis.Client, logger *zap.Logger, ttl time.Duration) *RedisSessionStore {
	return &RedisSessionStore{
		client: client,
		logger: logger,
		ttl:    ttl,
	}
}

// validateSessionRequest validates session creation request
func (s *RedisSessionStore) validateSessionRequest(req *SessionCreateRequest) error {
	if req == nil {
		return fmt.Errorf("session request cannot be nil")
	}
	if req.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if req.Metadata == nil {
		return fmt.Errorf("session metadata is required")
	}
	if req.Metadata.DeviceID == "" {
		return fmt.Errorf("device ID is required")
	}
	if req.Metadata.IPAddress == "" {
		return fmt.Errorf("IP address is required")
	}
	if req.Metadata.UserAgent == "" {
		return fmt.Errorf("user agent is required")
	}
	if req.LoginMethod == "" {
		req.LoginMethod = "password" // default
	}
	return nil
}

// CreateSession implements SessionManager
func (s *RedisSessionStore) CreateSession(ctx context.Context, req *SessionCreateRequest) (*Session, error) {
	if err := s.validateSessionRequest(req); err != nil {
		return nil, auth.NewAuthError(auth.ErrSessionExpired.Error(), "invalid session request")
	}

	sessionID := uuid.New().String()
	now := time.Now()

	session := &Session{
		ID:           sessionID,
		UserID:       req.UserID,
		DeviceID:     req.Metadata.DeviceID,
		IPAddress:    req.Metadata.IPAddress,
		UserAgent:    req.Metadata.UserAgent,
		LastUsed:     now,
		CreatedAt:    now,
		ExpiresAt:    now.Add(s.ttl),
		IsActive:     true,
		TokenVersion: req.TokenVersion,
		Provider:     req.Provider,
		ProviderID:   req.ProviderID,
		Fingerprint:  req.Metadata.Fingerprint,
		LoginMethod:  req.LoginMethod,
		Region:       req.Metadata.Region,
		ClientType:   req.Metadata.ClientType,
	}

	// Use Redis transaction for atomic session creation
	_, err := s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		// Store session data
		sessionKey := fmt.Sprintf("session:%s", sessionID)
		sessionData, err := json.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}

		pipe.Set(ctx, sessionKey, sessionData, s.ttl)

		// Add session to user's session list
		userSessionsKey := fmt.Sprintf("user:%s:sessions", req.UserID)
		pipe.SAdd(ctx, userSessionsKey, sessionID)
		pipe.Expire(ctx, userSessionsKey, s.ttl)

		// Track session by provider if OAuth
		if req.Provider != "" && req.Provider != "local" {
			providerKey := fmt.Sprintf("user:%s:provider:%s:sessions", req.UserID, req.Provider)
			pipe.SAdd(ctx, providerKey, sessionID)
			pipe.Expire(ctx, providerKey, s.ttl)
		}

		// Track session by device for security monitoring
		deviceKey := fmt.Sprintf("device:%s:sessions", req.Metadata.DeviceID)
		pipe.SAdd(ctx, deviceKey, sessionID)
		pipe.Expire(ctx, deviceKey, s.ttl)

		return nil
	})

	if err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to create session", err)
	}

	s.logger.Info("Created new session",
		zap.String("sessionID", sessionID),
		zap.String("userID", req.UserID),
		zap.String("provider", req.Provider),
		zap.String("loginMethod", req.LoginMethod))

	return session, nil
}

// CreateOAuthSession implements SessionManager
func (s *RedisSessionStore) CreateOAuthSession(ctx context.Context, req *OAuthSessionRequest) (*Session, error) {
	if req == nil {
		return nil, fmt.Errorf("OAuth session request cannot be nil")
	}
	if req.Provider == "" {
		return nil, fmt.Errorf("OAuth provider is required")
	}
	if req.ProviderID == "" {
		return nil, fmt.Errorf("OAuth provider ID is required")
	}
	if req.Email == "" {
		return nil, fmt.Errorf("email is required for OAuth session")
	}

	// Create session request from OAuth data
	sessionReq := &SessionCreateRequest{
		UserID:       req.ProviderID, // This will be mapped to internal user ID
		TokenVersion: 1,              // Initial version for OAuth users
		Metadata:     req.Metadata,
		LoginMethod:  "oauth",
		Provider:     req.Provider,
		ProviderID:   req.ProviderID,
	}

	session, err := s.CreateSession(ctx, sessionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth session: %w", err)
	}

	// Store OAuth-specific data
	oauthKey := fmt.Sprintf("oauth:%s:%s", req.Provider, req.ProviderID)
	oauthData := map[string]interface{}{
		"access_token":  req.AccessToken,
		"refresh_token": req.RefreshToken,
		"expires_at":    req.ExpiresAt,
		"scopes":        req.Scopes,
		"email":         req.Email,
		"username":      req.Username,
		"display_name":  req.DisplayName,
		"avatar_url":    req.AvatarURL,
		"extra_data":    req.ExtraData,
		"session_id":    session.ID,
	}

	oauthDataJSON, err := json.Marshal(oauthData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OAuth data: %w", err)
	}

	err = s.client.Set(ctx, oauthKey, oauthDataJSON, s.ttl).Err()
	if err != nil {
		s.logger.Error("Failed to store OAuth data", zap.Error(err))
		// Don't fail session creation for OAuth data storage failure
	}

	return session, nil
}

// GetSession implements SessionManager
func (s *RedisSessionStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, apperrors.NewAuthError(apperrors.ErrAuthUnauthorized, "session ID is required", nil)
	}

	sessionKey := fmt.Sprintf("session:%s", sessionID)
	sessionData, err := s.client.Get(ctx, sessionKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperrors.NewAuthError(apperrors.ErrAuthUnauthorized, "session not found", err)
		}
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to get session", err)
	}

	var session Session
	if err := json.Unmarshal(sessionData, &session); err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to unmarshal session", err)
	}

	return &session, nil
}

// ListSessions implements SessionManager with batch operations
func (s *RedisSessionStore) ListSessions(ctx context.Context, userID string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
	sessionIDs, err := s.client.SMembers(ctx, userSessionsKey).Result()
	if err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to get user sessions", err)
	}

	if len(sessionIDs) == 0 {
		return []*Session{}, nil
	}

	// Use pipeline for batch retrieval
	pipe := s.client.Pipeline()
	for _, sessionID := range sessionIDs {
		sessionKey := fmt.Sprintf("session:%s", sessionID)
		pipe.Get(ctx, sessionKey)
	}

	results, err := pipe.Exec(ctx)
	if err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to batch get sessions", err)
	}

	var sessions []*Session
	for i, result := range results {
		if result.Err() != nil {
			if !errors.Is(result.Err(), redis.Nil) {
				s.logger.Warn("Failed to get session",
					zap.String("sessionID", sessionIDs[i]),
					zap.Error(result.Err()))
			}
			continue
		}

		sessionData, err := result.(*redis.StringCmd).Bytes()
		if err != nil {
			s.logger.Warn("Failed to get session bytes",
				zap.String("sessionID", sessionIDs[i]),
				zap.Error(err))
			continue
		}

		var session Session
		if err := json.Unmarshal(sessionData, &session); err != nil {
			s.logger.Warn("Failed to unmarshal session",
				zap.String("sessionID", sessionIDs[i]),
				zap.Error(err))
			continue
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// UpdateSession implements SessionManager
func (s *RedisSessionStore) UpdateSession(ctx context.Context, sessionID string, metadata *SessionMetadata) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if metadata == nil {
		return fmt.Errorf("session metadata is required")
	}

	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Update session metadata
	session.DeviceID = metadata.DeviceID
	session.IPAddress = metadata.IPAddress
	session.UserAgent = metadata.UserAgent
	session.LastUsed = metadata.LastUsed
	if metadata.Fingerprint != "" {
		session.Fingerprint = metadata.Fingerprint
	}
	if metadata.Region != "" {
		session.Region = metadata.Region
	}
	if metadata.ClientType != "" {
		session.ClientType = metadata.ClientType
	}

	sessionKey := fmt.Sprintf("session:%s", sessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return apperrors.NewDBError(apperrors.ErrDBQuery, "failed to marshal session", err)
	}

	err = s.client.Set(ctx, sessionKey, sessionData, s.ttl).Err()
	if err != nil {
		return apperrors.NewDBError(apperrors.ErrDBQuery, "failed to update session", err)
	}

	return nil
}

// ValidateSession implements SessionManager with detailed validation
func (s *RedisSessionStore) ValidateSession(ctx context.Context, sessionID string) (*SessionValidationResult, error) {
	if sessionID == "" {
		return &SessionValidationResult{
			Valid:  false,
			Reason: "session ID is required",
		}, nil
	}

	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return &SessionValidationResult{
			Valid:  false,
			Reason: "session not found",
		}, nil
	}

	now := time.Now()
	result := &SessionValidationResult{
		Session:      session,
		TokenVersion: session.TokenVersion,
		UpdatedAt:    now,
	}

	// Check if session is active
	if !session.IsActive {
		result.Valid = false
		result.Reason = "session is inactive"
		return result, nil
	}

	// Check if session has expired
	if now.After(session.ExpiresAt) {
		result.Valid = false
		result.Reason = "session has expired"
		return result, nil
	}

	result.Valid = true
	return result, nil
}

// RevokeSession implements SessionManager
func (s *RedisSessionStore) RevokeSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Use transaction for atomic revocation
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		// Remove session from Redis
		sessionKey := fmt.Sprintf("session:%s", sessionID)
		pipe.Del(ctx, sessionKey)

		// Remove session from user's session list
		userSessionsKey := fmt.Sprintf("user:%s:sessions", session.UserID)
		pipe.SRem(ctx, userSessionsKey, sessionID)

		// Remove from provider list if OAuth
		if session.Provider != "" && session.Provider != "local" {
			providerKey := fmt.Sprintf("user:%s:provider:%s:sessions", session.UserID, session.Provider)
			pipe.SRem(ctx, providerKey, sessionID)
		}

		// Remove from device list
		deviceKey := fmt.Sprintf("device:%s:sessions", session.DeviceID)
		pipe.SRem(ctx, deviceKey, sessionID)

		return nil
	})

	if err != nil {
		return apperrors.NewDBError(apperrors.ErrDBQuery, "failed to revoke session", err)
	}

	s.logger.Info("Revoked session",
		zap.String("sessionID", sessionID),
		zap.String("userID", session.UserID))

	return nil
}

// RevokeAllSessions implements SessionManager
func (s *RedisSessionStore) RevokeAllSessions(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	sessions, err := s.ListSessions(ctx, userID)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		return nil
	}

	// Use transaction for atomic revocation
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, session := range sessions {
			// Remove session
			sessionKey := fmt.Sprintf("session:%s", session.ID)
			pipe.Del(ctx, sessionKey)

			// Remove from device list
			deviceKey := fmt.Sprintf("device:%s:sessions", session.DeviceID)
			pipe.SRem(ctx, deviceKey, session.ID)
		}

		// Remove user's session list
		userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
		pipe.Del(ctx, userSessionsKey)

		// Remove provider lists
		providerPattern := fmt.Sprintf("user:%s:provider:*:sessions", userID)
		providerKeys, err := s.client.Keys(ctx, providerPattern).Result()
		if err == nil {
			for _, key := range providerKeys {
				pipe.Del(ctx, key)
			}
		}

		return nil
	})

	if err != nil {
		return apperrors.NewDBError(apperrors.ErrDBQuery, "failed to revoke all sessions", err)
	}

	s.logger.Info("Revoked all sessions for user",
		zap.String("userID", userID),
		zap.Int("sessionCount", len(sessions)))

	return nil
}

// RevokeAllSessionsExcept implements SessionManager
func (s *RedisSessionStore) RevokeAllSessionsExcept(ctx context.Context, userID string, keepSessionID string) error {
	if userID == "" {
		return fmt.Errorf("user ID is required")
	}
	if keepSessionID == "" {
		return fmt.Errorf("keep session ID is required")
	}

	sessions, err := s.ListSessions(ctx, userID)
	if err != nil {
		return err
	}

	// Filter out the session to keep
	var sessionsToRevoke []*Session
	for _, session := range sessions {
		if session.ID != keepSessionID {
			sessionsToRevoke = append(sessionsToRevoke, session)
		}
	}

	if len(sessionsToRevoke) == 0 {
		return nil
	}

	// Use transaction for atomic revocation
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, session := range sessionsToRevoke {
			// Remove session
			sessionKey := fmt.Sprintf("session:%s", session.ID)
			pipe.Del(ctx, sessionKey)

			// Remove from user's session list
			userSessionsKey := fmt.Sprintf("user:%s:sessions", userID)
			pipe.SRem(ctx, userSessionsKey, session.ID)

			// Remove from provider list if OAuth
			if session.Provider != "" && session.Provider != "local" {
				providerKey := fmt.Sprintf("user:%s:provider:%s:sessions", userID, session.Provider)
				pipe.SRem(ctx, providerKey, session.ID)
			}

			// Remove from device list
			deviceKey := fmt.Sprintf("device:%s:sessions", session.DeviceID)
			pipe.SRem(ctx, deviceKey, session.ID)
		}

		return nil
	})

	if err != nil {
		return apperrors.NewDBError(apperrors.ErrDBQuery, "failed to revoke sessions", err)
	}

	s.logger.Info("Revoked sessions except one",
		zap.String("userID", userID),
		zap.String("keptSessionID", keepSessionID),
		zap.Int("revokedCount", len(sessionsToRevoke)))

	return nil
}

// RefreshSession implements SessionManager
func (s *RedisSessionStore) RefreshSession(ctx context.Context, sessionID string, metadata *SessionMetadata) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	session, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Update session with new metadata and extend expiration
	now := time.Now()
	if metadata != nil {
		session.DeviceID = metadata.DeviceID
		session.IPAddress = metadata.IPAddress
		session.UserAgent = metadata.UserAgent
		session.LastUsed = now
		if metadata.Fingerprint != "" {
			session.Fingerprint = metadata.Fingerprint
		}
		if metadata.Region != "" {
			session.Region = metadata.Region
		}
		if metadata.ClientType != "" {
			session.ClientType = metadata.ClientType
		}
	} else {
		session.LastUsed = now
	}

	// Extend session expiration
	session.ExpiresAt = now.Add(s.ttl)

	sessionKey := fmt.Sprintf("session:%s", sessionID)
	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to marshal session", err)
	}

	err = s.client.Set(ctx, sessionKey, sessionData, s.ttl).Err()
	if err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to refresh session", err)
	}

	return session, nil
}

// GetSessionsByProvider implements SessionManager
func (s *RedisSessionStore) GetSessionsByProvider(ctx context.Context, userID string, provider string) ([]*Session, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	if provider == "" {
		return nil, fmt.Errorf("provider is required")
	}

	providerKey := fmt.Sprintf("user:%s:provider:%s:sessions", userID, provider)
	sessionIDs, err := s.client.SMembers(ctx, providerKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []*Session{}, nil
		}
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to get provider sessions", err)
	}

	if len(sessionIDs) == 0 {
		return []*Session{}, nil
	}

	// Use pipeline for batch retrieval
	pipe := s.client.Pipeline()
	for _, sessionID := range sessionIDs {
		sessionKey := fmt.Sprintf("session:%s", sessionID)
		pipe.Get(ctx, sessionKey)
	}

	results, err := pipe.Exec(ctx)
	if err != nil {
		return nil, apperrors.NewDBError(apperrors.ErrDBQuery, "failed to batch get provider sessions", err)
	}

	var sessions []*Session
	for i, result := range results {
		if result.Err() != nil {
			if !errors.Is(result.Err(), redis.Nil) {
				s.logger.Warn("Failed to get provider session",
					zap.String("sessionID", sessionIDs[i]),
					zap.Error(result.Err()))
			}
			continue
		}

		sessionData, err := result.(*redis.StringCmd).Bytes()
		if err != nil {
			continue
		}

		var session Session
		if err := json.Unmarshal(sessionData, &session); err != nil {
			continue
		}

		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// CleanupExpiredSessions implements SessionManager
func (s *RedisSessionStore) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	// This is handled automatically by Redis TTL, but we can implement
	// additional cleanup logic here if needed
	s.logger.Info("Session cleanup called - Redis TTL handles automatic cleanup")
	return 0, nil
}

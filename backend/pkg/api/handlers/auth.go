package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kdot/k4-chat/backend/configs"
	"github.com/kdot/k4-chat/backend/internal/auth"
	"github.com/kdot/k4-chat/backend/internal/auth/coordinator"
	"github.com/kdot/k4-chat/backend/internal/auth/sessions"
	"github.com/kdot/k4-chat/backend/internal/auth/tokens"
	"github.com/kdot/k4-chat/backend/internal/database/models"
)

const (
	// Default timeout values if not configured
	DefaultSessionTimeout = 10 * time.Second
	DefaultTokenTimeout   = 5 * time.Second
	DefaultLogoutTimeout  = 5 * time.Second
)

/*
File Next Steps:
- move coordinator and redis to server. Reuse same DI'd client
- rework user tier logic fine for now, need to get MVP
- add oauth support
- password reset
- failing DRY principles, need to refactor clean up
- move utils to shorten file length, other files will likely use them too. pkg/utils?
- add metrics
- CACHE CAHCE CACHE, rate limit too
*/

/*
AuthHandlerInterface defines the methods that the AuthHandler is required to implement
All methods leverage the middleware for rate limiting, CORS, and security headers
*/
type AuthHandlerInterface interface {
	SignUp(w http.ResponseWriter, r *http.Request)
	SignIn(w http.ResponseWriter, r *http.Request)
	SignOut(w http.ResponseWriter, r *http.Request)
	SignOutAllDevices(w http.ResponseWriter, r *http.Request)
	RevokeSpecificSession(w http.ResponseWriter, r *http.Request)
	RefreshTokens(w http.ResponseWriter, r *http.Request)
	GetActiveSessions(w http.ResponseWriter, r *http.Request)
}

// AuthHandler handles authentication endpoints with professional error handling,
// audit logging, and integration with session-token coordinator
type AuthHandler struct {
	service     auth.Service
	coordinator *coordinator.SessionTokenCoordinator
	config      *configs.AuthHandlerConfig
	auditConfig *configs.AuditConfig
	logger      *zap.Logger
	validator   *validator.Validate
	redis       *redis.Client
}

// NewAuthHandler creates a new professional auth handler with all dependencies
func NewAuthHandler(
	service auth.Service,
	coordinator *coordinator.SessionTokenCoordinator,
	config *configs.HandlersConfig,
	logger *zap.Logger,
	redis *redis.Client,
) AuthHandlerInterface {
	return &AuthHandler{
		service:     service,
		coordinator: coordinator,
		config:      &config.Auth,
		auditConfig: &config.Audit,
		logger:      logger,
		validator:   validator.New(),
		redis:       redis,
	}
}

// AuditEvent represents an audit log event
type AuditEvent struct {
	Action    string                 `json:"action"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// auditLog records security-relevant events
func (h *AuthHandler) auditLog(event AuditEvent) {
	if !h.auditConfig.Enabled {
		return
	}

	// Only log if configured to log this type of event
	if !event.Success && !h.auditConfig.LogFailures {
		return
	}
	if event.Success && !h.auditConfig.LogSuccessful {
		return
	}

	// filter sensitive fields
	if h.auditConfig.IncludeDetails && event.Details != nil {
		for _, field := range h.auditConfig.SensitiveFields {
			delete(event.Details, field)
		}
	} else if !h.auditConfig.IncludeDetails {
		event.Details = nil
	}

	h.logger.Info("AUTH_AUDIT",
		zap.String("action", event.Action),
		zap.String("user_id", event.UserID),
		zap.String("session_id", event.SessionID),
		zap.String("ip_address", event.IPAddress),
		zap.String("user_agent", event.UserAgent),
		zap.Bool("success", event.Success),
		zap.String("error", event.Error),
		zap.Time("timestamp", event.Timestamp),
		zap.Any("details", event.Details),
	)
}

// parseAndValidateRequest safely parses and validates JSON request with size limits
func (h *AuthHandler) parseAndValidateRequest(r *http.Request, dest interface{}) error {
	// Use configured max body size or default from main config
	maxSize := int64(1 << 20) // 1MB default
	if cfg := configs.GetConfig(); cfg != nil {
		maxSize = int64(cfg.Handlers.Request.MaxRequestBodySize)
	}

	r.Body = http.MaxBytesReader(nil, r.Body, maxSize)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate struct tags
	if err := h.validator.Struct(dest); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// convertUserToDTO safely converts User model to DTO (no sensitive data)
func (h *AuthHandler) convertUserToDTO(user *models.User) auth.UserDTO {
	return auth.UserDTO{
		ID:           user.ID.String(),
		Email:        user.Email,
		Username:     user.Username,
		DisplayName:  user.DisplayName,
		AvatarURL:    user.AvatarURL,
		CreatedAt:    user.CreatedAt,
		LastActiveAt: user.LastActiveAt,
		IsActive:     user.IsActive,
	}
}

// getClientInfo extracts client information for security logging and fingerprinting
func (h *AuthHandler) getClientInfo(r *http.Request) (ip, userAgent string) {
	// Get real IP (considering proxies, load balancers)
	ip = r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
		// Get first IP if comma-separated list
		if ip != "" {
			ips := strings.Split(ip, ",")
			ip = strings.TrimSpace(ips[0])
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	userAgent = r.Header.Get("User-Agent")
	return ip, userAgent
}

// createSessionMetadata creates session metadata from request
func (h *AuthHandler) createSessionMetadata(r *http.Request) *sessions.SessionMetadata {
	ip, userAgent := h.getClientInfo(r)

	return &sessions.SessionMetadata{
		IPAddress:   ip,
		UserAgent:   userAgent,
		LastUsed:    time.Now(),
		CreatedAt:   time.Now(),
		Fingerprint: tokens.GenerateFingerprint(r),
		DeviceID:    tokens.GenerateDeviceID(userAgent, ip),
		Region:      r.Header.Get(""), // TODO: need to figure out how to grab region
		ClientType:  h.detectClientType(userAgent),
	}
}

// detectClientType attempts to detect client type from user agent
func (h *AuthHandler) detectClientType(userAgent string) string {
	userAgent = strings.ToLower(userAgent)

	if strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") || strings.Contains(userAgent, "iphone") {
		return "mobile"
	}
	if strings.Contains(userAgent, "postman") || strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "api") {
		return "api"
	}
	return "web"
}

// convertStringToUserTier converts a string tier to tokens.UserTier enum
func (h *AuthHandler) convertStringToUserTier(tierStr string) tokens.UserTier {
	switch tierStr {
	case "premium":
		return tokens.TierPremium
	case "admin":
		return tokens.TierAdmin
	case "moderator":
		return tokens.TierModerator
	default:
		return tokens.TierFree
	}
}

// getUserTierWithFallback retrieves user tier with Redis cache and database fallback
func (h *AuthHandler) getUserTierWithFallback(ctx context.Context, userID string) (tokens.UserTier, bool, error) {
	userTierStr, fromCache, err := GetUserTierFromRedisWithFallback(ctx, h.redis, userID, nil)
	if err != nil {
		h.logger.Error("Failed to get user tier",
			zap.String("user_id", userID),
			zap.Error(err))
		// Use default tier as fallback
		// TODO: honestly this is temporary probably should not be treating this like a free tier, also
		// Dont really think handler needs access, shouldn't be storing user tier client side, let actors control limits etc
		userTierStr = "free"
	}

	userTier := h.convertStringToUserTier(userTierStr)

	h.logger.Debug("User tier retrieved",
		zap.String("user_id", userID),
		zap.String("tier", userTierStr),
		zap.Bool("from_cache", fromCache))

	return userTier, fromCache, err
}

// SignUp handles user registration with session and token creation
func (h *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()
	ip, userAgent := h.getClientInfo(r)

	// Create timeout context for this operation
	timeout := DefaultSessionTimeout
	if h.config != nil {
		timeout = h.config.SessionCreateTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Parse and validate request
	var req auth.SignUpRequest
	if err := h.parseAndValidateRequest(r, &req); err != nil {
		h.logger.Warn("Invalid signup request",
			zap.String("ip", ip),
			zap.String("user_agent", userAgent),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signup_attempt",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     "invalid_request",
			Timestamp: time.Now(),
			Details:   map[string]interface{}{"error": "validation_failed"},
		})

		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request", map[string]string{
			"details": "Request validation failed",
		})
		return
	}

	// Validate request structure
	if err := h.validateSignUpRequest(&req); err != nil {
		h.auditLog(AuditEvent{
			Action:    "signup_attempt",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     "invalid_structure",
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Process signup based on type
	var user *models.User
	var err error

	switch req.RequestType {
	case "email":
		if req.Email == nil {
			WriteErrorResponse(w, http.StatusBadRequest, "Missing email signup data", nil)
			return
		}
		user, err = h.service.SignUpWithEmail(timeoutCtx, req)

	case "oauth":
		if req.OAuth == nil {
			WriteErrorResponse(w, http.StatusBadRequest, "Missing OAuth signup data", nil)
			return
		}
		user, err = h.service.SignUpWithOAuth(timeoutCtx, req)

	default:
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request type", nil)
		return
	}

	// Handle service errors
	if err != nil {
		h.logger.Error("Signup failed",
			zap.String("type", req.RequestType),
			zap.String("ip", ip),
			zap.Duration("duration", time.Since(startTime)),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signup_attempt",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
			Details:   map[string]interface{}{"type": req.RequestType},
		})

		// Map domain errors to HTTP status codes
		status := h.mapErrorToHTTPStatus(err)
		message := "Signup failed"

		// Provide more specific error messages if detailed errors are enabled
		if h.config.EnableDetailedErrors {
			message = err.Error()
		}

		WriteErrorResponse(w, status, message, nil)
		return
	}

	// Success - create session and tokens using coordinator
	userDTO := h.convertUserToDTO(user)
	sessionMetadata := h.createSessionMetadata(r)

	// Get user tier with Redis cache and database fallback
	userTier, _, err := h.getUserTierWithFallback(ctx, user.ID.String())
	if err != nil {
		h.logger.Error("Failed to get user tier",
			zap.String("user_id", user.ID.String()),
			zap.Error(err))
		// Use default tier as fallback for new users
		userTier = tokens.TierFree
	}

	// Create session with tokens atomically
	coordinatorReq := &coordinator.SessionTokenRequest{
		UserID:          user.ID.String(),
		TokenVersion:    1, // New user starts with version 1
		Tier:            userTier,
		SessionMetadata: sessionMetadata,
		LoginMethod:     req.RequestType,
	}

	// Handle OAuth-specific fields
	if req.RequestType == "oauth" && req.OAuth != nil {
		coordinatorReq.Provider = req.OAuth.Provider
		coordinatorReq.Scopes = []string{} // OAuth scopes would be populated here
	}

	// Create session and tokens
	sessionTokenResponse, err := h.coordinator.CreateSessionWithTokens(timeoutCtx, coordinatorReq)
	if err != nil {
		h.logger.Error("Failed to create session and tokens",
			zap.String("user_id", user.ID.String()),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signup_session_failed",
			UserID:    user.ID.String(),
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to complete registration", nil)
		return
	}

	// Build success response
	response := auth.AuthResponse{
		User:         userDTO,
		AccessToken:  sessionTokenResponse.TokenPair.AccessToken,
		RefreshToken: sessionTokenResponse.TokenPair.RefreshToken,
		ExpiresIn:    sessionTokenResponse.TokenPair.ExpiresIn,
		TokenType:    sessionTokenResponse.TokenPair.TokenType,
	}

	h.logger.Info("User signup successful",
		zap.String("user_id", userDTO.ID),
		zap.String("username", userDTO.Username),
		zap.String("session_id", sessionTokenResponse.Session.ID),
		zap.String("ip", ip),
		zap.Duration("duration", time.Since(startTime)))

	h.auditLog(AuditEvent{
		Action:    "signup_success",
		UserID:    user.ID.String(),
		SessionID: sessionTokenResponse.Session.ID,
		IPAddress: ip,
		UserAgent: userAgent,
		Success:   true,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"type": req.RequestType},
	})

	WriteJSONResponse(w, http.StatusCreated, response)
}

// validateSignUpRequest performs additional business logic validation
func (h *AuthHandler) validateSignUpRequest(req *auth.SignUpRequest) error {
	switch req.RequestType {
	case "email":
		if req.Email == nil {
			return fmt.Errorf("email signup data is required")
		}
	case "oauth":
		if req.OAuth == nil {
			return fmt.Errorf("OAuth signup data is required")
		}
	default:
		return fmt.Errorf("invalid request type: %s", req.RequestType)
	}
	return nil
}

// SignIn handles user authentication with session and token creation
func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()
	ip, userAgent := h.getClientInfo(r)

	// Create timeout context
	timeout := DefaultSessionTimeout
	if h.config != nil {
		timeout = h.config.SessionCreateTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Parse and validate request
	var req auth.SignInRequest
	if err := h.parseAndValidateRequest(r, &req); err != nil {
		h.logger.Warn("Invalid signin request",
			zap.String("ip", ip),
			zap.String("user_agent", userAgent),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signin_attempt",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     "invalid_request",
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request", nil)
		return
	}

	// Process signin based on type
	var user *models.User
	var err error

	switch req.RequestType {
	case "email":
		if req.Email == nil {
			WriteErrorResponse(w, http.StatusBadRequest, "Missing email signin data", nil)
			return
		}
		user, err = h.service.SignInWithEmail(timeoutCtx, req)

	case "oauth":
		if req.OAuth == nil {
			WriteErrorResponse(w, http.StatusBadRequest, "Missing OAuth signin data", nil)
			return
		}
		user, err = h.service.SignInWithOAuth(timeoutCtx, req)

	default:
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request type", nil)
		return
	}

	// Handle authentication errors
	if err != nil {
		h.logger.Warn("Signin failed",
			zap.String("type", req.RequestType),
			zap.String("ip", ip),
			zap.Duration("duration", time.Since(startTime)),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signin_attempt",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
			Details:   map[string]interface{}{"type": req.RequestType},
		})

		// Map domain errors to HTTP status codes
		status := h.mapErrorToHTTPStatus(err)
		WriteErrorResponse(w, status, "Authentication failed", nil)
		return
	}

	// Success - create session and tokens
	userDTO := h.convertUserToDTO(user)
	sessionMetadata := h.createSessionMetadata(r)

	// Get user tier with Redis cache and database fallback
	userTier, _, err := h.getUserTierWithFallback(ctx, user.ID.String())
	if err != nil {
		h.logger.Error("Failed to get user tier",
			zap.String("user_id", user.ID.String()),
			zap.Error(err))
		// Use default tier as fallback
		userTier = tokens.TierFree
	}

	// Determine token version (increment if refresh token rotation enabled)
	tokenVersion := 1 // This would typically come from user record

	coordinatorReq := &coordinator.SessionTokenRequest{
		UserID:          user.ID.String(),
		TokenVersion:    tokenVersion,
		Tier:            userTier,
		SessionMetadata: sessionMetadata,
		LoginMethod:     req.RequestType,
	}

	// Handle OAuth-specific fields
	if req.RequestType == "oauth" && req.OAuth != nil {
		coordinatorReq.Provider = req.OAuth.Provider
	}

	// Create session and tokens atomically
	sessionTokenResponse, err := h.coordinator.CreateSessionWithTokens(timeoutCtx, coordinatorReq)
	if err != nil {
		h.logger.Error("Failed to create session and tokens",
			zap.String("user_id", user.ID.String()),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signin_session_failed",
			UserID:    user.ID.String(),
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to complete authentication", nil)
		return
	}

	// Build success response
	response := auth.AuthResponse{
		User:         userDTO,
		AccessToken:  sessionTokenResponse.TokenPair.AccessToken,
		RefreshToken: sessionTokenResponse.TokenPair.RefreshToken,
		ExpiresIn:    sessionTokenResponse.TokenPair.ExpiresIn,
		TokenType:    sessionTokenResponse.TokenPair.TokenType,
	}

	h.logger.Info("User signin successful",
		zap.String("user_id", userDTO.ID),
		zap.String("username", userDTO.Username),
		zap.String("session_id", sessionTokenResponse.Session.ID),
		zap.String("ip", ip),
		zap.Duration("duration", time.Since(startTime)))

	h.auditLog(AuditEvent{
		Action:    "signin_success",
		UserID:    user.ID.String(),
		SessionID: sessionTokenResponse.Session.ID,
		IPAddress: ip,
		UserAgent: userAgent,
		Success:   true,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"type": req.RequestType},
	})

	WriteJSONResponse(w, http.StatusOK, response)
}

// SignOut invalidates the current session (requires authentication)
func (h *AuthHandler) SignOut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()
	ip, userAgent := h.getClientInfo(r)

	// Get session from middleware context
	session, ok := GetSession(ctx)
	if !ok {
		h.logger.Error("Session not found in context during signout")
		WriteErrorResponse(w, http.StatusUnauthorized, "Session not found", nil)
		return
	}

	// Get user ID from context
	userID, ok := GetUserID(ctx)
	if !ok {
		h.logger.Error("User ID not found in context during signout")
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found", nil)
		return
	}

	// Create timeout context
	timeout := DefaultLogoutTimeout
	if h.config != nil {
		timeout = h.config.LogoutTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Revoke session and tokens atomically
	err := h.coordinator.RevokeSessionAndTokens(timeoutCtx, session.ID)
	if err != nil {
		h.logger.Error("Failed to revoke session and tokens",
			zap.String("user_id", userID),
			zap.String("session_id", session.ID),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signout_failed",
			UserID:    userID,
			SessionID: session.ID,
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to sign out", nil)
		return
	}

	h.logger.Info("User signed out successfully",
		zap.String("user_id", userID),
		zap.String("session_id", session.ID),
		zap.String("ip", ip),
		zap.Duration("duration", time.Since(startTime)))

	h.auditLog(AuditEvent{
		Action:    "signout_success",
		UserID:    userID,
		SessionID: session.ID,
		IPAddress: ip,
		UserAgent: userAgent,
		Success:   true,
		Timestamp: time.Now(),
	})

	WriteJSONResponse(w, http.StatusOK, map[string]string{
		"message": "Successfully signed out",
	})
}

// SignOutAllDevices invalidates all sessions for the user (requires authentication)
func (h *AuthHandler) SignOutAllDevices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip, userAgent := h.getClientInfo(r)

	// Get user ID from context
	userID, ok := GetUserID(ctx)
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found", nil)
		return
	}

	// Get current session ID
	sessionID := ""
	if session, ok := GetSession(ctx); ok {
		sessionID = session.ID
	}

	// Create timeout context
	timeout := DefaultLogoutTimeout
	if h.config != nil {
		timeout = h.config.LogoutTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Revoke all user sessions and tokens
	err := h.coordinator.RevokeAllUserSessionsAndTokens(timeoutCtx, userID)
	if err != nil {
		h.logger.Error("Failed to revoke all user sessions",
			zap.String("user_id", userID),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "signout_all_failed",
			UserID:    userID,
			SessionID: sessionID,
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to sign out all devices", nil)
		return
	}
	h.auditLog(AuditEvent{
		Action:    "signout_all_success",
		UserID:    userID,
		SessionID: sessionID,
		IPAddress: ip,
		UserAgent: userAgent,
	})
}

// RefreshTokens generates new tokens using refresh token (public endpoint)
func (h *AuthHandler) RefreshTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()
	ip, userAgent := h.getClientInfo(r)

	// Parse refresh token request
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := h.parseAndValidateRequest(r, &req); err != nil {
		h.logger.Warn("Invalid refresh token request",
			zap.String("ip", ip),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "token_refresh_attempt",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     "invalid_request",
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request", nil)
		return
	}

	// Create timeout context
	timeout := DefaultTokenTimeout
	if h.config != nil {
		timeout = h.config.TokenRefreshTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create session metadata for the refresh
	sessionMetadata := h.createSessionMetadata(r)

	// Refresh session and tokens atomically
	refreshReq := &coordinator.RefreshRequest{
		RefreshToken:    req.RefreshToken,
		SessionMetadata: sessionMetadata,
	}

	response, err := h.coordinator.RefreshSessionAndTokens(timeoutCtx, refreshReq)
	if err != nil {
		h.logger.Warn("Token refresh failed",
			zap.String("ip", ip),
			zap.Duration("duration", time.Since(startTime)),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "token_refresh_failed",
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		// Map token errors to appropriate status codes
		status := http.StatusUnauthorized
		message := "Token refresh failed"

		if strings.Contains(err.Error(), "expired") {
			message = "Refresh token expired"
		} else if strings.Contains(err.Error(), "invalid") {
			message = "Invalid refresh token"
		}

		WriteErrorResponse(w, status, message, nil)
		return
	}

	// Build success response
	tokenResponse := auth.AuthResponse{
		AccessToken:  response.TokenPair.AccessToken,
		RefreshToken: response.TokenPair.RefreshToken,
		ExpiresIn:    response.TokenPair.ExpiresIn,
		TokenType:    response.TokenPair.TokenType,
	}

	h.logger.Info("Token refresh successful",
		zap.String("user_id", response.Session.UserID),
		zap.String("session_id", response.Session.ID),
		zap.String("ip", ip),
		zap.Duration("duration", time.Since(startTime)))

	h.auditLog(AuditEvent{
		Action:    "token_refresh_success",
		UserID:    response.Session.UserID,
		SessionID: response.Session.ID,
		IPAddress: ip,
		UserAgent: userAgent,
		Success:   true,
		Timestamp: time.Now(),
	})

	WriteJSONResponse(w, http.StatusOK, tokenResponse)
}

// GetActiveSessions returns all active sessions for the user (requires authentication)
func (h *AuthHandler) GetActiveSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip, userAgent := h.getClientInfo(r)

	// Get user ID from context
	userID, ok := GetUserID(ctx)
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found", nil)
		return
	}

	// Get sessions using the service
	sessionInfos, err := h.service.GetActiveSessions(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get active sessions",
			zap.String("user_id", userID),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "get_sessions_failed",
			UserID:    userID,
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})

		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to retrieve sessions", nil)
		return
	}

	h.auditLog(AuditEvent{
		Action:    "get_sessions_success",
		UserID:    userID,
		IPAddress: ip,
		UserAgent: userAgent,
		Success:   true,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"session_count": len(sessionInfos)},
	})

	WriteJSONResponse(w, http.StatusOK, map[string]interface{}{
		"sessions": sessionInfos,
		"count":    len(sessionInfos),
	})
}

// RevokeSpecificSession revokes a specific session by ID (requires authentication)
func (h *AuthHandler) RevokeSpecificSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip, userAgent := h.getClientInfo(r)

	// Get user ID from context
	userID, ok := GetUserID(ctx)
	if !ok {
		WriteErrorResponse(w, http.StatusUnauthorized, "User not found", nil)
		return
	}

	// Get session ID from URL path
	sessionID := chi.URLParam(r, "sessionID")
	if sessionID == "" {
		WriteErrorResponse(w, http.StatusBadRequest, "Session ID is required", nil)
		return
	}

	// Create timeout context
	timeout := DefaultLogoutTimeout
	if h.config != nil {
		timeout = h.config.LogoutTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Revoke the specific session
	err := h.coordinator.RevokeSessionAndTokens(timeoutCtx, sessionID)
	if err != nil {
		h.logger.Error("Failed to revoke specific session",
			zap.String("user_id", userID),
			zap.String("target_session_id", sessionID),
			zap.Error(err))

		h.auditLog(AuditEvent{
			Action:    "revoke_session_failed",
			UserID:    userID,
			IPAddress: ip,
			UserAgent: userAgent,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
			Details:   map[string]interface{}{"target_session_id": sessionID},
		})

		WriteErrorResponse(w, http.StatusInternalServerError, "Failed to revoke session", nil)
		return
	}

	h.logger.Info("Session revoked successfully",
		zap.String("user_id", userID),
		zap.String("target_session_id", sessionID),
		zap.String("ip", ip))

	h.auditLog(AuditEvent{
		Action:    "revoke_session_success",
		UserID:    userID,
		IPAddress: ip,
		UserAgent: userAgent,
		Success:   true,
		Timestamp: time.Now(),
		Details:   map[string]interface{}{"target_session_id": sessionID},
	})

	WriteJSONResponse(w, http.StatusOK, map[string]string{
		"message":    "Session revoked successfully",
		"session_id": sessionID,
	})
}

// mapErrorToHTTPStatus maps domain errors to appropriate HTTP status codes
func (h *AuthHandler) mapErrorToHTTPStatus(err error) int {
	// Check if it's a structured auth error
	if authErr := auth.GetAuthError(err); authErr != nil {
		switch authErr.Type {
		case "validation_error":
			return http.StatusBadRequest
		case "user_error":
			if authErr.Code == "USER_EXISTS" {
				return http.StatusConflict
			}
			return http.StatusBadRequest
		case "auth_error":
			return http.StatusUnauthorized
		case "rate_limit":
			return http.StatusTooManyRequests
		case "not_implemented":
			return http.StatusNotImplemented
		default:
			return http.StatusInternalServerError
		}
	}

	// Fallback to checking standard errors
	errorMessage := err.Error()
	switch {
	case strings.Contains(errorMessage, "already exists"):
		return http.StatusConflict
	case strings.Contains(errorMessage, "invalid") && strings.Contains(errorMessage, "credentials"):
		return http.StatusUnauthorized
	case strings.Contains(errorMessage, "not found"):
		return http.StatusNotFound
	case strings.Contains(errorMessage, "unauthorized"):
		return http.StatusUnauthorized
	case strings.Contains(errorMessage, "forbidden"):
		return http.StatusForbidden
	case strings.Contains(errorMessage, "timeout"):
		return http.StatusRequestTimeout
	case strings.Contains(errorMessage, "not implemented"):
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}

// Helper functions to get values from middleware context

// GetSession retrieves session from middleware context
func GetSession(ctx context.Context) (*sessions.Session, bool) {
	session, ok := ctx.Value(ContextKeySession).(*sessions.Session)
	return session, ok
}

// GetFreshTier retrieves fresh user tier from middleware context
func GetFreshTier(ctx context.Context) (tokens.UserTier, bool) {
	tier, ok := ctx.Value(ContextKeyFreshTier).(tokens.UserTier)
	return tier, ok
}

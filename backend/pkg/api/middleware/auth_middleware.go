package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"slices"

	"github.com/kdot/k4-chat/backend/internal/auth/sessions"
	"github.com/kdot/k4-chat/backend/internal/auth/tokens"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// contextKey is a custom type for context keys
type contextKey string

const (
	sessionContextKey contextKey = "session"
	freshTierKey      contextKey = "fresh_tier"
	claimsContextKey  contextKey = "claims"
	userContextKey    contextKey = "user"
	userIDContextKey  contextKey = "user_id"
)

// UserService defines interface for fetching user information
type UserService interface {
	GetUserTierByStringID(ctx context.Context, userID string) (string, error)
}

// RateLimiter defines interface for rate limiting
type RateLimiter interface {
	Allow(ctx context.Context, key string) (bool, error)
	GetRemaining(ctx context.Context, key string) (int, error)
	GetReset(ctx context.Context, key string) (time.Time, error)
	GetConfig() RateLimiterConfig
}

// RateLimiterConfig defines rate limiter configuration
type RateLimiterConfig struct {
	DefaultLimit   int
	WhitelistedIPs []string
}

func (c RateLimiterConfig) IsIPWhitelisted(ip string) bool {
	return slices.Contains(c.WhitelistedIPs, ip)
}

// AuthMiddleware handles authentication and rate limiting with fresh tier validation
type AuthMiddleware struct {
	sessionManager sessions.SessionManager
	rateLimiter    RateLimiter
	tokenStore     tokens.TokenStore
	userService    UserService
	redisClient    redis.Cmdable
	logger         *zap.Logger
}

// NewAuthMiddleware creates a new authentication middleware with user service
func NewAuthMiddleware(
	sessionManager sessions.SessionManager,
	rateLimiter RateLimiter,
	tokenStore tokens.TokenStore,
	userService UserService,
	redisClient redis.Cmdable,
	logger *zap.Logger,
) *AuthMiddleware {
	return &AuthMiddleware{
		sessionManager: sessionManager,
		rateLimiter:    rateLimiter,
		tokenStore:     tokenStore,
		userService:    userService,
		redisClient:    redisClient,
		logger:         logger,
	}
}

// checkRefreshNeeded checks if a session needs to refresh due to tier changes
func (m *AuthMiddleware) checkRefreshNeeded(ctx context.Context, sessionID string, userID string) (bool, string, error) {
	if sessionID == "" || userID == "" {
		return false, "", nil
	}

	// Check session-specific refresh flag first
	sessionRefreshKey := "session:" + sessionID + ":refresh_needed"
	sessionRefreshNeeded, err := m.redisClient.Get(ctx, sessionRefreshKey).Result()
	if err == nil {
		// Clear the flag after checking
		m.redisClient.Del(ctx, sessionRefreshKey)
		return true, sessionRefreshNeeded, nil
	}

	// Check user-level refresh flag
	userRefreshKey := "user:" + userID + ":refresh_needed"
	_, err = m.redisClient.Get(ctx, userRefreshKey).Result()
	if err == nil {
		return true, "tier_change", nil
	}

	// If error (including redis.Nil), no refresh needed
	return false, "", nil
}

// Authenticate is a middleware that handles authentication with fresh tier validation
func (m *AuthMiddleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client IP and user agent
		ip := r.RemoteAddr
		userAgent := r.UserAgent()

		// Clean up the IP address (remove port if present)
		ip = strings.Split(ip, ":")[0]

		// Create rate limit key
		rateLimitKey := "auth:ip:" + ip

		// Check if IP is whitelisted for rate limiting bypass
		config := m.rateLimiter.GetConfig()
		isWhitelisted := config.IsIPWhitelisted(ip)

		// Check rate limit only if not whitelisted
		if !isWhitelisted {
			allowed, err := m.rateLimiter.Allow(r.Context(), rateLimitKey)
			if err != nil {
				m.logger.Error("Rate limit check failed",
					zap.Error(err),
					zap.String("ip", ip))
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				remaining, err := m.rateLimiter.GetRemaining(r.Context(), rateLimitKey)
				if err != nil {
					m.logger.Error("Failed to get remaining requests")
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}
				reset, err := m.rateLimiter.GetReset(r.Context(), rateLimitKey)
				if err != nil {
					m.logger.Error("Failed to get reset time")
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.DefaultLimit))
				w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
				w.Header().Set("X-RateLimit-Reset", reset.Format(time.RFC3339))
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}

		// Get token from header - ALWAYS required for protected endpoints
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No authorization header - this is an unauthenticated request
			next.ServeHTTP(w, r)
			return
		}

		// Extract token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		token := tokenParts[1]

		// First validate basic token structure
		basicClaims, err := m.tokenStore.ValidateToken(r.Context(), token)
		if err != nil {
			m.logger.Warn("Invalid token",
				zap.Error(err),
				zap.String("ip", ip))
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Get session to extract device info for enhanced validation
		session, err := m.sessionManager.GetSession(r.Context(), basicClaims.SessionID)
		if err != nil {
			m.logger.Warn("Session not found",
				zap.Error(err),
				zap.String("sessionID", basicClaims.SessionID))
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		// Enhanced token validation with session and device binding
		claims, err := m.tokenStore.ValidateTokenWithSession(r.Context(), token, session.ID, session.DeviceID)
		if err != nil {
			m.logger.Warn("Token binding validation failed",
				zap.Error(err),
				zap.String("session_id", session.ID),
				zap.String("device_id", session.DeviceID),
				zap.String("user_id", basicClaims.UserID),
				zap.String("ip", ip),
				zap.String("user_agent", userAgent))
			http.Error(w, "Token binding validation failed", http.StatusUnauthorized)
			return
		}

		// Validate session
		validationResult, err := m.sessionManager.ValidateSession(r.Context(), session.ID)
		if err != nil || validationResult == nil || !validationResult.Valid {
			reason := "invalid session"
			if validationResult != nil && validationResult.Reason != "" {
				reason = validationResult.Reason
			}
			m.logger.Warn("Invalid session",
				zap.Error(err),
				zap.String("sessionID", session.ID),
				zap.String("reason", reason))
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		// Check if session needs refresh due to tier changes
		refreshNeeded, refreshReason, err := m.checkRefreshNeeded(r.Context(), claims.SessionID, claims.UserID)
		if err != nil {
			m.logger.Warn("Failed to check refresh needed", zap.Error(err))
		}

		if refreshNeeded {
			m.logger.Info("Token refresh required due to tier changes",
				zap.String("userID", claims.UserID),
				zap.String("sessionID", claims.SessionID),
				zap.String("reason", refreshReason))

			// Set a custom header to indicate refresh is needed
			w.Header().Set("X-Refresh-Required", "true")
			w.Header().Set("X-Refresh-Reason", refreshReason)
			http.Error(w, "Token refresh required", http.StatusUnauthorized)
			return
		}

		// Fetch FRESH tier from user service instead of using cached token tier
		var freshTier tokens.UserTier
		freshTierStr, err := m.userService.GetUserTierByStringID(r.Context(), claims.UserID)
		if err != nil {
			m.logger.Warn("Failed to fetch fresh user tier, using cached tier",
				zap.Error(err),
				zap.String("userID", claims.UserID))
			freshTier = claims.Tier // Fallback to cached tier
		} else {
			// Convert string to UserTier
			freshTier = stringToUserTier(freshTierStr)

			// Log if tier has changed since token was issued
			if claims.Tier != freshTier {
				m.logger.Info("User tier has changed since token was issued",
					zap.String("userID", claims.UserID),
					zap.String("tokenTier", string(claims.Tier)),
					zap.String("freshTier", string(freshTier)))
			}
		}

		// Update session metadata
		metadata := &sessions.SessionMetadata{
			DeviceID:  session.DeviceID,
			IPAddress: ip,
			UserAgent: userAgent,
			LastUsed:  time.Now(),
		}
		if err := m.sessionManager.UpdateSession(r.Context(), session.ID, metadata); err != nil {
			m.logger.Error("Failed to update session",
				zap.Error(err),
				zap.String("sessionID", session.ID))
		}

		// Add session and claims to context
		ctx := context.WithValue(r.Context(), sessionContextKey, session)
		ctx = context.WithValue(ctx, freshTierKey, freshTier)
		ctx = context.WithValue(ctx, claimsContextKey, claims)
		ctx = context.WithValue(ctx, userContextKey, claims)
		ctx = context.WithValue(ctx, userIDContextKey, claims.UserID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireTier returns a middleware that ensures the user has the required tier or higher
func RequireTier(requiredTier tokens.UserTier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from context
			_, ok := GetUserID(r.Context())
			if !ok {
				http.Error(w, "User ID not found in context", http.StatusForbidden)
				return
			}

			// Try to get fresh tier from context first (set by auth middleware)
			userTier, hasFreshTier := GetFreshTier(r.Context())
			if !hasFreshTier {
				// Fallback to token claims if fresh tier not available
				claims, ok := GetClaims(r.Context())
				if !ok {
					http.Error(w, "User claims not found in context", http.StatusForbidden)
					return
				}
				userTier = claims.Tier
			}

			// Check if user has required tier or higher
			if !userTier.HasTier(requiredTier) {
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			// Permission granted; proceed to the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// Convenience middleware functions for common tiers
func RequirePremium() func(http.Handler) http.Handler {
	return RequireTier(tokens.TierPremium)
}

func RequireAdmin() func(http.Handler) http.Handler {
	return RequireTier(tokens.TierAdmin)
}

func RequireModerator() func(http.Handler) http.Handler {
	return RequireTier(tokens.TierModerator)
}

// Context helper functions

// GetFreshTier retrieves the fresh tier from context (set by auth middleware)
func GetFreshTier(ctx context.Context) (tokens.UserTier, bool) {
	tier, ok := ctx.Value(freshTierKey).(tokens.UserTier)
	return tier, ok
}

// GetClaims retrieves token claims from context
func GetClaims(ctx context.Context) (*tokens.TokenClaims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*tokens.TokenClaims)
	return claims, ok
}

// GetUserID retrieves user ID from context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}

// GetSession retrieves session from context
func GetSession(ctx context.Context) (*sessions.Session, bool) {
	session, ok := ctx.Value(sessionContextKey).(*sessions.Session)
	return session, ok
}

// stringToUserTier converts a string tier to tokens.UserTier
func stringToUserTier(tierStr string) tokens.UserTier {
	switch tierStr {
	case "free":
		return tokens.TierFree
	case "premium":
		return tokens.TierPremium
	case "admin":
		return tokens.TierAdmin
	case "moderator":
		return tokens.TierModerator
	default:
		return tokens.TierFree // Default to free for unknown tiers
	}
}

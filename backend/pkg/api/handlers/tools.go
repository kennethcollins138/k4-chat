package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"

	"github.com/kdot/k4-chat/backend/configs"
	"github.com/kdot/k4-chat/backend/internal/auth/tokens"
	"github.com/kdot/k4-chat/backend/pkg/api/middleware"
)

/*
=========================================================================================================================
											RESPONSE UTILS
=========================================================================================================================
*/
// WriteJSONResponse writes the given data as a JSON response with the specified HTTP status code.
// It sets the "Content-Type" header to "application/json" and encodes the provided data into JSON.
// If the data is nil, no JSON is written.
//
// Parameters:
//   - w: The http.ResponseWriter used to write the response.
//   - status: The HTTP status code to set in the response.
//   - data: The data to encode as JSON and write to the response.
func WriteJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		err := json.NewEncoder(w).Encode(data)
		if err != nil {
			fmt.Println(err)
		}
	}
}

// WriteErrorResponse writes an error response as JSON with a given HTTP status code, error message, and additional details.
// It constructs a JSON object containing the error message and details, then calls WriteJSONResponse to send the response.
//
// Parameters:
//   - w: The http.ResponseWriter used to write the response.
//   - status: The HTTP status code to set in the response.
//   - message: A string describing the error.
//   - details: Additional details about the error, which can be any type.
func WriteErrorResponse(w http.ResponseWriter, status int, message string, details interface{}) {
	response := map[string]interface{}{
		"error":   message,
		"details": details,
	}
	WriteJSONResponse(w, status, response)
}

// HandleValidationErrors is a helper for writing validation error responses.
func HandleValidationErrors(w http.ResponseWriter, err error) {
	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		errs := make([]string, 0, len(validationErrs))
		for _, ve := range validationErrs {
			errs = append(errs, ve.Error())
		}
		WriteErrorResponse(w, http.StatusBadRequest, "Validation failed", errs)
		return
	}
	WriteErrorResponse(w, http.StatusBadRequest, "Validation failed", err.Error())
}

// StatusResponseWriter is a custom response writer that captures status code
type StatusResponseWriter struct {
	http.ResponseWriter
	StatusCode   int
	BytesWritten int
}

// NewStatusResponseWriter creates a new status response writer
func NewStatusResponseWriter(w http.ResponseWriter) *StatusResponseWriter {
	return &StatusResponseWriter{
		ResponseWriter: w,
		StatusCode:     http.StatusOK, // Default to 200 OK
	}
}

// WriteHeader captures the status code and passes it to the wrapped writer
func (srw *StatusResponseWriter) WriteHeader(code int) {
	srw.StatusCode = code
	srw.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written
func (srw *StatusResponseWriter) Write(b []byte) (int, error) {
	n, err := srw.ResponseWriter.Write(b)
	srw.BytesWritten += n
	return n, err
}

// Hijack implements the http.Hijacker interface
func (srw *StatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := srw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// Flush implements the http.Flusher interface
func (srw *StatusResponseWriter) Flush() {
	f, ok := srw.ResponseWriter.(http.Flusher)
	if ok {
		f.Flush()
	}
}

// Push implements the http.Pusher interface
func (srw *StatusResponseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := srw.ResponseWriter.(http.Pusher)
	if !ok {
		return errors.New("underlying ResponseWriter does not implement http.Pusher")
	}
	return p.Push(target, opts)
}

/*
=========================================================================================================================

	AUTH REQUEST UTILS

=========================================================================================================================
*/
var (
	ErrTokenMissing        = errors.New("authorization token is missing")
	ErrInvalidTokenFormat  = errors.New("invalid token format")
	ErrTokenVerification   = errors.New("token verification failed")
	ErrVersionMismatch     = errors.New("token version mismatch")
	ErrMissingContextValue = errors.New("missing value in context")
)

// GetUserIDFromClaims extracts the user ID from the claims in request context.
func GetUserIDFromClaims(r *http.Request) (string, error) {
	// Get the user ID from the context (set by the JWT middleware).
	claims, ok := r.Context().Value(ContextKeyUser).(*tokens.TokenClaims)
	if !ok {
		return "", errors.New("no user claims found")
	}
	return claims.UserID, nil
}

// GetTokenVersionFromClaims extracts the token version from the claims in request context.
func GetTokenVersionFromClaims(r *http.Request) (int, error) {
	// Get the token version from the context (set by the JWT middleware).
	claims, ok := r.Context().Value(ContextKeyUser).(*tokens.TokenClaims)
	if !ok {
		return -1, errors.New("no user claims found")
	}
	return claims.TokenVersion, nil
}

// GetUserTierFromRedisWithFallback retrieves user tier from Redis cache with PostgreSQL fallback
// This function provides better error handling and supports fallback to database when cache misses
func GetUserTierFromRedisWithFallback(ctx context.Context, redisClient redis.Cmdable, userID string, userService middleware.UserService) (string, bool, error) {
	if userID == "" {
		return "", false, fmt.Errorf("userID is required")
	}

	// Get proper Redis configuration
	cfg := configs.GetConfig()
	if cfg == nil {
		return "", false, fmt.Errorf("configuration not loaded")
	}

	// Use proper key structure with config
	redisKey := cfg.Database.Redis.KeyPrefix + "user_tier:" + userID

	// Try to get from Redis cache first (using simple string key for user tier)
	userTier, err := redisClient.Get(ctx, redisKey).Result()
	if err == nil {
		// Cache hit
		return userTier, true, nil
	}

	// Check if it's a cache miss (redis.Nil) vs actual error
	if !errors.Is(err, redis.Nil) {
		// Log Redis error but don't fail - we'll fallback to database
		// This error should be logged but we continue with fallback
		return "", false, fmt.Errorf("redis error retrieving user tier: %w", err)
	}

	// Cache miss - try fallback to user service if provided
	if userService != nil {
		freshTier, dbErr := userService.GetUserTierByStringID(ctx, userID)
		if dbErr != nil {
			return "", false, fmt.Errorf("failed to get user tier from database: %w", dbErr)
		}

		// Cache the result in Redis for future requests (with TTL)
		cacheTTL := cfg.Database.Redis.DefaultTTL
		if cacheTTL == 0 {
			cacheTTL = time.Hour // Default 1 hour if not configured
		}

		cacheErr := redisClient.Set(ctx, redisKey, freshTier, cacheTTL).Err()
		if cacheErr != nil {
			// Log caching error but don't fail the request
			// We got the data from DB successfully
		}

		return freshTier, false, nil // false indicates it came from database, not cache
	}

	// No fallback service provided and cache miss
	return "", false, fmt.Errorf("user tier not cached for user: %s and no fallback service provided", userID)
}

/*
====================================================================================================================
										CONTEXT UTILS
====================================================================================================================
*/
// GetClaims gets the full claims from context
func GetClaims(ctx context.Context) (*tokens.TokenClaims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*tokens.TokenClaims)
	return claims, ok
}

// GetUserIDFromContext gets the user ID from the context
func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

// SetUserIDToContext sets the user ID in the context
func SetUserIDToContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// SetUserID sets the user ID in the request context
func SetUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
	return r.WithContext(ctx)
}

// GetUserID gets the user ID from the context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

// SetTokenVersion sets the token version in the request context
func SetTokenVersion(r *http.Request, version int) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyTokenVersion, version)
	return r.WithContext(ctx)
}

// GetTokenVersion gets the token version from the context
func GetTokenVersion(ctx context.Context) (int, bool) {
	version, ok := ctx.Value(ContextKeyTokenVersion).(int)
	return version, ok
}

// SetRegion sets the region in the request context
func SetRegion(r *http.Request, region string) *http.Request {
	ctx := context.WithValue(r.Context(), ContextKeyRegion, region)
	return r.WithContext(ctx)
}

// GetRegion gets the region from the context
func GetRegion(ctx context.Context) (string, bool) {
	region, ok := ctx.Value(ContextKeyRegion).(string)
	return region, ok
}

// GetSessionID gets the session ID from the context
func GetSessionID(ctx context.Context) (string, bool) {
	sessionID, ok := ctx.Value(ContextKeySessionID).(string)
	return sessionID, ok
}

// SetSessionID sets the session ID in the context
func SetSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, ContextKeySessionID, sessionID)
}

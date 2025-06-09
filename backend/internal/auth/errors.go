package auth

import (
	"errors"
	"fmt"
)

// Domain-specific authentication errors
var (
	// User-related errors
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotActive      = errors.New("user account is not active")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrEmailTaken         = errors.New("email is already taken")

	// Password-related errors
	ErrWeakPassword       = errors.New("password does not meet security requirements")
	ErrPasswordTooShort   = errors.New("password is too short")
	ErrPasswordTooLong    = errors.New("password is too long")
	ErrPasswordHashFailed = errors.New("failed to hash password")

	// Email-related errors
	ErrInvalidEmail     = errors.New("invalid email format")
	ErrEmailNotVerified = errors.New("email address not verified")

	// Session/Token-related errors
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrTokenExpired    = errors.New("token has expired")
	ErrTokenRevoked    = errors.New("token has been revoked")
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session has expired")
	ErrTooManySessions = errors.New("too many active sessions")

	// OAuth-related errors
	ErrInvalidOAuthProvider = errors.New("invalid OAuth provider")
	ErrOAuthCodeInvalid     = errors.New("invalid OAuth authorization code")
	ErrOAuthStateInvalid    = errors.New("invalid OAuth state parameter")
	ErrOAuthExchangeFailed  = errors.New("OAuth token exchange failed")

	// Rate limiting errors
	ErrTooManyAttempts   = errors.New("too many failed attempts")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// Validation errors
	ErrInvalidInput         = errors.New("invalid input")
	ErrMissingRequiredField = errors.New("missing required field")
	ErrFieldTooLong         = errors.New("field value too long")
	ErrFieldTooShort        = errors.New("field value too short")
	ErrInvalidFormat        = errors.New("invalid format")
)

// AuthError represents a structured authentication error
type AuthError struct {
	Type    string                 `json:"type"`
	Message string                 `json:"message"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
	Err     error                  `json:"-"` // Internal error, not exposed
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewAuthError creates a new structured auth error
func NewAuthError(errType, message string) *AuthError {
	return &AuthError{
		Type:    errType,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// NewAuthErrorWithCode creates a new auth error with a specific error code
func NewAuthErrorWithCode(errType, message, code string) *AuthError {
	return &AuthError{
		Type:    errType,
		Message: message,
		Code:    code,
		Details: make(map[string]interface{}),
	}
}

// WithDetails adds details to an auth error
func (e *AuthError) WithDetails(key string, value interface{}) *AuthError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithError wraps an internal error
func (e *AuthError) WithError(err error) *AuthError {
	e.Err = err
	return e
}

// Common error constructors for convenience
func NewUserNotFoundError() *AuthError {
	return NewAuthErrorWithCode("user_error", "User not found", "USER_NOT_FOUND")
}

func NewInvalidCredentialsError() *AuthError {
	return NewAuthErrorWithCode("auth_error", "Invalid credentials", "INVALID_CREDENTIALS")
}

func NewUserAlreadyExistsError(field string) *AuthError {
	return NewAuthErrorWithCode("user_error", "User already exists", "USER_EXISTS").
		WithDetails("field", field)
}

func NewWeakPasswordError(requirements []string) *AuthError {
	return NewAuthErrorWithCode("validation_error", "Password does not meet requirements", "WEAK_PASSWORD").
		WithDetails("requirements", requirements)
}

func NewValidationError(field, reason string) *AuthError {
	return NewAuthErrorWithCode("validation_error", "Validation failed", "VALIDATION_FAILED").
		WithDetails("field", field).
		WithDetails("reason", reason)
}

func NewRateLimitError(retryAfter int) *AuthError {
	return NewAuthErrorWithCode("rate_limit", "Rate limit exceeded", "RATE_LIMIT_EXCEEDED").
		WithDetails("retry_after_seconds", retryAfter)
}

// IsAuthError checks if an error is an AuthError
func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

// GetAuthError extracts AuthError from error chain
func GetAuthError(err error) *AuthError {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr
	}
	return nil
}

package apperrors

// ErrorCode represents a unique error code for each error type/domain
// Group codes by domain: 1000-1999 Auth, 2000-2999 Token, 3000-3999 DB, etc.
const (
	// Auth errors
	ErrAuthInvalidCredentials = 1001
	ErrAuthUnauthorized       = 1002
	ErrAuthForbidden          = 1003

	// Token errors
	ErrTokenSign            = 2001
	ErrTokenParse           = 2002
	ErrTokenExpired         = 2003
	ErrTokenBlacklisted     = 2004
	ErrTokenNotFound        = 2005
	ErrTokenVersionMismatch = 2006
	ErrTokenBinding         = 2007 // Token binding validation failed

	// DB errors
	ErrDBConnection = 3001
	ErrDBQuery      = 3002
)

// Constructors for each domain
func NewAuthError(code int, msg string, cause error) *AppError {
	return &AppError{Code: code, Message: msg, Cause: cause}
}

func NewTokenError(code int, msg string, cause error) *AppError {
	return &AppError{Code: code, Message: msg, Cause: cause}
}

func NewDBError(code int, msg string, cause error) *AppError {
	return &AppError{Code: code, Message: msg, Cause: cause}
}

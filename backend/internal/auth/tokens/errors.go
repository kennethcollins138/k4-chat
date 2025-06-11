package tokens

import "errors"

// Token-related errors
var (
	ErrTokenExpired         = errors.New("token has expired")
	ErrTokenInvalid         = errors.New("token is invalid")
	ErrTokenBlacklisted     = errors.New("token has been blacklisted")
	ErrTokenRotationFailed  = errors.New("token rotation failed")
	ErrTokenNotFound        = errors.New("token not found")
	ErrTokenVersionMismatch = errors.New("token version mismatch")
	ErrTokenReuse           = errors.New("token reuse detected")
)

package auth

import "time"

/*
================================================================================================================
										SHARED RESOURCES
================================================================================================================
*/

// BaseRequest contains common fields for auth requests
type BaseRequest struct {
	RequestType string `json:"type" validate:"required,oneof=oauth email"`
}

/*
================================================================================================================
										SIGN UP REQUESTS
================================================================================================================
*/

// SignUpRequest is the main signup request wrapper
type SignUpRequest struct {
	RequestType string              `json:"type" validate:"required,oneof=oauth email"`
	Email       *EmailSignUpRequest `json:"email,omitempty"`
	OAuth       *OAuthSignUpRequest `json:"oauth,omitempty"`
}

// EmailSignUpRequest handles email-based signup
type EmailSignUpRequest struct {
	Email       string  `json:"email" validate:"required,email"`
	Username    string  `json:"username" validate:"required,min=3,max=50,alphanum"`
	Password    string  `json:"password" validate:"required,min=8"`
	DisplayName *string `json:"display_name,omitempty" validate:"omitempty,max=100"`
}

// OAuthSignUpRequest handles OAuth-based signup
type OAuthSignUpRequest struct {
	Provider    string `json:"provider" validate:"required,oneof=google github discord"`
	Code        string `json:"code" validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required,url"`
	State       string `json:"state" validate:"required"`
}

/*
================================================================================================================
										SIGN IN REQUESTS
================================================================================================================
*/

// SignInRequest is the main signin request wrapper
type SignInRequest struct {
	RequestType string              `json:"type" validate:"required,oneof=oauth email"`
	Email       *EmailSignInRequest `json:"email,omitempty"`
	OAuth       *OAuthSignInRequest `json:"oauth,omitempty"`
	RememberMe  bool                `json:"remember_me"`
}

// EmailSignInRequest handles email-based signin
type EmailSignInRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// OAuthSignInRequest handles OAuth-based signin
type OAuthSignInRequest struct {
	Provider    string `json:"provider" validate:"required,oneof=google github discord"`
	Code        string `json:"code" validate:"required"`
	RedirectURI string `json:"redirect_uri" validate:"required,url"`
	State       string `json:"state" validate:"required"`
}

/*
================================================================================================================
										RESPONSES
================================================================================================================
*/

// AuthResponse represents successful authentication
type AuthResponse struct {
	User         UserDTO `json:"user"`
	AccessToken  string  `json:"access_token"`
	RefreshToken string  `json:"refresh_token"`
	ExpiresIn    int64   `json:"expires_in"`
	TokenType    string  `json:"token_type"`
}

// UserDTO represents user data for API responses (never includes sensitive fields)
type UserDTO struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	DisplayName  *string   `json:"display_name,omitempty"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	IsActive     bool      `json:"is_active"`
}

// ErrorResponse represents auth error responses
type ErrorResponse struct {
	Error       string                 `json:"error"`
	Description string                 `json:"error_description,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

/*
================================================================================================================
										SESSION MANAGEMENT
================================================================================================================
*/

// RefreshTokenRequest handles token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// SessionInfo represents session information
type SessionInfo struct {
	SessionID    string    `json:"session_id"`
	DeviceInfo   string    `json:"device_info"`
	IPAddress    string    `json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	IsActive     bool      `json:"is_active"`
}

// SessionsResponse lists active sessions
type SessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"

	"github.com/kdot/k4-chat/backend/internal/auth"
	"github.com/kdot/k4-chat/backend/internal/database/models"

	"go.uber.org/zap"
)

const (
	// MaxRequestBodySize limits request body size to prevent DoS attacks
	MaxRequestBodySize = 1 << 20 // 1MB
)

/*
AuthHandlerInterface defines the methods that the AuthHandler is required to implement
A lot of underlying methods network logic is passed to middleware
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

type AuthHandler struct {
	service   auth.Service
	logger    *zap.Logger
	validator *validator.Validate
}

func NewAuthHandler(service auth.Service, logger *zap.Logger) AuthHandlerInterface {
	return &AuthHandler{
		service:   service,
		logger:    logger,
		validator: validator.New(),
	}
}

// parseAndValidateRequest safely parses and validates JSON request
func (h *AuthHandler) parseAndValidateRequest(r *http.Request, dest interface{}) error {
	// Limit request body size to prevent DoS attacks
	r.Body = http.MaxBytesReader(nil, r.Body, MaxRequestBodySize)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // Reject extra fields for security

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

// getClientInfo extracts client information for security logging
func (h *AuthHandler) getClientInfo(r *http.Request) (ip, userAgent string) {
	// Get real IP (considering proxies)
	ip = r.Header.Get("X-Real-IP")
	if ip == "" {
		ip = r.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	userAgent = r.Header.Get("User-Agent")
	return ip, userAgent
}

func (h *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip, userAgent := h.getClientInfo(r)

	// Parse and validate request
	var req auth.SignUpRequest
	if err := h.parseAndValidateRequest(r, &req); err != nil {
		h.logger.Warn("Invalid signup request",
			zap.String("ip", ip),
			zap.String("user_agent", userAgent),
			zap.Error(err))
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request", map[string]string{
			"details": "Request validation failed",
		})
		return
	}

	// Validate request structure
	if err := h.validateSignUpRequest(&req); err != nil {
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
		user, err = h.service.SignUpWithEmail(ctx, req)

	case "oauth":
		if req.OAuth == nil {
			WriteErrorResponse(w, http.StatusBadRequest, "Missing OAuth signup data", nil)
			return
		}
		user, err = h.service.SignUpWithOAuth(ctx, req)

	default:
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request type", nil)
		return
	}

	// Handle service errors
	if err != nil {
		// Log error but don't expose details to client
		h.logger.Error("Signup failed",
			zap.String("type", req.RequestType),
			zap.String("ip", ip),
			zap.Error(err))

		// Map domain errors to HTTP status codes
		status := h.mapErrorToHTTPStatus(err)
		WriteErrorResponse(w, status, "Signup failed", nil)
		return
	}
	
	// Success - convert to DTO and return
	userDTO := h.convertUserToDTO(user)

	// TODO: Generate and include JWT tokens in response
	response := auth.AuthResponse{
		User:         userDTO,
		AccessToken:  "",   // TODO: Generate JWT
		RefreshToken: "",   // TODO: Generate refresh token
		ExpiresIn:    3600, // 1 hour
		TokenType:    "Bearer",
	}

	h.logger.Info("User signup successful",
		zap.String("user_id", userDTO.ID),
		zap.String("username", userDTO.Username),
		zap.String("ip", ip))

	WriteJSONResponse(w, http.StatusCreated, response)
}

// validateSignUpRequest performs additional business logic validation
func (h *AuthHandler) validateSignUpRequest(req *auth.SignUpRequest) error {
	switch req.RequestType {
	case "email":
		if req.Email == nil {
			return fmt.Errorf("email signup data is required")
		}
		// Additional email-specific validation could go here

	case "oauth":
		if req.OAuth == nil {
			return fmt.Errorf("OAuth signup data is required")
		}
		// Additional OAuth-specific validation could go here

	default:
		return fmt.Errorf("invalid request type: %s", req.RequestType)
	}

	return nil
}

func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context() // TODO: Use when implementing signin logic
	ip, userAgent := h.getClientInfo(r)

	var req auth.SignInRequest
	if err := h.parseAndValidateRequest(r, &req); err != nil {
		h.logger.Warn("Invalid signin request",
			zap.String("ip", ip),
			zap.String("user_agent", userAgent),
			zap.Error(err))
		WriteErrorResponse(w, http.StatusBadRequest, "Invalid request", nil)
		return
	}

	// TODO: Implement signin logic similar to signup
	// For now, return not implemented
	WriteErrorResponse(w, http.StatusNotImplemented, "SignIn not yet implemented", nil)
}

func (h *AuthHandler) SignOut(w http.ResponseWriter, r *http.Request) {
	WriteErrorResponse(w, http.StatusNotImplemented, "SignOut not yet implemented", nil)
}

func (h *AuthHandler) SignOutAllDevices(w http.ResponseWriter, r *http.Request) {
	WriteErrorResponse(w, http.StatusNotImplemented, "SignOutAllDevices not yet implemented", nil)
}

func (h *AuthHandler) RevokeSpecificSession(w http.ResponseWriter, r *http.Request) {
	WriteErrorResponse(w, http.StatusNotImplemented, "RevokeSpecificSession not yet implemented", nil)
}

func (h *AuthHandler) RefreshTokens(w http.ResponseWriter, r *http.Request) {
	WriteErrorResponse(w, http.StatusNotImplemented, "RefreshTokens not yet implemented", nil)
}

func (h *AuthHandler) GetActiveSessions(w http.ResponseWriter, r *http.Request) {
	WriteErrorResponse(w, http.StatusNotImplemented, "GetActiveSessions not yet implemented", nil)
}

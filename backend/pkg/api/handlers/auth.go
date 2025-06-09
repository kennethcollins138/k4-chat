package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/kdot/k4-chat/backend/internal/auth"
	"github.com/kdot/k4-chat/backend/internal/database/models"

	"go.uber.org/zap"
)

type AuthHandler struct {
	service auth.Service
	logger  *zap.Logger
}

func NewAuthHandler(service auth.Service, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		service: service,
		logger:  logger,
	}
}

func (h *AuthHandler) SignUp(w http.ResponseWriter, r *http.Request) {
	// Initialize Variables
	var req auth.SignUpRequest
	var err error
	var user *models.User

	// Move logic to context/middleware for more readability and separation.
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		WriteErrorResponse(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	defer func(Body io.ReadCloser) {
		// TODO: Swap to helper function to enforce DRY, this too clunky
		err = Body.Close()
		if err != nil {
			h.logger.Error("failed to close request body", zap.Error(err))
			// TODO: retry logic make more elegant
		}
	}(r.Body)

	// Read body, switch statement to propagate to proper service.
	switch req.RequestType {
	case "oauth":
		user, err = h.service.SignUpWithOAuth(r.Context(), req)
		if err != nil {
			WriteErrorResponse(w, http.StatusInternalServerError, "failed to sign up with OAuth", err)
			return
		}
	case "email":
		user, err = h.service.SignUpWithEmail(r.Context(), req)
		if err != nil {
			WriteErrorResponse(w, http.StatusInternalServerError, "failed to sign up with email", err)
			return
		}
	default:
		WriteErrorResponse(w, http.StatusBadRequest, "invalid request type", nil)
		return
	}
	// Next Steps
	// Handle token logic, fingerprinting, etc
	// Security headers etc

	// probably wrap User Object into dto, need to handle auth as well
	WriteJSONResponse(w, http.StatusOK, user)
	return
}

func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) (*models.User, error) {
	return nil, nil
}

func (h *AuthHandler) SignOut(ctx context.Context, signoutRequest interface{}) error {
	return nil
}

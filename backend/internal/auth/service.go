package auth

import (
	"context"
	"github.com/kdot/k4-chat/backend/internal/database/models"
	"go.uber.org/zap"
)

/*
 	Service is the service layer of the auth endpoint.
	This endpoint will handle various user actions like signin, signup, and signout.
	I plan on adding session support such as SignOutAllDevices

	All data handling, casting, formatting will be done here.
*/

// Service interface
type Service interface {
	SignUpWithOAuth(ctx context.Context, req SignUpRequest) (*models.User, error)
	SignUpWithEmail(ctx context.Context, req SignUpRequest) (*models.User, error)

	SignInWithOAuth(ctx context.Context, req SignInRequest) (*models.User, error)
	SignInWithEmail(ctx context.Context, req SignInRequest) (*models.User, error)

	SignOut(ctx context.Context, req interface{}) error
	SignOutAllDevices(ctx context.Context, req interface{}) error
}

// Service handles authentication operations
type service struct {
	repo   Repository
	logger *zap.Logger
}

// NewService creates a new auth service
func NewService(repo Repository, logger *zap.Logger) Service {
	return &service{
		repo:   repo,
		logger: logger,
	}
}

/*
   ==========================================================================================================
   											SIGN UP
   ==========================================================================================================
*/

// SignUpWithOAuth handles user signup with oauth need to add token logic for write
func (s *service) SignUpWithOAuth(ctx context.Context, req SignUpRequest) (*models.User, error) {
	// validation should've occured within middleware already
	// handle data and write to repository
	// OAUTH is last second
	return nil, nil
}

func (s *service) SignUpWithEmail(ctx context.Context, req SignUpRequest) (*models.User, error) {
	return nil, nil
}

/*
   ==========================================================================================================
   											SIGN IN
   ==========================================================================================================
*/

func (s *service) SignInWithOAuth(ctx context.Context, req SignInRequest) (*models.User, error) {
	return nil, nil
}

func (s *service) SignInWithEmail(ctx context.Context, req SignInRequest) (*models.User, error) {
	return nil, nil
}

/*
   ==========================================================================================================
   											SIGN OUT
   ==========================================================================================================
*/

func (s *service) SignOut(ctx context.Context, req interface{}) error {
	return nil
}

func (s *service) SignOutAllDevices(ctx context.Context, req interface{}) error {
	return nil
}

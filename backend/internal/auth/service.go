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
	// Authentication methods
	SignUpWithOAuth(ctx context.Context, req SignUpRequest) (*models.User, error)
	SignUpWithEmail(ctx context.Context, req SignUpRequest) (*models.User, error)
	SignInWithOAuth(ctx context.Context, req SignInRequest) (*models.User, error)
	SignInWithEmail(ctx context.Context, req SignInRequest) (*models.User, error)

	// Session management
	SignOut(ctx context.Context, sessionToken string) error
	SignOutAllDevices(ctx context.Context, userID string) error
	RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error)
	GetActiveSessions(ctx context.Context, userID string) ([]SessionInfo, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error

	// User validation
	ValidatePassword(password string) error
	ValidateEmail(email string) error
	ValidateUsername(username string) error
}

// Service handles authentication operations
type service struct {
	repo      Repository
	logger    *zap.Logger
	pwHasher  PasswordHasher
	validator InputValidator
}

// NewService creates a new auth service
func NewService(repo Repository, logger *zap.Logger) Service {
	return &service{
		repo:      repo,
		logger:    logger,
		pwHasher:  NewArgon2Hasher(),
		validator: NewDefaultValidator(),
	}
}

// NewServiceWithHasher creates a service with custom password hasher
func NewServiceWithHasher(repo Repository, logger *zap.Logger, hasher PasswordHasher) Service {
	return &service{
		repo:      repo,
		logger:    logger,
		pwHasher:  hasher,
		validator: NewDefaultValidator(),
	}
}

/*
==========================================================================================================
										SIGN UP
==========================================================================================================
*/

// SignUpWithEmail handles user signup with email and password
func (s *service) SignUpWithEmail(ctx context.Context, req SignUpRequest) (*models.User, error) {
	if req.Email == nil {
		return nil, NewValidationError("email", "email signup data is required")
	}

	emailReq := req.Email

	// Validate input
	if err := s.validateEmailSignUpRequest(emailReq); err != nil {
		return nil, err
	}

	// Check if user already exists
	if exists, err := s.repo.UserExistsByEmail(ctx, emailReq.Email); err != nil {
		s.logger.Error("Failed to check if user exists", zap.Error(err))
		return nil, NewAuthError("database_error", "Failed to validate user").WithError(err)
	} else if exists {
		return nil, NewUserAlreadyExistsError("email")
	}

	if exists, err := s.repo.UserExistsByUsername(ctx, emailReq.Username); err != nil {
		s.logger.Error("Failed to check if username exists", zap.Error(err))
		return nil, NewAuthError("database_error", "Failed to validate username").WithError(err)
	} else if exists {
		return nil, NewUserAlreadyExistsError("username")
	}

	// Hash password
	hashedPassword, err := s.pwHasher.HashPassword(emailReq.Password)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, NewAuthError("security_error", "Failed to process password").WithError(err)
	}

	// Create user
	createReq := models.CreateUserRequest{
		Email:       emailReq.Email,
		Username:    emailReq.Username,
		Password:    hashedPassword,
		DisplayName: emailReq.DisplayName,
	}

	user, err := s.repo.CreateUser(ctx, createReq)
	if err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, NewAuthError("database_error", "Failed to create user").WithError(err)
	}

	s.logger.Info("User created successfully",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username),
		zap.String("email", user.Email))

	return user, nil
}

// SignUpWithOAuth handles user signup with OAuth providers
func (s *service) SignUpWithOAuth(ctx context.Context, req SignUpRequest) (*models.User, error) {
	if req.OAuth == nil {
		return nil, NewValidationError("oauth", "OAuth signup data is required")
	}

	// TODO: Implement OAuth signup logic
	// 1. Validate OAuth request
	// 2. Exchange code for token with provider
	// 3. Get user info from provider
	// 4. Create or link user account
	// 5. Generate session tokens

	return nil, NewAuthError("not_implemented", "OAuth signup not yet implemented")
}

/*
==========================================================================================================
										SIGN IN
==========================================================================================================
*/

// SignInWithEmail handles user signin with email and password
func (s *service) SignInWithEmail(ctx context.Context, req SignInRequest) (*models.User, error) {
	if req.Email == nil {
		return nil, NewValidationError("email", "email signin data is required")
	}

	emailReq := req.Email

	// Validate input
	if err := s.validator.ValidateEmail(emailReq.Email); err != nil {
		return nil, NewValidationError("email", err.Error())
	}

	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, emailReq.Email)
	if err != nil {
		s.logger.Warn("Sign in attempt with non-existent email",
			zap.String("email", emailReq.Email))
		return nil, NewInvalidCredentialsError()
	}

	// Check if user is active
	if !user.IsActive {
		return nil, NewAuthError("user_error", "Account is deactivated").
			WithDetails("reason", "account_disabled")
	}

	// Verify password
	if err := s.pwHasher.VerifyPassword(emailReq.Password, user.PasswordHash); err != nil {
		s.logger.Warn("Invalid password attempt",
			zap.String("user_id", user.ID.String()),
			zap.String("email", user.Email))
		return nil, NewInvalidCredentialsError()
	}

	// Update last active timestamp
	if err := s.repo.UpdateUserLastActive(ctx, user.ID); err != nil {
		s.logger.Warn("Failed to update user last active", zap.Error(err))
		// Don't fail the login for this
	}

	s.logger.Info("User signed in successfully",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username))

	return user, nil
}

// SignInWithOAuth handles user signin with OAuth providers
func (s *service) SignInWithOAuth(ctx context.Context, req SignInRequest) (*models.User, error) {
	if req.OAuth == nil {
		return nil, NewValidationError("oauth", "OAuth signin data is required")
	}

	// TODO: Implement OAuth signin logic
	return nil, NewAuthError("not_implemented", "OAuth signin not yet implemented")
}

/*
==========================================================================================================
										SESSION MANAGEMENT
==========================================================================================================
*/

// SignOut invalidates a specific session
func (s *service) SignOut(ctx context.Context, sessionToken string) error {
	// TODO: Implement session invalidation
	return NewAuthError("not_implemented", "SignOut not yet implemented")
}

// SignOutAllDevices invalidates all sessions for a user
func (s *service) SignOutAllDevices(ctx context.Context, userID string) error {
	// TODO: Implement all session invalidation for user
	return NewAuthError("not_implemented", "SignOutAllDevices not yet implemented")
}

// RefreshToken generates new tokens using a refresh token
func (s *service) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// TODO: Implement token refresh logic
	return nil, NewAuthError("not_implemented", "RefreshToken not yet implemented")
}

// GetActiveSessions returns all active sessions for a user
func (s *service) GetActiveSessions(ctx context.Context, userID string) ([]SessionInfo, error) {
	// TODO: Implement get active sessions
	return nil, NewAuthError("not_implemented", "GetActiveSessions not yet implemented")
}

// RevokeSession invalidates a specific session for a user
func (s *service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	// TODO: Implement session revocation
	return NewAuthError("not_implemented", "RevokeSession not yet implemented")
}

/*
==========================================================================================================
										VALIDATION
==========================================================================================================
*/

// ValidatePassword validates password strength
func (s *service) ValidatePassword(password string) error {
	return s.validator.ValidatePassword(password)
}

// ValidateEmail validates email format
func (s *service) ValidateEmail(email string) error {
	return s.validator.ValidateEmail(email)
}

// ValidateUsername validates username format
func (s *service) ValidateUsername(username string) error {
	return s.validator.ValidateUsername(username)
}

// validateEmailSignUpRequest validates email signup request
func (s *service) validateEmailSignUpRequest(req *EmailSignUpRequest) error {
	if err := s.validator.ValidateEmail(req.Email); err != nil {
		return NewValidationError("email", err.Error())
	}

	if err := s.validator.ValidateUsername(req.Username); err != nil {
		return NewValidationError("username", err.Error())
	}

	if err := s.validator.ValidatePassword(req.Password); err != nil {
		return NewValidationError("password", err.Error())
	}

	// Validate display name if provided
	if req.DisplayName != nil && len(*req.DisplayName) > 100 {
		return NewValidationError("display_name", "display name too long")
	}

	return nil
}

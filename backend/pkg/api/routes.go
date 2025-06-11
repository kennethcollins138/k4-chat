package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/kdot/k4-chat/backend/configs"
	"github.com/kdot/k4-chat/backend/internal/auth/sessions"
	"github.com/kdot/k4-chat/backend/internal/auth/tokens"
	"github.com/kdot/k4-chat/backend/pkg/api/middleware"
)

func (s *Server) RegisterRoutes(router chi.Router) {
	// long todo list unfortunately.

	// Initialize configs
	cfg := configs.GetConfig()
	tokenConfig := &tokens.TokenConfig{
		AccessTokenTTL:     cfg.Auth.JWT.AccessTokenTTL,
		RefreshTokenTTL:    cfg.Auth.JWT.RefreshTokenTTL,
		JWTSecret:          cfg.Envs.Auth.AccessTokenSecret,
		EnableTokenBinding: cfg.Auth.Sessions.DeviceBinding,
		EnableRotation:     cfg.Auth.Sessions.EnableRotation,
	}

	// Initialize token/sessage management
	tokenStore := tokens.NewRedisTokenStore(
		s.redis, s.logger, tokenConfig,
	)

	sessionManager := sessions.NewRedisSessionStore(s.redis, s.logger, tokenConfig.RefreshTokenTTL)

	// Initialize Auth middleware
	authMiddleware := middleware.NewAuthMiddleware(sessionManager, nil, tokenStore, nil, s.redis, s.logger)
	// Initialize actor managers/supervisor

	// Initialize auth service and handler

	// Initialize user service and handler

	// Initialize chat service and handler

	router.Route("/auth", func(r chi.Router) {
		// public routes
		r.Group(func(r chi.Router) {
			// Assign middleware like rate limting
			// r.Post("/signup", authHandler.SignUp )
			// r.Post("/signin"), authHandler.SignIn)
			// r.Post("/refresh", authHandler.RefreshToken)
		})
		// private auth routes
		r.Group(func(r chi.Router) {
			// TODO: AUTH AUTHENTICATE middleware
			// r.Post("/signout", authHandler.SignOut)
			// r.Post("/signout-all", authHandler.SignOutAllDevices)
			// r.Get("/sessions", authHandler.GetActiveSessions)
			// r.Delete("/sessions/{sessionID}", authHandler.RevokeSpecificSession)
		})
	})

	router.Route("/user", func(r chi.Router) {
		// user settings, etc
	})

	router.Route("/chat", func(r chi.Router) {
		// CRUD chat, upgrade to websocket here
	})

	// think about billing later a lot of similar middleware
}

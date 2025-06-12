package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/kdot/k4-chat/backend/configs"
	"github.com/kdot/k4-chat/backend/internal/auth"
	"github.com/kdot/k4-chat/backend/internal/auth/coordinator"
	"github.com/kdot/k4-chat/backend/internal/auth/sessions"
	"github.com/kdot/k4-chat/backend/internal/auth/tokens"
	"github.com/kdot/k4-chat/backend/pkg/api/handlers"
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

	// Initialize token/session management
	tokenStore := tokens.NewRedisTokenStore(
		s.redis, s.logger, tokenConfig,
	)

	sessionManager := sessions.NewRedisSessionStore(s.redis, s.logger, tokenConfig.RefreshTokenTTL)
	sessionTokenCoordinator := coordinator.NewSessionTokenCoordinator(
		sessionManager,
		tokenStore,
		s.redis,
		nil, // UserService not implemented yet
		s.logger,
	)

	// Initialize Auth middleware
	authMiddleware := middleware.NewAuthMiddleware(sessionManager, nil, tokenStore, s.pg, s.redis, s.logger)
	// Initialize actor managers/supervisor
	// TODO: Potentially think about passing Supervisor actor as global to service layer.
	// Share PIDs keep services as main business logic for handling req, but offload lot of handling to actors.
	// EXAMPLE: user signs in, I create a UserActor through useractormanager, keeps the buckets and sorts the user actor to necessary bucket,
	// UserActor/Cache is warm and ready for a ws connection, if user's sessions are still valid, but user isn't active tear down instance until they are active again(eg on Token Refresh)

	// Initialize auth service and handler
	authRepo := auth.NewRepository(s.pg, s.logger)
	authService := auth.NewService(authRepo, s.logger)

	// Initialize professional auth handler with coordinator and configuration
	authHandler := handlers.NewAuthHandler(
		authService,
		sessionTokenCoordinator,
		&cfg.Handlers,
		s.logger,
		s.redis,
	)

	// Initialize user service and handler

	// Initialize chat service and handler

	router.Route("/auth", func(r chi.Router) {
		// public routes
		r.Group(func(r chi.Router) {
			// TODO: probably will need some middleare at this level
			r.Post("/signup", authHandler.SignUp)
			r.Post("/signin", authHandler.SignIn)
			r.Post("/refresh", authHandler.RefreshTokens)
		})
		// private auth routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Post("/signout", authHandler.SignOut)
			r.Post("/signout-all", authHandler.SignOutAllDevices)
			r.Get("/sessions", authHandler.GetActiveSessions)
			r.Delete("/sessions/{sessionID}", authHandler.RevokeSpecificSession)
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

package api

import (
	"github.com/docker/docker/api/server/router"
	"github.com/go-chi/chi/v5"
)

func (s *Server) RegisterRoutes(r chi.Router) {
	// long todo list unfortunately.

	// Initialize configs

	// Initialize token/sessage management

	// Initialize Auth middleware

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

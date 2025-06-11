package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kdot/k4-chat/backend/configs"
	"github.com/kdot/k4-chat/backend/internal/database"
	"github.com/kdot/k4-chat/backend/pkg/api/middleware"
)

/*
Server Repepresents the API server interface.
It encapsulates all necessary components for handling http requests.

Plans:
- Add support for websockets
- Add support for grpc

Fields:
  - cfg: Server config initialized in main.go to pass configs into the server
  - logger: Global logger initialized in main.go that will be DI to different handlers.
  - pg: Postgres Pool Db for storage/persistance
  - redis: Redis client for caching/rate_limiting/sessions etc.
  - srv: Actual Server itself
*/
type Server struct {
	cfg    *configs.ServerConfig
	logger *zap.Logger
	pg     *database.DB
	redis  *redis.Client
	srv    *http.Server
}

/*
NewServer created and initializes a new server instance.
The server is configured but not started. Call the Run() method to begin accepting requests.
*/
func NewServer(config *configs.ServerConfig, logger *zap.Logger, db *database.DB, redis *redis.Client) *Server {
	return &Server{
		cfg:    config,
		logger: logger,
		pg:     db,
		redis:  redis,
	}
}

/*
Run starts the API server and begins accepting HTTP requests
It initializes all necessary components, routes, and starts the configured server.

This method will block until the server server is stopped or encounters a fatal error.
*/
func (s *Server) Run() error {
	s.srv = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.Port),
		Handler:           s.Routes(),
		ReadTimeout:       s.cfg.ReadTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		MaxHeaderBytes:    s.cfg.MaxHeaderBytes,
	}

	banner := `
 _  ___  _          ____ _   _    _  _____ 
| |/ / || |        / ___| | | |  / \|_   _|
| ' /| || |_ _____| |   | |_| | / _ \ | |  
| . \|__   _|_____| |___|  _  |/ ___ \| |  
|_|\_\  |_|        \____|_| |_/_/   \_\_|  


	`
	s.logger.Info(fmt.Sprintf("Api Server started at port: %d", s.cfg.Port))
	s.logger.Info(banner)
	return nil
}

func (s *Server) Routes() http.Handler {
	router := chi.NewRouter()
	config := configs.GetConfig()
	// TODO: add security middleware (needs to adopt new config logic)
	// Middleware for metrics, logging, recovery, and timeouts should be here
	router.Use(
		chiMiddleware.RequestID,
		chiMiddleware.RealIP,
		chiMiddleware.Recoverer,
		chiMiddleware.Timeout(s.cfg.IdleTimeout),
		middleware.SecurityMiddleware(
			&config.Middleware, s.logger,
		),
	)

	router.Route("api/v1", func(r chi.Router) {
		// add some type of activity tracking middleware here for prometheus
		// initialize metrics endpoint
		s.RegisterRoutes(r)
	})
	return router
}

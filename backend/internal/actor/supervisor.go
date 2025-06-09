package actor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/kdot/k4-chat/backend/internal/database"
)

/*
FILE NEEDS:
 - TODO: Supervision logic: actor failure, restart, health check, etc.
 - TODO: Logging  not just fmt.Println
 - TODO: Configuration
 - TODO: Unit Testing
 - TODO: Health Checking
 - TODO: Concurrency/Race checks
 - TODO: Documentation

Would be nice before sprint is over:
  - Structured logging specifically error context
  - Configuration Validation, yaml files with loader would be great
  - Metrics (prometheus would be amazing)
  - Circuit Breaker would be nice (need to wait until we have actual LLM and db connections)
  - Backoff strategies would be great as well
*/

/*
	Next Steps:
		- Will write docs for this and general unit tests for this file
		- moving to Managers strating specifically with UserManagerActor
		- need to setup message bus as well
*/

// SupervisorConfig defines configuration for the supervisor
type SupervisorConfig struct {
	MaxRestarts     int           // Maximum restarts within the restart window
	RestartWindow   time.Duration // Time window for restart counting
	ShutdownTimeout time.Duration // Timeout for graceful shutdown
}

// DefaultSupervisorConfig returns sensible defaults
func DefaultSupervisorConfig() SupervisorConfig {
	return SupervisorConfig{
		MaxRestarts:     5,
		RestartWindow:   time.Minute,
		ShutdownTimeout: 30 * time.Second,
	}
}

// ChildActorInfo tracks information about child actors
type ChildActorInfo struct {
	PID         *actor.PID
	Name        string
	StartTime   time.Time
	Restarts    int
	LastRestart time.Time
	Producer    actor.Producer
}

// SupervisorActor is the root actor that manages the lifecycle of all system actors
type SupervisorActor struct {
	config   SupervisorConfig
	children map[string]*ChildActorInfo
	mu       sync.RWMutex

	// Child actor PIDs
	userManager  *actor.PID
	llmManager   *actor.PID
	toolsManager *actor.PID

	// Shutdown coordination
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	isShuttingDown bool
}

// NewSupervisorActor creates a new supervisor with configuration
func NewSupervisorActor(config SupervisorConfig) actor.Producer {
	return func() actor.Receiver {
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
		return &SupervisorActor{
			config:         config,
			children:       make(map[string]*ChildActorInfo),
			shutdownCtx:    shutdownCtx,
			shutdownCancel: shutdownCancel,
		}
	}
}

// Receive handles all messages sent to the supervisor
func (s *SupervisorActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Initialized:
		// Handle actor initialization
		log.Println("SupervisorActor: Initialized")

	case actor.Initialized:
		// Handle actor initialization (non-pointer version)
		log.Println("SupervisorActor: Initialized (non-pointer)")

	case *actor.Started:
		s.handleStarted(ctx)

	case actor.Started:
		s.handleStarted(ctx)

	case *actor.Stopped:
		s.handleStopped(ctx)

	case actor.Stopped:
		s.handleStopped(ctx)

	case *ShutdownRequest:
		s.handleShutdownRequest(ctx, msg)

	case *HealthCheckRequest:
		s.handleHealthCheck(ctx, msg)

	case *GetSystemStatusRequest:
		s.handleSystemStatus(ctx, msg)

	default:
		log.Printf("SupervisorActor: Unknown message type: %T", msg)
	}
}

// handleStarted initializes the supervisor and spawns child actors
func (s *SupervisorActor) handleStarted(ctx *actor.Context) {
	log.Println("SupervisorActor: Starting system...")

	// Start child actors in order of dependency
	s.spawnChildActors(ctx)

	log.Println("SupervisorActor: System startup complete")
}

// spawnChildActors creates all the main system actors
func (s *SupervisorActor) spawnChildActors(ctx *actor.Context) {
	// TODO: Initialize database connection here
	// For now, we'll pass nil - this should be injected via dependency injection
	var db *database.DB = nil

	// Spawn UserManagerActor
	s.userManager = s.spawnChild(ctx, "user-manager", NewUserManagerActor(db))

	// Spawn LLMManagerActor
	s.llmManager = s.spawnChild(ctx, "llm-manager", func() actor.Receiver {
		return &LLMManagerActor{}
	})

	// Spawn ToolsManagerActor
	s.toolsManager = s.spawnChild(ctx, "tools-manager", func() actor.Receiver {
		return &ToolsManagerActor{}
	})
}

// spawnChild spawns a child actor with tracking
func (s *SupervisorActor) spawnChild(ctx *actor.Context, name string, producer actor.Producer) *actor.PID {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Spawn the child actor
	pid := ctx.SpawnChild(producer, name)

	// Track the child
	s.children[name] = &ChildActorInfo{
		PID:       pid,
		Name:      name,
		StartTime: time.Now(),
		Restarts:  0,
		Producer:  producer,
	}

	log.Printf("SupervisorActor: Spawned child actor: %s (PID: %s)", name, pid.GetID())
	return pid
}

// handleHealthCheck processes health check requests
func (s *SupervisorActor) handleHealthCheck(ctx *actor.Context, msg *HealthCheckRequest) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &HealthCheckResponse{
		Healthy:   true,
		Children:  make(map[string]ChildHealth),
		Timestamp: time.Now(),
	}

	// Check each child actor
	for name, child := range s.children {
		uptime := time.Since(child.StartTime)
		status.Children[name] = ChildHealth{
			Name:     name,
			Healthy:  true, // Basic implementation - could ping child actors
			Uptime:   uptime,
			Restarts: child.Restarts,
		}
	}

	// Send response back
	ctx.Respond(status)
}

// handleSystemStatus provides detailed system information
func (s *SupervisorActor) handleSystemStatus(ctx *actor.Context, msg *GetSystemStatusRequest) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SystemStatusResponse{
		Timestamp:      time.Now(),
		TotalChildren:  len(s.children),
		IsShuttingDown: s.isShuttingDown,
		Children:       make([]ChildActorInfo, 0, len(s.children)),
	}

	for _, child := range s.children {
		status.Children = append(status.Children, *child)
	}

	ctx.Respond(status)
}

// handleShutdownRequest initiates graceful shutdown
func (s *SupervisorActor) handleShutdownRequest(ctx *actor.Context, msg *ShutdownRequest) {
	s.initiateShutdown(ctx, nil)
}

// initiateShutdown performs graceful shutdown of all child actors
func (s *SupervisorActor) initiateShutdown(ctx *actor.Context, reason error) {
	s.mu.Lock()
	s.isShuttingDown = true
	s.mu.Unlock()

	if reason != nil {
		log.Printf("SupervisorActor: Initiating shutdown due to: %v", reason)
	} else {
		log.Println("SupervisorActor: Initiating graceful shutdown")
	}

	// Cancel shutdown context
	s.shutdownCancel()

	// Note: In a basic implementation, we rely on the actor system to handle cleanup
	log.Println("SupervisorActor: Shutdown initiated")
}

// handleStopped cleans up when the supervisor is stopped
func (s *SupervisorActor) handleStopped(ctx *actor.Context) {
	log.Println("SupervisorActor: Stopped")
	s.shutdownCancel()
}

// GetUserManager returns the user manager PID for external access
func (s *SupervisorActor) GetUserManager() *actor.PID {
	return s.userManager
}

// GetLLMManager returns the LLM manager PID for external access
func (s *SupervisorActor) GetLLMManager() *actor.PID {
	return s.llmManager
}

// GetToolsManager returns the tools manager PID for external access
func (s *SupervisorActor) GetToolsManager() *actor.PID {
	return s.toolsManager
}

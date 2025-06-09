package actor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/kdot/k4-chat/backend/internal/database"
	"github.com/kdot/k4-chat/backend/internal/database/models"
	"github.com/kdot/k4-chat/backend/internal/utils"
)

/*
UserManagerActor manages the lifecycle of all UserActors in the system.

Key Responsibilities:
- UserActor lifecycle management (spawn, monitor, shutdown)
- Connection routing to appropriate UserActors
- User creation and authentication
- Graceful shutdown coordination
- Health monitoring and metrics collection
- Resource cleanup and idle UserActor management

Relationships:
SupervisorActor -> UserManagerActor (1:1)
UserManagerActor -> UserActor (1:N users)
UserActor -> ConnectionActor (1:N connections per user)

Design Notes:
- Maintains a registry of active UserActors
- Handles graceful shutdown by coordinating with all UserActors
- Monitors UserActor health and handles failures
- Provides metrics and status information
- Manages idle UserActor cleanup to prevent resource leaks
*/

// DefaultUserManagerConfig returns sensible defaults
func DefaultUserManagerConfig() UserManagerConfig {
	return UserManagerConfig{
		MaxActiveUsers:      1000,
		IdleTimeout:         30 * time.Minute,
		ShutdownTimeout:     30 * time.Second,
		HealthCheckInterval: 2 * time.Minute,
		CleanupInterval:     5 * time.Minute,
	}
}

// UserActorInfo tracks information about managed UserActors
type UserActorInfo struct {
	PID               *actor.PID
	UserID            string
	SpawnedAt         time.Time
	LastActiveAt      time.Time
	ConnectionCount   int
	SessionCount      int
	IsHealthy         bool
	ShutdownRequested bool
}

// UserManagerActor manages user lifecycle and operations
type UserManagerActor struct {
	config  UserManagerConfig
	db      *database.DB
	users   map[string]*UserActorInfo // map of userID to UserActor info
	mu      sync.RWMutex              // protects users map
	metrics UserManagerMetrics

	// Shutdown coordination
	isShuttingDown bool
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// Background tasks
	healthCheckTicker *time.Ticker
	cleanupTicker     *time.Ticker
}

// NewUserManagerActor creates a new user manager actor with configuration
func NewUserManagerActor(db *database.DB) actor.Producer {
	return func() actor.Receiver {
		config := DefaultUserManagerConfig()
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

		return &UserManagerActor{
			config:         config,
			db:             db,
			users:          make(map[string]*UserActorInfo),
			shutdownCtx:    shutdownCtx,
			shutdownCancel: shutdownCancel,
		}
	}
}

// NewUserManagerActorWithConfig creates a user manager with custom configuration
func NewUserManagerActorWithConfig(db *database.DB, config UserManagerConfig) actor.Producer {
	return func() actor.Receiver {
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

		return &UserManagerActor{
			config:         config,
			db:             db,
			users:          make(map[string]*UserActorInfo),
			shutdownCtx:    shutdownCtx,
			shutdownCancel: shutdownCancel,
		}
	}
}

// Receive handles messages for user management
func (um *UserManagerActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		um.handleStarted(ctx)

	case *actor.Stopped:
		um.handleStopped(ctx)

	case *CreateUserRequest:
		um.handleCreateUser(ctx, msg)

	case *GetUserRequest:
		um.handleGetUser(ctx, msg)

	case *UpdateUserLastActiveRequest:
		um.handleUpdateUserLastActive(ctx, msg)

	case *UserConnectionEstablished:
		um.handleUserConnection(ctx, msg)

	case *UserConnectionClosed:
		um.handleUserDisconnection(ctx, msg)

	case *ShutdownRequest:
		um.handleShutdownRequest(ctx, msg)

	case *GetUserManagerStatusRequest:
		um.handleGetStatus(ctx, msg)

	case *GetUserManagerMetricsRequest:
		um.handleGetMetrics(ctx, msg)

	case *CleanupIdleUsersRequest:
		um.handleCleanupIdleUsers(ctx, msg)

	case *UserStateUpdate:
		um.handleUserStateUpdate(ctx, msg)

	case *HealthCheckTick:
		um.handleHealthCheckTick(ctx)

	case *CleanupTick:
		um.handleCleanupTick(ctx)

	default:
		log.Printf("UserManagerActor: Unknown message type: %T", msg)
	}
}

// handleStarted initializes the user manager
func (um *UserManagerActor) handleStarted(ctx *actor.Context) {
	log.Println("UserManagerActor: Started")

	// Start background tasks
	um.startBackgroundTasks(ctx)

	log.Printf("UserManagerActor: Initialized with config - MaxUsers: %d, IdleTimeout: %v",
		um.config.MaxActiveUsers, um.config.IdleTimeout)
}

// handleStopped cleans up when the user manager is stopped
func (um *UserManagerActor) handleStopped(ctx *actor.Context) {
	log.Println("UserManagerActor: Stopping...")

	// Cancel shutdown context
	um.shutdownCancel()

	// Stop background tasks
	um.stopBackgroundTasks()

	// Force stop all remaining UserActors
	um.mu.Lock()
	for userID, userInfo := range um.users {
		if userInfo.PID != nil {
			log.Printf("UserManagerActor: Force stopping UserActor %s", userID)
			ctx.Engine().Stop(userInfo.PID)
		}
	}
	um.mu.Unlock()

	log.Println("UserManagerActor: Stopped")
}

// handleCreateUser creates a new user in the database with proper password hashing
func (um *UserManagerActor) handleCreateUser(ctx *actor.Context, msg *CreateUserRequest) {
	// Hash password before storing
	hashedPassword, err := utils.HashPassword(msg.Password)
	if err != nil {
		log.Printf("UserManagerActor: Failed to hash password: %v", err)
		ctx.Respond(&CreateUserResponse{
			User:  nil,
			Error: fmt.Errorf("failed to process password: %w", err),
		})
		return
	}

	// Convert actor message to database request
	dbReq := models.CreateUserRequest{
		Email:       msg.Email,
		Username:    msg.Username,
		Password:    hashedPassword,
		DisplayName: msg.DisplayName,
	}

	// Create user in database
	user, err := um.db.CreateUser(ctx.Context(), dbReq)

	// Send response back to sender
	response := &CreateUserResponse{
		User:  user,
		Error: err,
	}
	ctx.Respond(response)

	// If user creation was successful, spawn a UserActor for this user
	if err == nil && user != nil {
		um.spawnUserActor(ctx, user.ID.String())
		um.metrics.UsersCreated++
		log.Printf("UserManagerActor: Created user %s (%s)", user.Username, user.ID.String())
	}
}

// handleGetUser retrieves a user from the database
func (um *UserManagerActor) handleGetUser(ctx *actor.Context, msg *GetUserRequest) {
	user, err := um.db.GetUserByID(ctx.Context(), msg.UserID)

	response := &GetUserResponse{
		User:  user,
		Error: err,
	}
	ctx.Respond(response)
}

// handleUpdateUserLastActive updates user's last active timestamp
func (um *UserManagerActor) handleUpdateUserLastActive(ctx *actor.Context, msg *UpdateUserLastActiveRequest) {
	err := um.db.UpdateUserLastActive(ctx.Context(), msg.UserID)

	response := &UpdateUserLastActiveResponse{
		Error: err,
	}
	ctx.Respond(response)

	// Update our tracking
	um.updateUserActivity(msg.UserID.String())
}

// handleUserConnection establishes a connection for a user
func (um *UserManagerActor) handleUserConnection(ctx *actor.Context, msg *UserConnectionEstablished) {
	// TODO: This will need to be enhanced when WebSocket layer is ready
	// For now, we need the userID to be provided somehow (e.g., from WebSocket auth context)
	log.Printf("UserManagerActor: User connection established - ConnectionID: %s", msg.ConnectionID)

	// Example of how this would work with WebSocket context:
	// userID := extractUserIDFromWebSocketContext(msg.ConnectionID)
	// userPID := um.getUserActor(ctx, userID)
	// if userPID != nil {
	//     ctx.Send(userPID, msg)
	// }
}

// handleUserDisconnection handles user disconnection
func (um *UserManagerActor) handleUserDisconnection(ctx *actor.Context, msg *UserConnectionClosed) {
	// TODO: Similar to above, need to map connectionID to userID
	log.Printf("UserManagerActor: User connection closed - ConnectionID: %s", msg.ConnectionID)

	// Example:
	// userID := extractUserIDFromConnection(msg.ConnectionID)
	// userPID := um.getUserActor(ctx, userID)
	// if userPID != nil {
	//     ctx.Send(userPID, msg)
	// }
}

// handleShutdownRequest initiates graceful shutdown of all UserActors
func (um *UserManagerActor) handleShutdownRequest(ctx *actor.Context, msg *ShutdownRequest) {
	log.Printf("UserManagerActor: Graceful shutdown requested: %s", msg.Reason)

	um.mu.Lock()
	um.isShuttingDown = true
	um.mu.Unlock()

	// Cancel background tasks
	um.shutdownCancel()

	// Send shutdown requests to all UserActors
	var shutdownWG sync.WaitGroup
	um.mu.RLock()
	activeUsers := make([]*UserActorInfo, 0, len(um.users))
	for _, userInfo := range um.users {
		if userInfo.PID != nil && !userInfo.ShutdownRequested {
			activeUsers = append(activeUsers, userInfo)
		}
	}
	um.mu.RUnlock()

	log.Printf("UserManagerActor: Initiating shutdown for %d UserActors", len(activeUsers))

	for _, userInfo := range activeUsers {
		shutdownWG.Add(1)
		userInfo.ShutdownRequested = true

		go func(info *UserActorInfo) {
			defer shutdownWG.Done()

			// Create timeout context for this specific user shutdown
			shutdownCtx, cancel := context.WithTimeout(context.Background(), um.config.ShutdownTimeout)
			defer cancel()

			// Send shutdown request to UserActor
			// Note: In a more complete implementation, we'd have a specific UserShutdownRequest message
			log.Printf("UserManagerActor: Requesting shutdown for UserActor %s", info.UserID)

			// For now, we'll just stop the actor directly
			// In a production system, you'd send a graceful shutdown message first
			ctx.Engine().Stop(info.PID)

			select {
			case <-shutdownCtx.Done():
				log.Printf("UserManagerActor: Timeout waiting for UserActor %s shutdown", info.UserID)
			case <-time.After(100 * time.Millisecond):
				// Assume shutdown completed
				log.Printf("UserManagerActor: UserActor %s shutdown completed", info.UserID)
			}
		}(userInfo)
	}

	// Wait for all UserActors to shutdown with overall timeout
	done := make(chan struct{})
	go func() {
		shutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("UserManagerActor: All UserActors shutdown gracefully")
	case <-time.After(um.config.ShutdownTimeout + 5*time.Second):
		log.Println("UserManagerActor: Shutdown timeout exceeded, some UserActors may not have stopped gracefully")
	}

	// Clean up registry
	um.mu.Lock()
	um.users = make(map[string]*UserActorInfo)
	um.mu.Unlock()

	log.Println("UserManagerActor: Graceful shutdown completed")
}

// handleGetStatus returns current manager status
func (um *UserManagerActor) handleGetStatus(ctx *actor.Context, msg *GetUserManagerStatusRequest) {
	um.mu.RLock()
	defer um.mu.RUnlock()

	activeUsers := make([]UserActorStatus, 0, len(um.users))
	totalConnections := 0
	totalSessions := 0

	for _, userInfo := range um.users {
		activeUsers = append(activeUsers, UserActorStatus{
			UserID:            userInfo.UserID,
			SpawnedAt:         userInfo.SpawnedAt,
			LastActiveAt:      userInfo.LastActiveAt,
			ConnectionCount:   userInfo.ConnectionCount,
			SessionCount:      userInfo.SessionCount,
			IsHealthy:         userInfo.IsHealthy,
			ShutdownRequested: userInfo.ShutdownRequested,
		})
		totalConnections += userInfo.ConnectionCount
		totalSessions += userInfo.SessionCount
	}

	response := &UserManagerStatusResponse{
		Timestamp:        time.Now(),
		TotalActiveUsers: len(um.users),
		TotalConnections: totalConnections,
		TotalSessions:    totalSessions,
		IsShuttingDown:   um.isShuttingDown,
		ActiveUsers:      activeUsers,
		Config:           um.config,
	}

	ctx.Respond(response)
}

// handleGetMetrics returns current metrics
func (um *UserManagerActor) handleGetMetrics(ctx *actor.Context, msg *GetUserManagerMetricsRequest) {
	um.mu.RLock()
	metrics := um.metrics
	metrics.TotalActiveUsers = len(um.users)

	totalConnections := 0
	totalSessions := 0
	for _, userInfo := range um.users {
		totalConnections += userInfo.ConnectionCount
		totalSessions += userInfo.SessionCount
	}
	metrics.TotalConnections = totalConnections
	metrics.TotalSessions = totalSessions
	um.mu.RUnlock()

	response := &UserManagerMetricsResponse{
		Timestamp: time.Now(),
		Metrics:   metrics,
	}

	ctx.Respond(response)
}

// handleCleanupIdleUsers removes idle UserActors to free resources
func (um *UserManagerActor) handleCleanupIdleUsers(ctx *actor.Context, msg *CleanupIdleUsersRequest) {
	if um.isShuttingDown {
		return
	}

	um.mu.Lock()
	defer um.mu.Unlock()

	now := time.Now()
	idleUsers := make([]*UserActorInfo, 0)

	for userID, userInfo := range um.users {
		// Check if user is idle (no connections and past idle timeout)
		if userInfo.ConnectionCount == 0 &&
			now.Sub(userInfo.LastActiveAt) > um.config.IdleTimeout {
			idleUsers = append(idleUsers, userInfo)
			log.Printf("UserManagerActor: Marking idle UserActor %s for cleanup (idle for %v)",
				userID, now.Sub(userInfo.LastActiveAt))
		}
	}

	// Clean up idle users
	for _, userInfo := range idleUsers {
		if userInfo.PID != nil {
			log.Printf("UserManagerActor: Cleaning up idle UserActor %s", userInfo.UserID)
			ctx.Engine().Stop(userInfo.PID)
			delete(um.users, userInfo.UserID)
			um.metrics.UsersShutdown++
			um.metrics.CleanupOperations++
		}
	}

	if len(idleUsers) > 0 {
		log.Printf("UserManagerActor: Cleaned up %d idle UserActors", len(idleUsers))
	}
}

// handleUserStateUpdate updates tracking information for a UserActor
func (um *UserManagerActor) handleUserStateUpdate(ctx *actor.Context, msg *UserStateUpdate) {
	um.mu.Lock()
	defer um.mu.Unlock()

	if userInfo, exists := um.users[msg.UserID]; exists {
		userInfo.LastActiveAt = msg.LastActiveAt
		userInfo.ConnectionCount = msg.ConnectionCount
		userInfo.SessionCount = msg.SessionCount
		userInfo.IsHealthy = true

		log.Printf("UserManagerActor: Updated state for UserActor %s - Connections: %d, Sessions: %d",
			msg.UserID, msg.ConnectionCount, msg.SessionCount)
	}
}

// handleHealthCheckTick performs health checks on all UserActors
func (um *UserManagerActor) handleHealthCheckTick(ctx *actor.Context) {
	if um.isShuttingDown {
		return
	}

	um.mu.RLock()
	userActors := make([]*UserActorInfo, 0, len(um.users))
	for _, userInfo := range um.users {
		if userInfo.PID != nil && !userInfo.ShutdownRequested {
			userActors = append(userActors, userInfo)
		}
	}
	um.mu.RUnlock()

	for _, userInfo := range userActors {
		// Send health check request to UserActor
		// This would be a ping/health check message
		go func(info *UserActorInfo) {
			healthReq := &GetUserInfoRequest{}
			future := ctx.Engine().Request(info.PID, healthReq, 5*time.Second)

			if _, err := future.Result(); err != nil {
				log.Printf("UserManagerActor: Health check failed for UserActor %s: %v", info.UserID, err)
				um.mu.Lock()
				info.IsHealthy = false
				um.metrics.HealthChecksFailed++
				um.mu.Unlock()
			} else {
				um.mu.Lock()
				info.IsHealthy = true
				um.mu.Unlock()
			}
		}(userInfo)
	}
}

// handleCleanupTick runs periodic cleanup tasks
func (um *UserManagerActor) handleCleanupTick(ctx *actor.Context) {
	// Trigger idle user cleanup
	ctx.Send(ctx.PID(), &CleanupIdleUsersRequest{})
}

// spawnUserActor creates a new UserActor for a user
func (um *UserManagerActor) spawnUserActor(ctx *actor.Context, userID string) *actor.PID {
	um.mu.Lock()
	defer um.mu.Unlock()

	// Check if user actor already exists (ensure consistency)
	if existingInfo, exists := um.users[userID]; exists {
		log.Printf("UserManagerActor: UserActor for %s already exists", userID)
		return existingInfo.PID
	}

	// Check if we're at capacity
	if len(um.users) >= um.config.MaxActiveUsers {
		log.Printf("UserManagerActor: Cannot spawn UserActor for %s - at capacity (%d/%d)",
			userID, len(um.users), um.config.MaxActiveUsers)
		return nil
	}

	// Spawn new user actor with consistent naming
	userPID := ctx.SpawnChild(NewUserActor(userID, um.db), "user-"+userID)

	// Track the new UserActor
	now := time.Now()
	um.users[userID] = &UserActorInfo{
		PID:               userPID,
		UserID:            userID,
		SpawnedAt:         now,
		LastActiveAt:      now,
		ConnectionCount:   0,
		SessionCount:      0,
		IsHealthy:         true,
		ShutdownRequested: false,
	}

	log.Printf("UserManagerActor: Spawned UserActor for user %s (%d/%d active)",
		userID, len(um.users), um.config.MaxActiveUsers)
	return userPID
}

// getUserActor returns the PID for a user's actor, spawning one if needed
func (um *UserManagerActor) getUserActor(ctx *actor.Context, userID string) *actor.PID {
	um.mu.RLock()
	if userInfo, exists := um.users[userID]; exists {
		um.mu.RUnlock()
		return userInfo.PID
	}
	um.mu.RUnlock()

	return um.spawnUserActor(ctx, userID)
}

// updateUserActivity updates the last active time for a user
func (um *UserManagerActor) updateUserActivity(userID string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	if userInfo, exists := um.users[userID]; exists {
		userInfo.LastActiveAt = time.Now()
	}
}

// startBackgroundTasks starts periodic maintenance tasks
func (um *UserManagerActor) startBackgroundTasks(ctx *actor.Context) {
	// Start health check ticker
	um.healthCheckTicker = time.NewTicker(um.config.HealthCheckInterval)
	go func() {
		for {
			select {
			case <-um.healthCheckTicker.C:
				ctx.Engine().Send(ctx.PID(), &HealthCheckTick{})
			case <-um.shutdownCtx.Done():
				return
			}
		}
	}()

	// Start cleanup ticker
	um.cleanupTicker = time.NewTicker(um.config.CleanupInterval)
	go func() {
		for {
			select {
			case <-um.cleanupTicker.C:
				ctx.Engine().Send(ctx.PID(), &CleanupTick{})
			case <-um.shutdownCtx.Done():
				return
			}
		}
	}()

	log.Printf("UserManagerActor: Started background tasks - HealthCheck: %v, Cleanup: %v",
		um.config.HealthCheckInterval, um.config.CleanupInterval)
}

// stopBackgroundTasks stops all background tasks
func (um *UserManagerActor) stopBackgroundTasks() {
	if um.healthCheckTicker != nil {
		um.healthCheckTicker.Stop()
		um.healthCheckTicker = nil
	}

	if um.cleanupTicker != nil {
		um.cleanupTicker.Stop()
		um.cleanupTicker = nil
	}

	log.Println("UserManagerActor: Stopped background tasks")
}

// GetActiveUsersCount returns the number of active users (with spawned actors)
func (um *UserManagerActor) GetActiveUsersCount() int {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return len(um.users)
}

// GetActiveUsers returns a list of active user IDs
func (um *UserManagerActor) GetActiveUsers() []string {
	um.mu.RLock()
	defer um.mu.RUnlock()

	users := make([]string, 0, len(um.users))
	for userID := range um.users {
		users = append(users, userID)
	}
	return users
}

// GetUserActorPID returns the PID for a specific user (if exists)
func (um *UserManagerActor) GetUserActorPID(userID string) *actor.PID {
	um.mu.RLock()
	defer um.mu.RUnlock()

	if userInfo, exists := um.users[userID]; exists {
		return userInfo.PID
	}
	return nil
}

// IsShuttingDown returns whether the manager is shutting down
func (um *UserManagerActor) IsShuttingDown() bool {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.isShuttingDown
}

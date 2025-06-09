package actor

import (
	"context"
	"log"

	"github.com/anthdm/hollywood/actor"
	"github.com/kdot/k4-chat/backend/internal/database"
	"github.com/kdot/k4-chat/backend/internal/database/models"
)

/*
UserManagerActor represents a single the manager/factory for creating UserActors.

Key Responsibilities:
- Managing connection support for single user
	- Controlling UserActor lifecycle
	- Connection is delegated to Connection actors
	- Chat sync is delegated to SyncActor

Relationships:
SupervisorActor -> UserManagerActor (1:1)
	- This relationship can potentially be distributed and have multiple managers, but for now 1:1
UserManagerActor -> UserActor (1:n users)
*/

// UserManagerActor manages user lifecycle and operations
type UserManagerActor struct {
	db    *database.DB
	users map[string]*actor.PID // map of userID to user actor PID
}

// NewUserManagerActor creates a new user manager actor
func NewUserManagerActor(db *database.DB) actor.Producer {
	return func() actor.Receiver {
		return &UserManagerActor{
			db:    db,
			users: make(map[string]*actor.PID),
		}
	}
}

// Receive handles messages for user management
func (um *UserManagerActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		log.Println("UserManagerActor: Started")

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

	default:
		log.Printf("UserManagerActor: Unknown message type: %T", msg)
	}
}

// handleCreateUser creates a new user in the database
func (um *UserManagerActor) handleCreateUser(ctx *actor.Context, msg *CreateUserRequest) {
	// Convert actor message to database request
	dbReq := models.CreateUserRequest{
		Email:       msg.Email,
		Username:    msg.Username,
		Password:    msg.Password, // TODO: Hash password before storing
		DisplayName: msg.DisplayName,
	}

	// Create user in database
	user, err := um.db.CreateUser(context.Background(), dbReq)

	// Send response back to sender
	response := &CreateUserResponse{
		User:  user,
		Error: err,
	}

	ctx.Respond(response)

	// If user creation was successful, spawn a UserActor for this user
	if err == nil && user != nil {
		um.spawnUserActor(ctx, user.ID.String())
		log.Printf("UserManagerActor: Created user %s (%s)", user.Username, user.ID.String())
	}
}

// handleGetUser retrieves a user from the database
func (um *UserManagerActor) handleGetUser(ctx *actor.Context, msg *GetUserRequest) {
	user, err := um.db.GetUserByID(context.Background(), msg.UserID)

	response := &GetUserResponse{
		User:  user,
		Error: err,
	}

	ctx.Respond(response)
}

// handleUpdateUserLastActive updates user's last active timestamp
func (um *UserManagerActor) handleUpdateUserLastActive(ctx *actor.Context, msg *UpdateUserLastActiveRequest) {
	err := um.db.UpdateUserLastActive(context.Background(), msg.UserID)

	response := &UpdateUserLastActiveResponse{
		Error: err,
	}

	ctx.Respond(response)
}

// handleUserConnection establishes a connection for a user
func (um *UserManagerActor) handleUserConnection(ctx *actor.Context, msg *UserConnectionEstablished) {
	// This would typically be sent by a WebSocket connection handler
	// For now, we'll just log it
	log.Printf("UserManagerActor: User connection established - ConnectionID: %s", msg.ConnectionID)

	// TODO: Forward to appropriate UserActor
	// userPID := um.getUserActor(userID)
	// if userPID != nil {
	//     ctx.Send(userPID, msg)
	// }
}

// handleUserDisconnection handles user disconnection
func (um *UserManagerActor) handleUserDisconnection(ctx *actor.Context, msg *UserConnectionClosed) {
	log.Printf("UserManagerActor: User connection closed - ConnectionID: %s", msg.ConnectionID)

	// TODO: Forward to appropriate UserActor
}

// spawnUserActor creates a new UserActor for a user
func (um *UserManagerActor) spawnUserActor(ctx *actor.Context, userID string) *actor.PID {
	// Check if user actor already exists
	if existingPID, exists := um.users[userID]; exists {
		return existingPID
	}

	// Spawn new user actor
	userPID := ctx.SpawnChild(NewUserActor(userID, um.db), "user-"+userID)
	um.users[userID] = userPID

	log.Printf("UserManagerActor: Spawned UserActor for user %s", userID)
	return userPID
}

// getUserActor returns the PID for a user's actor, spawning one if needed
func (um *UserManagerActor) getUserActor(ctx *actor.Context, userID string) *actor.PID {
	if pid, exists := um.users[userID]; exists {
		return pid
	}
	return um.spawnUserActor(ctx, userID)
}

// GetActiveUsersCount returns the number of active users (with spawned actors)
func (um *UserManagerActor) GetActiveUsersCount() int {
	return len(um.users)
}

// GetActiveUsers returns a list of active user IDs
func (um *UserManagerActor) GetActiveUsers() []string {
	users := make([]string, 0, len(um.users))
	for userID := range um.users {
		users = append(users, userID)
	}
	return users
}

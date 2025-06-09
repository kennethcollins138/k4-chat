package actor

import (
	"log"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/kdot/k4-chat/backend/internal/database"
)

/*
UserActor represents a single user and manages all their connections, chat sessions, and operations.

Key Responsibilities:
- Managing multiple connections (multi-device support)
- Managing chat sessions and their lifecycle
- Coordinating message flow between connections and sessions
- Handling user-specific operations (create sessions, send messages, etc.)
- Broadcasting updates to all user connections

Relationships:
UserManagerActor -> UserActor (1:N users)
UserActor -> ConnectionActor (1:N connections per user)
UserActor -> ChatSessionActor (1:N chat sessions per user)
*/
type UserActor struct {
	userID       string
	db           *database.DB
	connections  map[string]*actor.PID // map of connectionID to connection actor PID
	chatSessions map[string]*actor.PID // map of chatSessionID to chatSession actor PID
	syncActor    *actor.PID            // actor that handles sync operations for the user

	// User state tracking
	lastActiveAt time.Time
	isOnline     bool
}

// NewUserActor creates a new user actor
func NewUserActor(userID string, db *database.DB) actor.Producer {
	return func() actor.Receiver {
		return &UserActor{
			userID:       userID,
			db:           db,
			connections:  make(map[string]*actor.PID),
			chatSessions: make(map[string]*actor.PID),
			syncActor:    nil, // Will be created when needed
			lastActiveAt: time.Now(),
			isOnline:     false,
		}
	}
}

// Receive handles messages for the user actor
func (u *UserActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		u.handleStarted(ctx)

	case *actor.Stopped:
		u.handleStopped(ctx)

	case *UserConnectionEstablished:
		u.handleConnectionEstablished(ctx, msg)

	case *UserConnectionClosed:
		u.handleConnectionClosed(ctx, msg)

	case *ConnectionReady:
		u.handleConnectionReady(ctx, msg)

	case *ForwardToUser:
		u.handleForwardToUser(ctx, msg)

	case *StartChatSession:
		u.handleStartChatSession(ctx, msg)

	case *StopChatSession:
		u.handleStopChatSession(ctx, msg)

	case *SendMessage:
		u.handleSendMessage(ctx, msg)

	case *CreateChatSessionRequest:
		u.handleCreateChatSession(ctx, msg)

	case *ListChatSessionsRequest:
		u.handleListChatSessions(ctx, msg)

	case *GetChatMessagesRequest:
		u.handleGetChatMessages(ctx, msg)

	case *GetUserInfoRequest:
		u.handleGetUserInfo(ctx, msg)

	case *ConsiderShutdown:
		u.handleConsiderShutdown(ctx, msg)

	default:
		log.Printf("UserActor[%s]: Unknown message type: %T", u.userID, msg)
	}
}

// handleStarted initializes the user actor
func (u *UserActor) handleStarted(ctx *actor.Context) {
	log.Printf("UserActor[%s]: Started", u.userID)
	u.lastActiveAt = time.Now()
}

// handleStopped cleans up when the user actor is stopped
func (u *UserActor) handleStopped(ctx *actor.Context) {
	log.Printf("UserActor[%s]: Stopped", u.userID)

	// Stop all connection actors
	for connectionID, connectionPID := range u.connections {
		if connectionPID != nil {
			ctx.Engine().Stop(connectionPID)
			log.Printf("UserActor[%s]: Stopped connection actor %s", u.userID, connectionID)
		}
	}

	// Stop all chat session actors
	for sessionID, sessionPID := range u.chatSessions {
		if sessionPID != nil {
			ctx.Engine().Stop(sessionPID)
			log.Printf("UserActor[%s]: Stopped chat session actor %s", u.userID, sessionID)
		}
	}

	// Stop sync actor if it exists
	if u.syncActor != nil {
		ctx.Engine().Stop(u.syncActor)
	}
}

// handleConnectionEstablished creates a new ConnectionActor for the user
func (u *UserActor) handleConnectionEstablished(ctx *actor.Context, msg *UserConnectionEstablished) {

}

// handleConnectionReady processes notification that a connection is ready
func (u *UserActor) handleConnectionReady(ctx *actor.Context, msg *ConnectionReady) {

}

// handleConnectionClosed manages connection cleanup
func (u *UserActor) handleConnectionClosed(ctx *actor.Context, msg *UserConnectionClosed) {

}

// handleForwardToUser forwards messages from connections to appropriate handlers
func (u *UserActor) handleForwardToUser(ctx *actor.Context, msg *ForwardToUser) {
	log.Printf("UserActor[%s]: Forwarding message from connection: %T", u.userID, msg.Message)

	// Route message based on type
	switch typedMsg := msg.Message.(type) {
	case *WebSocketMessage:
		u.handleWebSocketMessage(ctx, typedMsg)
	default:
		log.Printf("UserActor[%s]: Unknown forwarded message type: %T", u.userID, msg.Message)
	}
}

// handleWebSocketMessage processes WebSocket messages from connections
func (u *UserActor) handleWebSocketMessage(ctx *actor.Context, msg *WebSocketMessage) {

}

// handleStartChatSession starts a chat session by spawning ChatSessionActor
func (u *UserActor) handleStartChatSession(ctx *actor.Context, msg *StartChatSession) {

}

// handleStopChatSession stops a chat session
func (u *UserActor) handleStopChatSession(ctx *actor.Context, msg *StopChatSession) {

}

// handleSendMessage handles message sending within a chat session
func (u *UserActor) handleSendMessage(ctx *actor.Context, msg *SendMessage) {
}

// handleCreateChatSession creates a new chat session via database
func (u *UserActor) handleCreateChatSession(ctx *actor.Context, msg *CreateChatSessionRequest) {

}

// handleListChatSessions lists user's chat sessions
func (u *UserActor) handleListChatSessions(ctx *actor.Context, msg *ListChatSessionsRequest) {

}

// handleGetChatMessages gets messages for a chat session
func (u *UserActor) handleGetChatMessages(ctx *actor.Context, msg *GetChatMessagesRequest) {

}

// handleGetUserInfo returns current user information and stats
func (u *UserActor) handleGetUserInfo(ctx *actor.Context, msg *GetUserInfoRequest) {

}

// handleConsiderShutdown considers shutting down the user actor if idle
func (u *UserActor) handleConsiderShutdown(ctx *actor.Context, msg *ConsiderShutdown) {

}

// broadcastToConnections sends a message to all active connections
func (u *UserActor) broadcastToConnections(ctx *actor.Context, message interface{}) {

}

// broadcastUserState sends current user state to a specific connection
func (u *UserActor) broadcastUserState(ctx *actor.Context, connectionID string) {

}

// GetConnectionCount returns the number of active connections
func (u *UserActor) GetConnectionCount() int {
	return len(u.connections)
}

// GetChatSessionCount returns the number of active chat sessions
func (u *UserActor) GetChatSessionCount() int {
	return len(u.chatSessions)
}

// GetUserID returns the user ID
func (u *UserActor) GetUserID() string {
	return u.userID
}

// IsOnline returns whether the user is currently online
func (u *UserActor) IsOnline() bool {
	return u.isOnline
}

// GetLastActiveAt returns when the user was last active
func (u *UserActor) GetLastActiveAt() time.Time {
	return u.lastActiveAt
}

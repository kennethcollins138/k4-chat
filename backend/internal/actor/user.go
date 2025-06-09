package actor

import (
	"log"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/google/uuid"
	"github.com/kdot/k4-chat/backend/internal/database"
	"github.com/kdot/k4-chat/backend/internal/database/models"
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
	log.Printf("UserActor[%s]: Connection established - ConnectionID: %s", u.userID, msg.ConnectionID)
	// TODO: Watchout for DDOS-by-multiplexing, 3 is temporary, don't use magic numbers
	if len(u.connections) >= 3 {
		ctx.Respond(&ConnectionRejected{
			Reason: "Too many active connections",
			UserID: u.userID,
		})
		log.Printf("User %s exceeded connection limit", u.userID)
		return
	}
	// Spawn ConnectionActor to handle this specific connection
	connectionPID := ctx.SpawnChild(
		NewConnectionActor(msg.ConnectionID, u.userID, msg.DeviceInfo, msg.IPAddress),
		"conn-"+msg.ConnectionID,
	)
	u.connections[msg.ConnectionID] = connectionPID

	// Update user state
	u.lastActiveAt = time.Now()
	u.isOnline = true

	// Notify parent (UserManagerActor) to update user's last active timestamp
	ctx.Send(ctx.Parent(), &UpdateUserLastActiveRequest{
		UserID: uuid.MustParse(u.userID),
	})

	log.Printf("UserActor[%s]: Spawned ConnectionActor for connection %s, total connections: %d",
		u.userID, msg.ConnectionID, len(u.connections))
}

// handleConnectionReady processes notification that a connection is ready
func (u *UserActor) handleConnectionReady(ctx *actor.Context, msg *ConnectionReady) {
	log.Printf("UserActor[%s]: Connection %s is ready", u.userID, msg.ConnectionID)

	// Broadcast current user state to the new connection
	u.broadcastUserState(ctx, msg.ConnectionID)
}

// handleConnectionClosed manages connection cleanup
func (u *UserActor) handleConnectionClosed(ctx *actor.Context, msg *UserConnectionClosed) {
	log.Printf("UserActor[%s]: Connection closed - ConnectionID: %s", u.userID, msg.ConnectionID)

	// Remove and stop connection actor
	if connectionPID, exists := u.connections[msg.ConnectionID]; exists {
		if connectionPID != nil {
			ctx.Engine().Stop(connectionPID)
		}
		delete(u.connections, msg.ConnectionID)
	}

	log.Printf("UserActor[%s]: Cleaned up connection %s, remaining connections: %d",
		u.userID, msg.ConnectionID, len(u.connections))

	// Update online status
	u.isOnline = len(u.connections) > 0

	// If no more connections, consider shutdown after idle timeout
	if len(u.connections) == 0 {
		log.Printf("UserActor[%s]: No active connections, scheduling shutdown consideration", u.userID)

		// Use a goroutine with timer for delayed message sending
		go func() {
			timer := time.NewTimer(5 * time.Minute)
			defer timer.Stop()

			select {
			case <-timer.C:
				// Send the shutdown consideration message after 5 minutes
				ctx.Engine().Send(ctx.PID(), &ConsiderShutdown{})
			case <-ctx.Context().Done():
				// Context was cancelled, stop the timer
				return
			}
		}()
	}
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
	switch msg.Type {
	case "start_chat_session":
		// Handle chat session start request
		log.Printf("UserActor[%s]: Received start chat session request", u.userID)

	case "send_message":
		// Handle message send request
		if data, ok := msg.Data.(ChatMessageReceived); ok {
			sendMsg := &SendMessage{
				SessionID: data.SessionID,
				Role:      "user",
				Content:   data.Content,
			}
			u.handleSendMessage(ctx, sendMsg)
		}

	default:
		log.Printf("UserActor[%s]: Unknown WebSocket message type: %s", u.userID, msg.Type)
	}
}

// handleStartChatSession starts a chat session by spawning ChatSessionActor
func (u *UserActor) handleStartChatSession(ctx *actor.Context, msg *StartChatSession) {
	sessionID := msg.SessionID.String()

	log.Printf("UserActor[%s]: Starting chat session %s with model %s",
		u.userID, sessionID, msg.ModelName)

	// Check if session already exists
	if _, exists := u.chatSessions[sessionID]; exists {
		log.Printf("UserActor[%s]: Chat session %s already exists", u.userID, sessionID)
		return
	}

	// TODO: Spawn ChatSessionActor when it's implemented
	// sessionPID := ctx.SpawnChild(
	//     NewChatSessionActor(msg.SessionID, u.userID, msg.ModelName),
	//     "chat-"+sessionID,
	// )
	// u.chatSessions[sessionID] = sessionPID

	// Temporary placeholder
	u.chatSessions[sessionID] = nil

	// Broadcast session started to all connections
	u.broadcastToConnections(ctx, &SessionStarted{
		SessionID: msg.SessionID,
		ModelName: msg.ModelName,
	})

	log.Printf("UserActor[%s]: Chat session %s started, total sessions: %d",
		u.userID, sessionID, len(u.chatSessions))
}

// handleStopChatSession stops a chat session
func (u *UserActor) handleStopChatSession(ctx *actor.Context, msg *StopChatSession) {
	sessionID := msg.SessionID.String()

	log.Printf("UserActor[%s]: Stopping chat session %s", u.userID, sessionID)

	if sessionPID, exists := u.chatSessions[sessionID]; exists {
		// Stop the chat session actor when we implement ChatSessionActor
		if sessionPID != nil {
			ctx.Engine().Stop(sessionPID)
		}
		delete(u.chatSessions, sessionID)

		// Broadcast session stopped to all connections
		u.broadcastToConnections(ctx, &SessionStopped{
			SessionID: msg.SessionID,
		})

		log.Printf("UserActor[%s]: Chat session %s stopped, remaining sessions: %d",
			u.userID, sessionID, len(u.chatSessions))
	}
}

// handleSendMessage handles message sending within a chat session
func (u *UserActor) handleSendMessage(ctx *actor.Context, msg *SendMessage) {
	sessionID := msg.SessionID.String()

	log.Printf("UserActor[%s]: Sending message to session %s", u.userID, sessionID)

	// Forward to appropriate chat session actor
	if sessionPID, exists := u.chatSessions[sessionID]; exists {
		if sessionPID != nil {
			ctx.Send(sessionPID, msg)
		} else {
			log.Printf("UserActor[%s]: Chat session %s exists but PID is nil", u.userID, sessionID)
		}
	} else {
		log.Printf("UserActor[%s]: Chat session %s not found", u.userID, sessionID)
	}

	// Update last active timestamp
	u.lastActiveAt = time.Now()
}

// handleCreateChatSession creates a new chat session via database
func (u *UserActor) handleCreateChatSession(ctx *actor.Context, msg *CreateChatSessionRequest) {
	log.Printf("UserActor[%s]: Creating chat session: %s", u.userID, msg.Title)

	// Convert to database request
	dbReq := models.CreateChatSessionRequest{
		Title:        msg.Title,
		ModelName:    msg.ModelName,
		SystemPrompt: msg.SystemPrompt,
		Temperature:  msg.Temperature,
		MaxTokens:    msg.MaxTokens,
	}

	// Create session in database
	session, err := u.db.CreateChatSession(ctx.Context(), msg.UserID, dbReq)

	// Send response
	response := &CreateChatSessionResponse{
		Session: session,
		Error:   err,
	}

	ctx.Respond(response)

	// If successful, start tracking the session and broadcast to connections
	if err == nil && session != nil {
		sessionID := session.ID.String()
		u.chatSessions[sessionID] = nil // TODO: Spawn actual ChatSessionActor

		// Broadcast new session to all connections
		u.broadcastToConnections(ctx, &SessionCreated{
			Session: *session,
		})

		log.Printf("UserActor[%s]: Created and broadcasting chat session %s", u.userID, sessionID)
	}
}

// handleListChatSessions lists user's chat sessions
func (u *UserActor) handleListChatSessions(ctx *actor.Context, msg *ListChatSessionsRequest) {
	sessions, err := u.db.ListChatSessions(ctx.Context(), msg.UserID, msg.Limit, msg.Offset)

	response := &ListChatSessionsResponse{
		Sessions: sessions,
		Error:    err,
	}

	ctx.Respond(response)
}

// handleGetChatMessages gets messages for a chat session
func (u *UserActor) handleGetChatMessages(ctx *actor.Context, msg *GetChatMessagesRequest) {
	messages, err := u.db.GetChatMessages(ctx.Context(), msg.SessionID, msg.Limit, msg.Offset)

	response := &GetChatMessagesResponse{
		Messages: messages,
		Error:    err,
	}

	ctx.Respond(response)
}

// handleGetUserInfo returns current user information and stats
func (u *UserActor) handleGetUserInfo(ctx *actor.Context, msg *GetUserInfoRequest) {
	response := &UserInfoResponse{
		UserID:          u.userID,
		ConnectionCount: len(u.connections),
		SessionCount:    len(u.chatSessions),
		LastActiveAt:    u.lastActiveAt,
		IsOnline:        u.isOnline,
	}

	ctx.Respond(response)
}

// handleConsiderShutdown considers shutting down the user actor if idle
func (u *UserActor) handleConsiderShutdown(ctx *actor.Context, msg *ConsiderShutdown) {
	// Only shutdown if still no connections
	if len(u.connections) == 0 {
		log.Printf("UserActor[%s]: Shutting down due to no active connections", u.userID)
		ctx.Engine().Stop(ctx.PID())
	} else {
		log.Printf("UserActor[%s]: Shutdown consideration cancelled, %d connections active",
			u.userID, len(u.connections))
	}
}

// broadcastToConnections sends a message to all active connections
func (u *UserActor) broadcastToConnections(ctx *actor.Context, message interface{}) {
	if len(u.connections) == 0 {
		return
	}

	log.Printf("UserActor[%s]: Broadcasting %T to %d connections",
		u.userID, message, len(u.connections))

	for connectionID, connectionPID := range u.connections {
		if connectionPID != nil {
			forwardMsg := &ForwardToConnection{
				ConnectionID: connectionID,
				Message:      message,
			}
			ctx.Send(connectionPID, forwardMsg)
		}
	}
}

// broadcastUserState sends current user state to a specific connection
func (u *UserActor) broadcastUserState(ctx *actor.Context, connectionID string) {
	if connectionPID, exists := u.connections[connectionID]; exists && connectionPID != nil {
		userState := &UserStateUpdate{
			UserID:          u.userID,
			IsOnline:        u.isOnline,
			ConnectionCount: len(u.connections),
			SessionCount:    len(u.chatSessions),
			LastActiveAt:    u.lastActiveAt,
		}

		forwardMsg := &ForwardToConnection{
			ConnectionID: connectionID,
			Message:      userState,
		}
		ctx.Send(connectionPID, forwardMsg)

		log.Printf("UserActor[%s]: Sent user state to connection %s", u.userID, connectionID)
	}
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

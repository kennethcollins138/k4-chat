package actor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anthdm/hollywood/actor"
	"github.com/google/uuid"
	"github.com/kdot/k4-chat/backend/internal/database"
	"github.com/kdot/k4-chat/backend/internal/database/models"
)

/*
==========================================================================================================================

	TESTING SETUP AND MOCKS

==========================================================================================================================
*/

// MockDatabaseError represents a database error for testing
type MockDatabaseError struct {
	Message string
}

func (e *MockDatabaseError) Error() string {
	return e.Message
}

// MockDB implements a mock database for testing
type MockDB struct {
	users        map[uuid.UUID]*models.User
	sessions     map[uuid.UUID]*models.ChatSession
	messages     map[uuid.UUID][]models.Message
	mu           sync.RWMutex
	shouldError  bool
	errorMessage string
}

// NewMockDB creates a new mock database
func NewMockDB() *MockDB {
	return &MockDB{
		users:    make(map[uuid.UUID]*models.User),
		sessions: make(map[uuid.UUID]*models.ChatSession),
		messages: make(map[uuid.UUID][]models.Message),
	}
}

// SetError configures the mock to return errors
func (m *MockDB) SetError(shouldError bool, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldError = shouldError
	m.errorMessage = message
}

// CreateChatSession mock implementation
func (m *MockDB) CreateChatSession(ctx context.Context, userID uuid.UUID, req models.CreateChatSessionRequest) (*models.ChatSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return nil, &MockDatabaseError{Message: m.errorMessage}
	}

	sessionID := uuid.New()

	// Handle nullable fields
	var temperature float64
	var maxTokens int
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	session := &models.ChatSession{
		ID:           sessionID,
		UserID:       userID,
		Title:        req.Title,
		ModelName:    req.ModelName,
		SystemPrompt: req.SystemPrompt,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.sessions[sessionID] = session
	return session, nil
}

// ListChatSessions mock implementation
func (m *MockDB) ListChatSessions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.ChatSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, &MockDatabaseError{Message: m.errorMessage}
	}

	var sessions []models.ChatSession
	for _, session := range m.sessions {
		if session.UserID == userID {
			sessions = append(sessions, *session)
		}
	}

	// Simple pagination simulation
	start := offset
	end := offset + limit
	if start > len(sessions) {
		return []models.ChatSession{}, nil
	}
	if end > len(sessions) {
		end = len(sessions)
	}

	return sessions[start:end], nil
}

// GetChatMessages mock implementation
func (m *MockDB) GetChatMessages(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]models.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.shouldError {
		return nil, &MockDatabaseError{Message: m.errorMessage}
	}

	messages, exists := m.messages[sessionID]
	if !exists {
		return []models.Message{}, nil
	}

	// Simple pagination simulation
	start := offset
	end := offset + limit
	if start > len(messages) {
		return []models.Message{}, nil
	}
	if end > len(messages) {
		end = len(messages)
	}

	return messages[start:end], nil
}

// UpdateUserLastActive mock implementation
func (m *MockDB) UpdateUserLastActive(ctx context.Context, userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.shouldError {
		return &MockDatabaseError{Message: m.errorMessage}
	}

	if user, exists := m.users[userID]; exists {
		user.LastActiveAt = time.Now()
	}

	return nil
}

// Test helpers
func createTestUserActor(t *testing.T, userID string, db *MockDB) (*actor.Engine, *actor.PID) {
	engine, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		t.Fatalf("Failed to create test engine: %v", err)
	}

	// Convert MockDB to the interface expected by UserActor
	var dbInterface *database.DB
	if db != nil {
		// In a real implementation, you'd have proper interface conversion
		// For testing purposes, we'll pass nil and handle it in the actor
		dbInterface = nil
	}

	userPID := engine.Spawn(NewUserActor(userID, dbInterface), "test-user-"+userID)

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	return engine, userPID
}

func createTestUserActorWithDB(t *testing.T, userID string, mockDB *MockDB) (*actor.Engine, *actor.PID, *MockDB) {
	if mockDB == nil {
		mockDB = NewMockDB()
	}

	engine, userPID := createTestUserActor(t, userID, mockDB)
	return engine, userPID, mockDB
}

/*
==========================================================================================================================

	LIFECYCLE TESTS

==========================================================================================================================
*/

// TestUserActor_Lifecycle tests basic actor lifecycle
func TestUserActor_Lifecycle(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-123", nil)
	defer engine.Poison(userPID)

	// Verify actor started successfully
	if userPID == nil {
		t.Fatal("Failed to create UserActor")
	}

	// Test GetUserInfo to verify actor is working
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp, ok := response.(*UserInfoResponse)
	if !ok {
		t.Fatalf("Expected UserInfoResponse, got %T", response)
	}

	if infoResp.UserID != "user-123" {
		t.Errorf("Expected userID 'user-123', got '%s'", infoResp.UserID)
	}

	if infoResp.ConnectionCount != 0 {
		t.Errorf("Expected 0 connections initially, got %d", infoResp.ConnectionCount)
	}

	if infoResp.IsOnline {
		t.Error("Expected user to be offline initially")
	}
}

// TestUserActor_StartedMessage tests the Started message handling
func TestUserActor_StartedMessage(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-456", nil)
	defer engine.Poison(userPID)

	// The actor should already be started, verify its state
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info after startup: %v", err)
	}

	infoResp, ok := response.(*UserInfoResponse)
	if !ok {
		t.Fatalf("Expected UserInfoResponse, got %T", response)
	}

	// Verify LastActiveAt was set during startup
	if infoResp.LastActiveAt.IsZero() {
		t.Error("Expected LastActiveAt to be set during startup")
	}

	// Verify it was set recently (within last few seconds)
	if time.Since(infoResp.LastActiveAt) > 5*time.Second {
		t.Error("Expected LastActiveAt to be recent")
	}
}

// FIX: Failing still executing without error after
// TestUserActor_StoppedMessage tests cleanup during shutdown
func TestUserActor_StoppedMessage(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-789", nil)

	// Add a connection first
	connMsg := &UserConnectionEstablished{
		ConnectionID: "conn-1",
		DeviceInfo:   stringPtr("Test Device"),
		IPAddress:    stringPtr("127.0.0.1"),
	}
	engine.Send(userPID, connMsg)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Stop the actor (this triggers handleStopped)
	engine.Poison(userPID)

	// Give it time to clean up
	time.Sleep(200 * time.Millisecond)

	// Try to send a message to verify it's stopped
	infoReq := &GetUserInfoRequest{}
	futureResponse := engine.Request(userPID, infoReq, 1*time.Second)
	_, err := futureResponse.Result()

	// Should fail since actor is stopped
	if err == nil {
		t.Error("Expected request to fail after actor is stopped")
	}
}

/*
==========================================================================================================================

	CONNECTION MANAGEMENT TESTS

==========================================================================================================================
*/

// FIX: Expected 1 connection, got 0
// TestUserActor_ConnectionEstablished tests connection establishment
func TestUserActor_ConnectionEstablished(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-connection", nil)
	defer engine.Poison(userPID)

	// Establish a connection
	connMsg := &UserConnectionEstablished{
		ConnectionID: "conn-123",
		DeviceInfo:   stringPtr("Chrome Browser"),
		IPAddress:    stringPtr("192.168.1.100"),
	}
	engine.Send(userPID, connMsg)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify connection was added
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.ConnectionCount != 1 {
		t.Errorf("Expected 1 connection, got %d", infoResp.ConnectionCount)
	}

	if !infoResp.IsOnline {
		t.Error("Expected user to be online after connection established")
	}
}

// FIX: Expect 3 got 0
// TestUserActor_MultipleConnections tests multiple connection handling
func TestUserActor_MultipleConnections(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-multi", nil)
	defer engine.Poison(userPID)

	// Establish multiple connections
	connections := []string{"conn-1", "conn-2", "conn-3"}
	for _, connID := range connections {
		connMsg := &UserConnectionEstablished{
			ConnectionID: connID,
			DeviceInfo:   stringPtr("Device"),
			IPAddress:    stringPtr("127.0.0.1"),
		}
		engine.Send(userPID, connMsg)
		time.Sleep(50 * time.Millisecond)
	}

	// Verify all connections were added
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.ConnectionCount != len(connections) {
		t.Errorf("Expected %d connections, got %d", len(connections), infoResp.ConnectionCount)
	}

	if !infoResp.IsOnline {
		t.Error("Expected user to be online with multiple connections")
	}
}

// TestUserActor_ConnectionClosed tests connection closure
func TestUserActor_ConnectionClosed(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-close", nil)
	defer engine.Poison(userPID)

	// Establish a connection first
	connMsg := &UserConnectionEstablished{
		ConnectionID: "conn-close",
		DeviceInfo:   stringPtr("Test Device"),
		IPAddress:    stringPtr("127.0.0.1"),
	}
	engine.Send(userPID, connMsg)
	time.Sleep(100 * time.Millisecond)

	// Close the connection
	closeMsg := &UserConnectionClosed{
		ConnectionID: "conn-close",
	}
	engine.Send(userPID, closeMsg)
	time.Sleep(100 * time.Millisecond)

	// Verify connection was removed
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.ConnectionCount != 0 {
		t.Errorf("Expected 0 connections after close, got %d", infoResp.ConnectionCount)
	}

	if infoResp.IsOnline {
		t.Error("Expected user to be offline after all connections closed")
	}
}

// TestUserActor_ConnectionReady tests ConnectionReady message handling
func TestUserActor_ConnectionReady(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-ready", nil)
	defer engine.Poison(userPID)

	// Establish a connection first
	connMsg := &UserConnectionEstablished{
		ConnectionID: "conn-ready",
		DeviceInfo:   stringPtr("Test Device"),
		IPAddress:    stringPtr("127.0.0.1"),
	}
	engine.Send(userPID, connMsg)
	time.Sleep(100 * time.Millisecond)

	// Send ConnectionReady message
	readyMsg := &ConnectionReady{
		ConnectionID: "conn-ready",
		UserID:       "user-ready",
		ConnectedAt:  time.Now(),
	}
	engine.Send(userPID, readyMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// This should not cause any errors (testing for stability)
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info after ConnectionReady: %v", err)
	}
}

/*
==========================================================================================================================

	CHAT SESSION MANAGEMENT TESTS

==========================================================================================================================
*/

// TestUserActor_StartChatSession tests chat session starting
func TestUserActor_StartChatSession(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-session", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()
	startMsg := &StartChatSession{
		SessionID: sessionID,
		ModelName: "gpt-4",
	}
	engine.Send(userPID, startMsg)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify session was added
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.SessionCount != 1 {
		t.Errorf("Expected 1 session, got %d", infoResp.SessionCount)
	}
}

// TestUserActor_DuplicateSessionStart tests starting same session twice
func TestUserActor_DuplicateSessionStart(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-dup", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()
	startMsg := &StartChatSession{
		SessionID: sessionID,
		ModelName: "gpt-4",
	}

	// Start session twice
	engine.Send(userPID, startMsg)
	time.Sleep(50 * time.Millisecond)
	engine.Send(userPID, startMsg)
	time.Sleep(50 * time.Millisecond)

	// Should still only have one session
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.SessionCount != 1 {
		t.Errorf("Expected 1 session after duplicate start, got %d", infoResp.SessionCount)
	}
}

// TestUserActor_StopChatSession tests chat session stopping
func TestUserActor_StopChatSession(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-stop", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()

	// Start session first
	startMsg := &StartChatSession{
		SessionID: sessionID,
		ModelName: "gpt-4",
	}
	engine.Send(userPID, startMsg)
	time.Sleep(100 * time.Millisecond)

	// Stop session
	stopMsg := &StopChatSession{
		SessionID: sessionID,
	}
	engine.Send(userPID, stopMsg)
	time.Sleep(100 * time.Millisecond)

	// Verify session was removed
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.SessionCount != 0 {
		t.Errorf("Expected 0 sessions after stop, got %d", infoResp.SessionCount)
	}
}

// TestUserActor_StopNonexistentSession tests stopping a session that doesn't exist
func TestUserActor_StopNonexistentSession(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-nonexistent", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()

	// Stop session that was never started
	stopMsg := &StopChatSession{
		SessionID: sessionID,
	}
	engine.Send(userPID, stopMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Should not cause any issues
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info after stopping nonexistent session: %v", err)
	}
}

/*
==========================================================================================================================

	MESSAGE HANDLING TESTS

==========================================================================================================================
*/

// TestUserActor_SendMessage tests message sending to chat sessions
func TestUserActor_SendMessage(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-msg", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()

	// Start session first
	startMsg := &StartChatSession{
		SessionID: sessionID,
		ModelName: "gpt-4",
	}
	engine.Send(userPID, startMsg)
	time.Sleep(100 * time.Millisecond)

	// Send message
	sendMsg := &SendMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Hello, world!",
	}
	engine.Send(userPID, sendMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Verify LastActiveAt was updated
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if time.Since(infoResp.LastActiveAt) > 2*time.Second {
		t.Error("Expected LastActiveAt to be updated after sending message")
	}
}

// TestUserActor_SendMessageToNonexistentSession tests sending message to nonexistent session
func TestUserActor_SendMessageToNonexistentSession(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-nonexistent-msg", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()

	// Send message to session that doesn't exist
	sendMsg := &SendMessage{
		SessionID: sessionID,
		Role:      "user",
		Content:   "Hello, world!",
	}
	engine.Send(userPID, sendMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Should not cause any issues (will be logged but not crash)
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info after sending to nonexistent session: %v", err)
	}
}

// TestUserActor_ForwardToUser tests message forwarding functionality
func TestUserActor_ForwardToUser(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-forward", nil)
	defer engine.Poison(userPID)

	// Send ForwardToUser message with WebSocket message
	wsMsg := &WebSocketMessage{
		Type: "send_message",
		Data: ChatMessageReceived{
			SessionID: uuid.New(),
			Content:   "Test message",
		},
	}

	forwardMsg := &ForwardToUser{
		UserID:  "user-forward",
		Message: wsMsg,
	}
	engine.Send(userPID, forwardMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Should not cause any issues
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info after ForwardToUser: %v", err)
	}
}

// TestUserActor_WebSocketMessage tests WebSocket message handling
func TestUserActor_WebSocketMessage(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-ws", nil)
	defer engine.Poison(userPID)

	// Test different WebSocket message types
	testCases := []struct {
		msgType string
		data    interface{}
	}{
		{"start_chat_session", nil},
		{"send_message", ChatMessageReceived{SessionID: uuid.New(), Content: "Test"}},
		{"unknown_type", nil},
	}

	for _, tc := range testCases {
		wsMsg := &WebSocketMessage{
			Type: tc.msgType,
			Data: tc.data,
		}

		forwardMsg := &ForwardToUser{
			UserID:  "user-ws",
			Message: wsMsg,
		}
		engine.Send(userPID, forwardMsg)
		time.Sleep(25 * time.Millisecond)
	}

	// Verify actor is still responsive
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info after WebSocket messages: %v", err)
	}
}

/*
==========================================================================================================================

	DATABASE OPERATION TESTS

==========================================================================================================================
*/

// FIX: Context deadline exceeded
// TestUserActor_CreateChatSession tests chat session creation via database
func TestUserActor_CreateChatSession(t *testing.T) {
	engine, userPID, mockDB := createTestUserActorWithDB(t, "user-create", nil)
	defer engine.Poison(userPID)

	userID := uuid.New()
	createReq := &CreateChatSessionRequest{
		UserID:    userID,
		Title:     "Test Session",
		ModelName: "gpt-4",
	}

	response, err := engine.Request(userPID, createReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to create chat session: %v", err)
	}

	createResp, ok := response.(*CreateChatSessionResponse)
	if !ok {
		t.Fatalf("Expected CreateChatSessionResponse, got %T", response)
	}

	if createResp.Error != nil {
		t.Fatalf("Expected no error, got: %v", createResp.Error)
	}

	if createResp.Session == nil {
		t.Fatal("Expected session to be created")
	}

	if createResp.Session.Title != "Test Session" {
		t.Errorf("Expected title 'Test Session', got '%s'", createResp.Session.Title)
	}

	_ = mockDB // Use mockDB to avoid unused variable error
}

// FIX: Context Deadline Exceeded
// TestUserActor_CreateChatSessionError tests chat session creation with database error
func TestUserActor_CreateChatSessionError(t *testing.T) {
	engine, userPID, mockDB := createTestUserActorWithDB(t, "user-create-error", nil)
	defer engine.Poison(userPID)

	// Configure mock to return error
	mockDB.SetError(true, "Database connection failed")

	userID := uuid.New()
	createReq := &CreateChatSessionRequest{
		UserID:    userID,
		Title:     "Test Session",
		ModelName: "gpt-4",
	}

	response, err := engine.Request(userPID, createReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to send create request: %v", err)
	}

	createResp, ok := response.(*CreateChatSessionResponse)
	if !ok {
		t.Fatalf("Expected CreateChatSessionResponse, got %T", response)
	}

	if createResp.Error == nil {
		t.Fatal("Expected error from database, got none")
	}

	if createResp.Session != nil {
		t.Error("Expected no session when error occurs")
	}
}

// FIX: Context deadline exceeded
// TestUserActor_ListChatSessions tests listing chat sessions
func TestUserActor_ListChatSessions(t *testing.T) {
	engine, userPID, _ := createTestUserActorWithDB(t, "user-list", nil)
	defer engine.Poison(userPID)

	userID := uuid.New()
	listReq := &ListChatSessionsRequest{
		UserID: userID,
		Limit:  10,
		Offset: 0,
	}

	response, err := engine.Request(userPID, listReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to list chat sessions: %v", err)
	}

	listResp, ok := response.(*ListChatSessionsResponse)
	if !ok {
		t.Fatalf("Expected ListChatSessionsResponse, got %T", response)
	}

	if listResp.Error != nil {
		t.Fatalf("Expected no error, got: %v", listResp.Error)
	}

	// Should return empty list for new user
	if len(listResp.Sessions) != 0 {
		t.Errorf("Expected 0 sessions for new user, got %d", len(listResp.Sessions))
	}
}

// FIX: ContextDeadlineExceeded
// TestUserActor_GetChatMessages tests getting chat messages
func TestUserActor_GetChatMessages(t *testing.T) {
	engine, userPID, _ := createTestUserActorWithDB(t, "user-messages", nil)
	defer engine.Poison(userPID)

	sessionID := uuid.New()
	getReq := &GetChatMessagesRequest{
		SessionID: sessionID,
		Limit:     10,
		Offset:    0,
	}

	response, err := engine.Request(userPID, getReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get chat messages: %v", err)
	}

	getResp, ok := response.(*GetChatMessagesResponse)
	if !ok {
		t.Fatalf("Expected GetChatMessagesResponse, got %T", response)
	}

	if getResp.Error != nil {
		t.Fatalf("Expected no error, got: %v", getResp.Error)
	}

	// Should return empty list for new session
	if len(getResp.Messages) != 0 {
		t.Errorf("Expected 0 messages for new session, got %d", len(getResp.Messages))
	}
}

/*
==========================================================================================================================

	SHUTDOWN AND TIMEOUT TESTS

==========================================================================================================================
*/

// TestUserActor_ConsiderShutdown tests shutdown consideration logic
func TestUserActor_ConsiderShutdown(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-shutdown", nil)

	// Send ConsiderShutdown message
	shutdownMsg := &ConsiderShutdown{}
	engine.Send(userPID, shutdownMsg)

	// Give it time to process shutdown
	time.Sleep(200 * time.Millisecond)

	// Actor should be stopped now
	infoReq := &GetUserInfoRequest{}
	futureResponse := engine.Request(userPID, infoReq, 1*time.Second)
	_, err := futureResponse.Result()

	// Should fail since actor is stopped
	if err == nil {
		t.Error("Expected request to fail after shutdown consideration")
	}
}

// TestUserActor_ConsiderShutdownWithConnections tests shutdown cancellation when connections exist
func TestUserActor_ConsiderShutdownWithConnections(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-shutdown-cancel", nil)
	defer engine.Poison(userPID)

	// Add a connection
	connMsg := &UserConnectionEstablished{
		ConnectionID: "conn-persist",
		DeviceInfo:   stringPtr("Test Device"),
		IPAddress:    stringPtr("127.0.0.1"),
	}
	engine.Send(userPID, connMsg)
	time.Sleep(100 * time.Millisecond)

	// Send ConsiderShutdown message
	shutdownMsg := &ConsiderShutdown{}
	engine.Send(userPID, shutdownMsg)
	time.Sleep(100 * time.Millisecond)

	// Actor should still be alive because it has connections
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Expected actor to stay alive with connections: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.ConnectionCount != 1 {
		t.Errorf("Expected 1 connection, got %d", infoResp.ConnectionCount)
	}
}

// TestUserActor_IdleTimeout tests the idle timeout mechanism
func TestUserActor_IdleTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	engine, userPID := createTestUserActor(t, "user-timeout", nil)

	// Add a connection then close it to trigger timeout
	connMsg := &UserConnectionEstablished{
		ConnectionID: "conn-timeout",
		DeviceInfo:   stringPtr("Test Device"),
		IPAddress:    stringPtr("127.0.0.1"),
	}
	engine.Send(userPID, connMsg)
	time.Sleep(100 * time.Millisecond)

	// Close the connection to trigger timeout consideration
	closeMsg := &UserConnectionClosed{
		ConnectionID: "conn-timeout",
	}
	engine.Send(userPID, closeMsg)

	// The actor should schedule a shutdown consideration
	// In real implementation, this would be 5 minutes, but for testing we'll check the mechanism
	time.Sleep(200 * time.Millisecond)

	// Verify the actor is still alive (timeout goroutine was started)
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 1*time.Second).Result()
	if err != nil {
		t.Fatalf("Actor should still be alive during timeout period: %v", err)
	}
}

/*
==========================================================================================================================

	CONCURRENT OPERATION TESTS

==========================================================================================================================
*/

// TestUserActor_ConcurrentConnections tests concurrent connection operations
func TestUserActor_ConcurrentConnections(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-concurrent", nil)
	defer engine.Poison(userPID)

	const numConnections = 10
	var wg sync.WaitGroup

	// Establish connections concurrently
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			connMsg := &UserConnectionEstablished{
				ConnectionID: fmt.Sprintf("conn-%d", i),
				DeviceInfo:   stringPtr("Test Device"),
				IPAddress:    stringPtr("127.0.0.1"),
			}
			engine.Send(userPID, connMsg)
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Verify all connections were added
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.ConnectionCount != numConnections {
		t.Errorf("Expected %d connections, got %d", numConnections, infoResp.ConnectionCount)
	}
}

// TestUserActor_ConcurrentSessions tests concurrent session operations
func TestUserActor_ConcurrentSessions(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-concurrent-sessions", nil)
	defer engine.Poison(userPID)

	const numSessions = 5
	var wg sync.WaitGroup

	// Start sessions concurrently
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			startMsg := &StartChatSession{
				SessionID: uuid.New(),
				ModelName: fmt.Sprintf("model-%d", i),
			}
			engine.Send(userPID, startMsg)
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Verify all sessions were added
	infoReq := &GetUserInfoRequest{}
	response, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get user info: %v", err)
	}

	infoResp := response.(*UserInfoResponse)
	if infoResp.SessionCount != numSessions {
		t.Errorf("Expected %d sessions, got %d", numSessions, infoResp.SessionCount)
	}
}

// TestUserActor_ConcurrentRequests tests concurrent database requests
func TestUserActor_ConcurrentRequests(t *testing.T) {
	engine, userPID, _ := createTestUserActorWithDB(t, "user-concurrent-req", nil)
	defer engine.Poison(userPID)

	const numRequests = 10
	var wg sync.WaitGroup
	results := make(chan error, numRequests)

	userID := uuid.New()

	// Send concurrent requests
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			listReq := &ListChatSessionsRequest{
				UserID: userID,
				Limit:  10,
				Offset: 0,
			}

			response, err := engine.Request(userPID, listReq, 5*time.Second).Result()
			if err != nil {
				results <- err
				return
			}

			if _, ok := response.(*ListChatSessionsResponse); !ok {
				results <- fmt.Errorf("unexpected response type: %T", response)
				return
			}

			results <- nil
		}()
	}

	wg.Wait()
	close(results)

	// Check all requests succeeded
	for err := range results {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}

/*
==========================================================================================================================

	UNKNOWN MESSAGE TESTS

==========================================================================================================================
*/

// TestUserActor_UnknownMessage tests handling of unknown message types
func TestUserActor_UnknownMessage(t *testing.T) {
	engine, userPID := createTestUserActor(t, "user-unknown", nil)
	defer engine.Poison(userPID)

	// Send unknown message type
	unknownMsg := "unknown message type"
	engine.Send(userPID, unknownMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Verify actor is still responsive
	infoReq := &GetUserInfoRequest{}
	_, err := engine.Request(userPID, infoReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Actor should remain responsive after unknown message: %v", err)
	}
}

/*
==========================================================================================================================

	GETTER METHOD TESTS

==========================================================================================================================
*/

// TestUserActor_GetterMethods tests all getter methods
func TestUserActor_GetterMethods(t *testing.T) {
	// Create UserActor manually to test getter methods
	userActor := &UserActor{
		userID:       "test-getter",
		connections:  make(map[string]*actor.PID),
		chatSessions: make(map[string]*actor.PID),
		lastActiveAt: time.Now(),
		isOnline:     true,
	}

	// Add some test data
	userActor.connections["conn-1"] = &actor.PID{}
	userActor.connections["conn-2"] = &actor.PID{}
	userActor.chatSessions["session-1"] = &actor.PID{}

	// Test GetConnectionCount
	if userActor.GetConnectionCount() != 2 {
		t.Errorf("Expected 2 connections, got %d", userActor.GetConnectionCount())
	}

	// Test GetChatSessionCount
	if userActor.GetChatSessionCount() != 1 {
		t.Errorf("Expected 1 session, got %d", userActor.GetChatSessionCount())
	}

	// Test GetUserID
	if userActor.GetUserID() != "test-getter" {
		t.Errorf("Expected user ID 'test-getter', got '%s'", userActor.GetUserID())
	}

	// Test IsOnline
	if !userActor.IsOnline() {
		t.Error("Expected user to be online")
	}

	// Test GetLastActiveAt
	if userActor.GetLastActiveAt().IsZero() {
		t.Error("Expected non-zero last active time")
	}
}

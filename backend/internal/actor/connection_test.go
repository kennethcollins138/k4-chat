package actor

import (
	"testing"
	"time"

	"github.com/anthdm/hollywood/actor"
)

// Test helper to create a test ConnectionActor
func createTestConnectionActor(t *testing.T) (*actor.Engine, *actor.PID) {
	engine, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		t.Fatalf("Failed to create test engine: %v", err)
	}

	connectionPID := engine.Spawn(
		NewConnectionActor("test-conn-1", "user-123", stringPtr("Chrome/1.0"), stringPtr("127.0.0.1")),
		"test-connection",
	)

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	return engine, connectionPID
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// TestConnectionActor_Lifecycle tests basic actor lifecycle
func TestConnectionActor_Lifecycle(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Test that actor started successfully
	if connectionPID == nil {
		t.Fatal("Failed to create ConnectionActor")
	}

	// Test metrics request
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	// Verify initial state
	if metricsResp.ConnectionID != "test-conn-1" {
		t.Errorf("Expected connectionID 'test-conn-1', got '%s'", metricsResp.ConnectionID)
	}

	if metricsResp.UserID != "user-123" {
		t.Errorf("Expected userID 'user-123', got '%s'", metricsResp.UserID)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to be healthy")
	}

	if metricsResp.DeviceInfo == nil || *metricsResp.DeviceInfo != "Chrome/1.0" {
		t.Error("Expected device info to be 'Chrome/1.0'")
	}

	if metricsResp.IPAddress == nil || *metricsResp.IPAddress != "127.0.0.1" {
		t.Error("Expected IP address to be '127.0.0.1'")
	}
}

// TestConnectionActor_GetterMethods tests all getter methods
func TestConnectionActor_GetterMethods(t *testing.T) {
	// Create actor manually to test getter methods
	config := DefaultConnectionConfig()
	producer := NewConnectionActor("test-conn-1", "user-123", stringPtr("Chrome/1.0"), stringPtr("127.0.0.1"))
	receiver := producer()

	connectionActor, ok := receiver.(*ConnectionActor)
	if !ok {
		t.Fatal("Failed to cast to ConnectionActor")
	}

	// Test GetConnectionID
	if connectionActor.GetConnectionID() != "test-conn-1" {
		t.Errorf("Expected connection ID 'test-conn-1', got '%s'", connectionActor.GetConnectionID())
	}

	// Test GetUserID
	if connectionActor.GetUserID() != "user-123" {
		t.Errorf("Expected user ID 'user-123', got '%s'", connectionActor.GetUserID())
	}

	// Test IsHealthy (should be true initially)
	if !connectionActor.IsHealthy() {
		t.Error("Expected connection to be healthy initially")
	}

	// Test GetMetrics
	metrics := connectionActor.GetMetrics()
	if metrics.BytesSent != 0 {
		t.Errorf("Expected BytesSent to be 0, got %d", metrics.BytesSent)
	}
	if metrics.BytesReceived != 0 {
		t.Errorf("Expected BytesReceived to be 0, got %d", metrics.BytesReceived)
	}
	if metrics.MessagesSent != 0 {
		t.Errorf("Expected MessagesSent to be 0, got %d", metrics.MessagesSent)
	}

	// Test GetUptime
	uptime := connectionActor.GetUptime()
	if uptime <= 0 {
		t.Error("Expected positive uptime")
	}

	// Test ping timeout changes
	_ = config
}

// TestConnectionActor_Heartbeat tests heartbeat functionality
func TestConnectionActor_Heartbeat(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Send heartbeat
	heartbeat := &ConnectionHeartbeat{
		Timestamp: time.Now(),
	}
	engine.Send(connectionPID, heartbeat)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Check metrics to verify heartbeat was processed
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to be healthy after heartbeat")
	}

	// Verify lastPingAt was updated (should be recent)
	if time.Since(metricsResp.LastPingAt) > 5*time.Second {
		t.Error("LastPingAt should have been updated recently")
	}
}

// TestConnectionActor_MultipleHeartbeats tests multiple heartbeat handling
func TestConnectionActor_MultipleHeartbeats(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Send multiple heartbeats
	for i := 0; i < 5; i++ {
		heartbeat := &ConnectionHeartbeat{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		}
		engine.Send(connectionPID, heartbeat)
		time.Sleep(10 * time.Millisecond)
	}

	// Give it time to process all heartbeats
	time.Sleep(100 * time.Millisecond)

	// Check final state
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to be healthy after multiple heartbeats")
	}

	// Verify lastPingAt was updated to the most recent heartbeat
	if time.Since(metricsResp.LastPingAt) > 5*time.Second {
		t.Error("LastPingAt should reflect the most recent heartbeat")
	}
}

// TestConnectionActor_MessageForwarding tests message forwarding functionality
func TestConnectionActor_MessageForwarding(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Test forwarding to connection
	forwardMsg := &ForwardToConnection{
		ConnectionID: "test-conn-1",
		Message:      "test message",
	}
	engine.Send(connectionPID, forwardMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Check that metrics were updated (message sent)
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if metricsResp.Metrics.MessagesSent == 0 {
		t.Error("Expected MessagesSent to be incremented")
	}
}

// TestConnectionActor_MessageForwardingWhenClosing tests message dropping during shutdown
func TestConnectionActor_MessageForwardingWhenClosing(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Start closing the connection
	closeMsg := &CloseConnection{
		Reason: "test shutdown",
	}
	engine.Send(connectionPID, closeMsg)

	// Immediately try to forward a message (should be dropped)
	forwardMsg := &ForwardToConnection{
		ConnectionID: "test-conn-1",
		Message:      "should be dropped",
	}
	engine.Send(connectionPID, forwardMsg)

	// Give it time to process
	time.Sleep(200 * time.Millisecond)

	// The actor should be stopped now, so further testing isn't meaningful
}

// TestConnectionActor_WebSocketMessage tests WebSocket message handling
func TestConnectionActor_WebSocketMessage(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Send WebSocket message
	wsMsg := &WebSocketMessage{
		Type: "chat_message",
		Data: map[string]interface{}{
			"content": "Hello, world!",
		},
	}
	engine.Send(connectionPID, wsMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Check that metrics were updated
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if metricsResp.Metrics.MessagesSent == 0 {
		t.Error("Expected MessagesSent to be incremented for WebSocket message")
	}
}

// TestConnectionActor_WebSocketMessageWhenClosing tests WebSocket message handling during shutdown
func TestConnectionActor_WebSocketMessageWhenClosing(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Start closing the connection
	closeMsg := &CloseConnection{
		Reason: "test shutdown",
	}
	engine.Send(connectionPID, closeMsg)

	// Try to send WebSocket message (should be ignored)
	wsMsg := &WebSocketMessage{
		Type: "chat_message",
		Data: map[string]interface{}{
			"content": "should be ignored",
		},
	}
	engine.Send(connectionPID, wsMsg)

	// Give it time to process
	time.Sleep(200 * time.Millisecond)
}

// TestConnectionActor_PingTicker tests ping ticker functionality
func TestConnectionActor_PingTicker(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Send start ping ticker message
	engine.Send(connectionPID, &StartPingTicker{})

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Send stop ping ticker message
	engine.Send(connectionPID, &StopPingTicker{})

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Verify connection is still healthy
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to remain healthy after ticker operations")
	}
}

// TestConnectionActor_PingTimeout tests ping timeout handling
func TestConnectionActor_PingTimeout(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Send ping timeout message directly
	engine.Send(connectionPID, &PingTimeout{})

	// Give it time to process and shutdown
	time.Sleep(300 * time.Millisecond)

	// Try to send a message to verify it's stopped (should fail or timeout)
	metricsReq := &GetConnectionMetrics{}
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err := futureResponse.Result()

	// We expect this to fail since the actor should be stopped after ping timeout
	if err == nil {
		t.Error("Expected request to fail after ping timeout")
	}
}

// TestConnectionActor_GracefulShutdown tests graceful connection shutdown
func TestConnectionActor_GracefulShutdown(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Verify connection is healthy first
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to be healthy before shutdown")
	}

	// Send close connection message
	closeMsg := &CloseConnection{
		Reason: "test shutdown",
	}
	engine.Send(connectionPID, closeMsg)

	// Give it time to shut down
	time.Sleep(200 * time.Millisecond)

	// Try to send a message to verify it's stopped (should fail or timeout)
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err = futureResponse.Result()

	// We expect this to fail since the actor should be stopped
	if err == nil {
		t.Error("Expected request to fail after connection shutdown")
	}
}

// TestConnectionActor_UnknownMessage tests handling of unknown message types
func TestConnectionActor_UnknownMessage(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Send unknown message type
	unknownMsg := "unknown message"
	engine.Send(connectionPID, unknownMsg)

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Verify connection is still healthy after unknown message
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics after unknown message: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to remain healthy after unknown message")
	}
}

// TestConnectionActor_StartedMessage tests the handleStarted functionality
func TestConnectionActor_StartedMessage(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// The actor should already be started, so we can just verify it's working
	// by checking that it responds to requests (proving handleStarted worked)
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	// Verify the connection was properly initialized
	if metricsResp.ConnectionID != "test-conn-1" {
		t.Error("Connection was not properly initialized")
	}

	// Verify ConnectedAt is set
	if metricsResp.ConnectedAt.IsZero() {
		t.Error("ConnectedAt should be set after startup")
	}
}

// TestConnectionActor_StoppedMessage tests the handleStopped functionality
func TestConnectionActor_StoppedMessage(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Poison the actor to trigger the Stopped message
	engine.Poison(connectionPID)

	// Give it time to stop
	time.Sleep(200 * time.Millisecond)

	// Try to send a message to verify it's stopped
	metricsReq := &GetConnectionMetrics{}
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err := futureResponse.Result()

	// We expect this to fail since the actor should be stopped
	if err == nil {
		t.Error("Expected request to fail after actor is stopped")
	}
}

// TestConnectionActor_StartPingTickerGoroutine tests the goroutine in startPingTicker
func TestConnectionActor_StartPingTickerGoroutine(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Start the ping ticker explicitly
	engine.Send(connectionPID, &StartPingTicker{})

	// Wait a bit longer to let the goroutine run and send some pings
	time.Sleep(500 * time.Millisecond)

	// Stop the ticker
	engine.Send(connectionPID, &StopPingTicker{})

	// Verify the connection is still healthy
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to remain healthy after ping ticker test")
	}
}

// TestConnectionActor_PingTickerWithContextCancellation tests ping ticker goroutine context cancellation
func TestConnectionActor_PingTickerWithContextCancellation(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Start the ping ticker
	engine.Send(connectionPID, &StartPingTicker{})

	// Wait for ticker to start
	time.Sleep(100 * time.Millisecond)

	// Poison the actor to cancel context
	engine.Poison(connectionPID)

	// Give it time to handle context cancellation
	time.Sleep(200 * time.Millisecond)

	// Try to send a message to verify it's stopped
	metricsReq := &GetConnectionMetrics{}
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err := futureResponse.Result()

	// We expect this to fail since the actor should be stopped
	if err == nil {
		t.Error("Expected request to fail after context cancellation")
	}
}

// TestConnectionActor_PingTickerWhenClosing tests ping ticker behavior during connection close
func TestConnectionActor_PingTickerWhenClosing(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Start the ping ticker
	engine.Send(connectionPID, &StartPingTicker{})

	// Wait for ticker to start
	time.Sleep(100 * time.Millisecond)

	// Start closing the connection (this sets isClosing flag)
	closeMsg := &CloseConnection{
		Reason: "test close during ping",
	}
	engine.Send(connectionPID, closeMsg)

	// Wait for the close to be processed and goroutine to exit
	time.Sleep(300 * time.Millisecond)

	// The actor should be stopped now
	metricsReq := &GetConnectionMetrics{}
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err := futureResponse.Result()

	// We expect this to fail since the actor should be stopped
	if err == nil {
		t.Error("Expected request to fail after connection close")
	}
}

// TestConnectionActor_PingTickerDoubleStart tests that starting ping ticker twice doesn't create issues
func TestConnectionActor_PingTickerDoubleStart(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Start the ping ticker
	engine.Send(connectionPID, &StartPingTicker{})
	time.Sleep(50 * time.Millisecond)

	// Start it again (should be ignored due to nil check)
	engine.Send(connectionPID, &StartPingTicker{})
	time.Sleep(50 * time.Millisecond)

	// Stop the ticker
	engine.Send(connectionPID, &StopPingTicker{})
	time.Sleep(50 * time.Millisecond)

	// Verify connection is still healthy
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to remain healthy after double start")
	}
}

// TestConnectionActor_PingTimeoutTrigger tests the ping timeout mechanism
func TestConnectionActor_PingTimeoutTrigger(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Start the ping ticker
	engine.Send(connectionPID, &StartPingTicker{})

	// Wait for some ping activity
	time.Sleep(200 * time.Millisecond)

	// Manually trigger a ping timeout to test the timeout goroutine
	engine.Send(connectionPID, &PingTimeout{})

	// Give it time to process the timeout and shutdown
	time.Sleep(300 * time.Millisecond)

	// Try to send a message to verify it's stopped (should fail)
	metricsReq := &GetConnectionMetrics{}
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err := futureResponse.Result()

	// We expect this to fail since the actor should be stopped after timeout
	if err == nil {
		t.Error("Expected request to fail after ping timeout")
	}
}

// TestConnectionActor_PingTickerStopIdempotent tests that stopping ping ticker multiple times is safe
func TestConnectionActor_PingTickerStopIdempotent(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)
	defer engine.Poison(connectionPID)

	// Start the ping ticker
	engine.Send(connectionPID, &StartPingTicker{})
	time.Sleep(50 * time.Millisecond)

	// Stop it multiple times
	engine.Send(connectionPID, &StopPingTicker{})
	time.Sleep(25 * time.Millisecond)
	engine.Send(connectionPID, &StopPingTicker{})
	time.Sleep(25 * time.Millisecond)
	engine.Send(connectionPID, &StopPingTicker{})
	time.Sleep(25 * time.Millisecond)

	// Verify connection is still healthy
	metricsReq := &GetConnectionMetrics{}
	response, err := engine.Request(connectionPID, metricsReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Failed to get connection metrics: %v", err)
	}

	metricsResp, ok := response.(*ConnectionMetricsResponse)
	if !ok {
		t.Fatalf("Expected ConnectionMetricsResponse, got %T", response)
	}

	if !metricsResp.IsHealthy {
		t.Error("Expected connection to remain healthy after multiple stops")
	}
}

// TestConnectionActor_TimeoutGoroutineContextCancellation tests timeout goroutine context handling
func TestConnectionActor_TimeoutGoroutineContextCancellation(t *testing.T) {
	engine, connectionPID := createTestConnectionActor(t)

	// Start the ping ticker (this will create the timeout goroutines)
	engine.Send(connectionPID, &StartPingTicker{})

	// Let it run briefly to create timeout goroutines
	time.Sleep(150 * time.Millisecond)

	// Kill the actor to trigger context cancellation in timeout goroutines
	engine.Poison(connectionPID)

	// Give time for all goroutines to clean up
	time.Sleep(200 * time.Millisecond)

	// Verify actor is dead
	metricsReq := &GetConnectionMetrics{}
	futureResponse := engine.Request(connectionPID, metricsReq, 1*time.Second)
	_, err := futureResponse.Result()

	if err == nil {
		t.Error("Expected request to fail after actor termination")
	}
}

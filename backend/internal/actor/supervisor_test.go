package actor

import (
	"sync"
	"testing"
	"time"

	"github.com/anthdm/hollywood/actor"
)

/*
==========================================================================================================================

	TESTING SETUP

==========================================================================================================================
*/
type testingInterface interface {
	Fatalf(format string, args ...interface{})
}

// Test helper to create a test engine
func createTestEngine(t testingInterface) *actor.Engine {
	engine, err := actor.NewEngine(actor.NewEngineConfig())
	if err != nil {
		t.Fatalf("Failed to create test engine: %v", err)
	}
	return engine
}

// Test helper to create and start a supervisor
func createTestSupervisor(t testingInterface, config SupervisorConfig) (*actor.Engine, *actor.PID) {
	engine := createTestEngine(t)
	supervisorPID := engine.Spawn(NewSupervisorActor(config), "test-supervisor")

	// Give it more time to start and spawn children
	time.Sleep(250 * time.Millisecond)

	return engine, supervisorPID
}

/*
==========================================================================================================================

	CONFIG TESTING

==========================================================================================================================
*/
// TestDefaultSupervisorConfig tests the default configuration
func TestDefaultSupervisorConfig(t *testing.T) {
	config := DefaultSupervisorConfig()

	if config.MaxRestarts != 5 {
		t.Errorf("Expected MaxRestarts to be 5, got %d", config.MaxRestarts)
	}

	if config.RestartWindow != time.Minute {
		t.Errorf("Expected RestartWindow to be 1 minute, got %v", config.RestartWindow)
	}

	if config.ShutdownTimeout != 30*time.Second {
		t.Errorf("Expected ShutdownTimeout to be 30 seconds, got %v", config.ShutdownTimeout)
	}
}

// TestCustomSupervisorConfig tests custom configuration
func TestCustomSupervisorConfig(t *testing.T) {
	customConfig := SupervisorConfig{
		MaxRestarts:     10,
		RestartWindow:   2 * time.Minute,
		ShutdownTimeout: 60 * time.Second,
	}

	engine, supervisorPID := createTestSupervisor(t, customConfig)
	defer engine.Poison(supervisorPID)

	// Verify the supervisor was created successfully
	if supervisorPID == nil {
		t.Fatal("Failed to create supervisor with custom config")
	}
}

// TestNewSupervisorActor tests the supervisor actor creation
func TestNewSupervisorActor(t *testing.T) {
	config := DefaultSupervisorConfig()
	producer := NewSupervisorActor(config)

	if producer == nil {
		t.Fatal("NewSupervisorActor returned nil producer")
	}

	// Test that the producer creates a receiver
	receiver := producer()
	if receiver == nil {
		t.Fatal("Producer returned nil receiver")
	}

	// Verify it's the correct type
	if _, ok := receiver.(*SupervisorActor); !ok {
		t.Fatal("Producer did not return SupervisorActor")
	}
}

// TestSupervisorStartup tests the supervisor startup process
func TestSupervisorStartup(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())
	defer engine.Poison(supervisorPID)

	// Test health check to verify children were spawned
	healthReq := &HealthCheckRequest{}
	response, err := engine.Request(supervisorPID, healthReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}

	healthResp, ok := response.(*HealthCheckResponse)
	if !ok {
		t.Fatalf("Expected HealthCheckResponse, got %T", response)
	}

	if !healthResp.Healthy {
		t.Error("Expected system to be healthy")
	}

	// Verify all expected children are present
	expectedChildren := []string{"user-manager", "llm-manager", "tools-manager"}
	if len(healthResp.Children) != len(expectedChildren) {
		t.Errorf("Expected %d children, got %d", len(expectedChildren), len(healthResp.Children))
	}

	for _, childName := range expectedChildren {
		if child, exists := healthResp.Children[childName]; !exists {
			t.Errorf("Expected child %s not found", childName)
		} else if !child.Healthy {
			t.Errorf("Expected child %s to be healthy", childName)
		}
	}
}

// TestHealthCheckResponse tests health check functionality
func TestHealthCheckResponse(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())
	defer engine.Poison(supervisorPID)

	// Send health check request
	healthReq := &HealthCheckRequest{}
	response, err := engine.Request(supervisorPID, healthReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}

	healthResp, ok := response.(*HealthCheckResponse)
	if !ok {
		t.Fatalf("Expected HealthCheckResponse, got %T", response)
	}

	// Verify response structure
	if healthResp.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if len(healthResp.Children) == 0 {
		t.Error("Expected children in health response")
	}

	// Verify child health structure
	for name, child := range healthResp.Children {
		if child.Name != name {
			t.Errorf("Child name mismatch: expected %s, got %s", name, child.Name)
		}

		if child.Uptime <= 0 {
			t.Errorf("Expected positive uptime for child %s", name)
		}

		if child.Restarts < 0 {
			t.Errorf("Expected non-negative restart count for child %s", name)
		}
	}
}

// TestSystemStatusResponse tests system status functionality
func TestSystemStatusResponse(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())
	defer engine.Poison(supervisorPID)

	// Send system status request
	statusReq := &GetSystemStatusRequest{}
	response, err := engine.Request(supervisorPID, statusReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("System status request failed: %v", err)
	}

	statusResp, ok := response.(*SystemStatusResponse)
	if !ok {
		t.Fatalf("Expected SystemStatusResponse, got %T", response)
	}

	// Verify response structure
	if statusResp.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	expectedChildCount := 3 // user-manager, llm-manager, tools-manager
	if statusResp.TotalChildren != expectedChildCount {
		t.Errorf("Expected %d total children, got %d", expectedChildCount, statusResp.TotalChildren)
	}

	if statusResp.IsShuttingDown {
		t.Error("Expected system not to be shutting down")
	}

	if len(statusResp.Children) != expectedChildCount {
		t.Errorf("Expected %d children in details, got %d", expectedChildCount, len(statusResp.Children))
	}

	// Verify child details
	for _, child := range statusResp.Children {
		if child.Name == "" {
			t.Error("Expected non-empty child name")
		}

		if child.PID == nil {
			t.Errorf("Expected non-nil PID for child %s", child.Name)
		}

		if child.StartTime.IsZero() {
			t.Errorf("Expected non-zero start time for child %s", child.Name)
		}

		if child.Producer == nil {
			t.Errorf("Expected non-nil producer for child %s", child.Name)
		}
	}
}

// TestShutdownRequest tests graceful shutdown
func TestShutdownRequest(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())

	// Send shutdown request
	shutdownReq := &ShutdownRequest{Reason: "test shutdown"}
	engine.Send(supervisorPID, shutdownReq)

	// Give it time to process shutdown
	time.Sleep(200 * time.Millisecond)

	// Verify system status shows shutting down
	statusReq := &GetSystemStatusRequest{}
	response, err := engine.Request(supervisorPID, statusReq, 5*time.Second).Result()
	if err == nil {
		if statusResp, ok := response.(*SystemStatusResponse); ok {
			if !statusResp.IsShuttingDown {
				t.Error("Expected system to be shutting down after shutdown request")
			}
		}
	}
	// Note: The actor might be stopped before we can check, which is also valid
}

// TestConcurrentHealthChecks tests concurrent access to health checks
func TestConcurrentHealthChecks(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())
	defer engine.Poison(supervisorPID)

	const numConcurrentRequests = 10
	var wg sync.WaitGroup
	results := make(chan *HealthCheckResponse, numConcurrentRequests)
	errors := make(chan error, numConcurrentRequests)

	// Launch concurrent health check requests
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			healthReq := &HealthCheckRequest{}
			response, err := engine.Request(supervisorPID, healthReq, 5*time.Second).Result()
			if err != nil {
				errors <- err
				return
			}

			if healthResp, ok := response.(*HealthCheckResponse); ok {
				results <- healthResp
			} else {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent health check failed: %v", err)
		}
	}

	// Verify all responses are valid
	responseCount := 0
	for healthResp := range results {
		responseCount++
		if !healthResp.Healthy {
			t.Error("Expected healthy response in concurrent test")
		}
		if len(healthResp.Children) != 3 {
			t.Errorf("Expected 3 children, got %d", len(healthResp.Children))
		}
	}

	if responseCount != numConcurrentRequests {
		t.Errorf("Expected %d responses, got %d", numConcurrentRequests, responseCount)
	}
}

// TestConcurrentSystemStatus tests concurrent access to system status
func TestConcurrentSystemStatus(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())
	defer engine.Poison(supervisorPID)

	const numConcurrentRequests = 10
	var wg sync.WaitGroup
	results := make(chan *SystemStatusResponse, numConcurrentRequests)
	errors := make(chan error, numConcurrentRequests)

	// Launch concurrent status requests
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			statusReq := &GetSystemStatusRequest{}
			response, err := engine.Request(supervisorPID, statusReq, 5*time.Second).Result()
			if err != nil {
				errors <- err
				return
			}

			if statusResp, ok := response.(*SystemStatusResponse); ok {
				results <- statusResp
			} else {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		if err != nil {
			t.Errorf("Concurrent status check failed: %v", err)
		}
	}

	// Verify all responses are valid
	responseCount := 0
	for statusResp := range results {
		responseCount++
		if statusResp.TotalChildren != 3 {
			t.Errorf("Expected 3 total children, got %d", statusResp.TotalChildren)
		}
		if len(statusResp.Children) != 3 {
			t.Errorf("Expected 3 children details, got %d", len(statusResp.Children))
		}
	}

	if responseCount != numConcurrentRequests {
		t.Errorf("Expected %d responses, got %d", numConcurrentRequests, responseCount)
	}
}

// TestUnknownMessageType tests handling of unknown messages
func TestUnknownMessageType(t *testing.T) {
	engine, supervisorPID := createTestSupervisor(t, DefaultSupervisorConfig())
	defer engine.Poison(supervisorPID)

	// Send an unknown message type
	unknownMsg := "unknown message"
	engine.Send(supervisorPID, unknownMsg)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Verify system still responds to known messages
	healthReq := &HealthCheckRequest{}
	response, err := engine.Request(supervisorPID, healthReq, 5*time.Second).Result()
	if err != nil {
		t.Fatalf("Health check after unknown message failed: %v", err)
	}

	if _, ok := response.(*HealthCheckResponse); !ok {
		t.Error("Expected system to still respond normally after unknown message")
	}
}

// TestChildActorInfo tests the ChildActorInfo structure
func TestChildActorInfo(t *testing.T) {
	engine := createTestEngine(t)
	defer engine.Poison(nil) // Close engine properly

	// Create a test child actor info
	testPID := &actor.PID{}
	testProducer := func() actor.Receiver {
		return &UserManagerActor{users: make(map[string]*actor.PID)}
	}

	info := &ChildActorInfo{
		PID:         testPID,
		Name:        "test-child",
		StartTime:   time.Now(),
		Restarts:    0,
		LastRestart: time.Time{},
		Producer:    testProducer,
	}

	// Verify structure fields
	if info.PID != testPID {
		t.Error("PID not set correctly")
	}

	if info.Name != "test-child" {
		t.Error("Name not set correctly")
	}

	if info.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}

	if info.Restarts != 0 {
		t.Error("Initial restarts should be 0")
	}

	if !info.LastRestart.IsZero() {
		t.Error("Initial LastRestart should be zero")
	}

	if info.Producer == nil {
		t.Error("Producer should not be nil")
	}
}

// TestMessageTypes tests the message type structures
func TestMessageTypes(t *testing.T) {
	// Test ShutdownRequest
	shutdownReq := &ShutdownRequest{Reason: "test reason"}
	if shutdownReq.Reason != "test reason" {
		t.Error("ShutdownRequest reason not set correctly")
	}

	// Test HealthCheckResponse
	healthResp := &HealthCheckResponse{
		Healthy:   true,
		Children:  make(map[string]ChildHealth),
		Timestamp: time.Now(),
	}

	if !healthResp.Healthy {
		t.Error("HealthCheckResponse healthy not set correctly")
	}

	if healthResp.Children == nil {
		t.Error("HealthCheckResponse children should not be nil")
	}

	if healthResp.Timestamp.IsZero() {
		t.Error("HealthCheckResponse timestamp should not be zero")
	}

	// Test ChildHealth
	childHealth := ChildHealth{
		Name:     "test-child",
		Healthy:  true,
		Uptime:   time.Minute,
		Restarts: 0,
	}

	if childHealth.Name != "test-child" {
		t.Error("ChildHealth name not set correctly")
	}

	if !childHealth.Healthy {
		t.Error("ChildHealth healthy not set correctly")
	}

	if childHealth.Uptime != time.Minute {
		t.Error("ChildHealth uptime not set correctly")
	}

	if childHealth.Restarts != 0 {
		t.Error("ChildHealth restarts not set correctly")
	}
	// Test SystemStatusResponse
	statusResp := &SystemStatusResponse{
		Timestamp:      time.Now(),
		TotalChildren:  3,
		IsShuttingDown: false,
		Children:       make([]ChildActorInfo, 0),
	}

	if statusResp.Timestamp.IsZero() {
		t.Error("SystemStatusResponse timestamp should not be zero")
	}

	if statusResp.TotalChildren != 3 {
		t.Error("SystemStatusResponse total children not set correctly")
	}

	if statusResp.IsShuttingDown {
		t.Error("SystemStatusResponse should not be shutting down initially")
	}

	if statusResp.Children == nil {
		t.Error("SystemStatusResponse children should not be nil")
	}
}

// TestSupervisorGetters tests the getter methods
func TestSupervisorGetters(t *testing.T) {
	// Note: These methods are currently on the struct, not accessible externally
	// This test verifies the methods exist and would work if exposed
	config := DefaultSupervisorConfig()
	producer := NewSupervisorActor(config)
	receiver := producer()
	supervisor, ok := receiver.(*SupervisorActor)
	if !ok {
		t.Fatal("Failed to cast to SupervisorActor")
	}

	// Test getter methods (these return nil until actors are spawned)
	userManager := supervisor.GetUserManager()
	llmManager := supervisor.GetLLMManager()
	toolsManager := supervisor.GetToolsManager()

	// Before spawning, these should be nil
	if userManager != nil {
		t.Error("UserManager should be nil before spawning")
	}

	if llmManager != nil {
		t.Error("LLMManager should be nil before spawning")
	}

	if toolsManager != nil {
		t.Error("ToolsManager should be nil before spawning")
	}
}

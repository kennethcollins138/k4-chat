package actor

import "time"

// Message types for supervisor communication
type (
	ShutdownRequest struct {
		Reason string
	}

	HealthCheckRequest struct{}

	HealthCheckResponse struct {
		Healthy   bool
		Children  map[string]ChildHealth
		Timestamp time.Time
	}

	ChildHealth struct {
		Name     string
		Healthy  bool
		Uptime   time.Duration
		Restarts int
	}

	GetSystemStatusRequest struct{}

	SystemStatusResponse struct {
		Timestamp      time.Time
		TotalChildren  int
		IsShuttingDown bool
		Children       []ChildActorInfo
	}
)

package actor

import (
	"time"

	"github.com/google/uuid"
	"github.com/kdot/k4-chat/backend/internal/database/models"
)

// UserManager configuration and metrics types
// GOING TO MOVE TO RESPECTIVE CONFIG AND METRIC FILES
type (
	UserManagerConfig struct {
		MaxActiveUsers      int           // Maximum number of active UserActors
		IdleTimeout         time.Duration // Time before idle UserActor is considered for cleanup
		ShutdownTimeout     time.Duration // Maximum time to wait for UserActor shutdown
		HealthCheckInterval time.Duration // How often to check UserActor health
		CleanupInterval     time.Duration // How often to run cleanup tasks
	}

	UserManagerMetrics struct {
		TotalActiveUsers   int
		TotalConnections   int
		TotalSessions      int
		UsersCreated       int64
		UsersShutdown      int64
		HealthChecksFailed int64
		CleanupOperations  int64
	}
)

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

// User management messages
type (
	CreateUserRequest struct {
		Email       string
		Username    string
		Password    string
		DisplayName *string
	}

	CreateUserResponse struct {
		User  *models.User
		Error error
	}

	GetUserRequest struct {
		UserID uuid.UUID
	}

	GetUserResponse struct {
		User  *models.User
		Error error
	}

	UpdateUserLastActiveRequest struct {
		UserID uuid.UUID
	}

	UpdateUserLastActiveResponse struct {
		Error error
	}
)

// Chat session messages
type (
	CreateChatSessionRequest struct {
		UserID       uuid.UUID
		Title        string
		ModelName    string
		SystemPrompt *string
		Temperature  *float64
		MaxTokens    *int
	}

	CreateChatSessionResponse struct {
		Session *models.ChatSession
		Error   error
	}

	GetChatSessionRequest struct {
		SessionID uuid.UUID
		UserID    uuid.UUID
	}

	GetChatSessionResponse struct {
		Session *models.ChatSession
		Error   error
	}

	ListChatSessionsRequest struct {
		UserID uuid.UUID
		Limit  int
		Offset int
	}

	ListChatSessionsResponse struct {
		Sessions []models.ChatSession
		Error    error
	}
)

// Message management messages
type (
	CreateMessageRequest struct {
		SessionID       uuid.UUID
		ParentMessageID *uuid.UUID
		Role            string
		Content         string
	}

	CreateMessageResponse struct {
		Message *models.Message
		Error   error
	}

	GetChatMessagesRequest struct {
		SessionID uuid.UUID
		Limit     int
		Offset    int
	}

	GetChatMessagesResponse struct {
		Messages []models.Message
		Error    error
	}
)

// User actor messages
type (
	ConnectionRejected struct {
		Reason string
		UserID string
	}
	UserConnectionEstablished struct {
		ConnectionID string
		DeviceInfo   *string
		IPAddress    *string
	}

	UserConnectionClosed struct {
		ConnectionID string
	}

	StartChatSession struct {
		SessionID uuid.UUID
		ModelName string
	}

	StopChatSession struct {
		SessionID uuid.UUID
	}

	SendMessage struct {
		SessionID       uuid.UUID
		ParentMessageID *uuid.UUID
		Role            string
		Content         string
	}

	// New message types for enhanced UserActor
	GetUserInfoRequest struct{}

	UserInfoResponse struct {
		UserID          string
		ConnectionCount int
		SessionCount    int
		LastActiveAt    time.Time
		IsOnline        bool
	}

	ConsiderShutdown struct{}

	// Session broadcast messages
	SessionStarted struct {
		SessionID uuid.UUID
		ModelName string
	}

	SessionStopped struct {
		SessionID uuid.UUID
	}

	SessionCreated struct {
		Session models.ChatSession
	}

	// User state broadcast message
	UserStateUpdate struct {
		UserID          string
		IsOnline        bool
		ConnectionCount int
		SessionCount    int
		LastActiveAt    time.Time
	}
)

// LLM manager messages
type (
	LLMStreamRequest struct {
		SessionID    uuid.UUID
		MessageID    uuid.UUID
		ModelName    string
		Messages     []models.Message
		Temperature  float64
		MaxTokens    int
		SystemPrompt *string
	}

	LLMStreamResponse struct {
		SessionID  uuid.UUID
		MessageID  uuid.UUID
		Content    string
		IsFinished bool
		Error      error
	}

	GetAvailableModelsRequest struct{}

	GetAvailableModelsResponse struct {
		Models []LLMModel
		Error  error
	}
)

// LLM model info
type LLMModel struct {
	Name        string  `json:"name"`
	Provider    string  `json:"provider"`
	Description string  `json:"description"`
	MaxTokens   int     `json:"max_tokens"`
	CostPer1K   float64 `json:"cost_per_1k"`
}

// Tools manager messages
type (
	WebSearchRequest struct {
		Query      string
		MaxResults int
	}

	WebSearchResponse struct {
		Results []SearchResult
		Error   error
	}

	ImageGenerationRequest struct {
		Prompt    string
		ModelName string
		Size      string
	}

	ImageGenerationResponse struct {
		ImageURL     string
		ThumbnailURL *string
		Width        *int
		Height       *int
		Error        error
	}
)

// Search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// Connection messages
type (
	WebSocketMessage struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}

	ChatMessageReceived struct {
		SessionID uuid.UUID `json:"session_id"`
		Content   string    `json:"content"`
	}

	StreamingUpdate struct {
		SessionID  uuid.UUID `json:"session_id"`
		MessageID  uuid.UUID `json:"message_id"`
		Content    string    `json:"content"`
		IsComplete bool      `json:"is_complete"`
	}

	// ConnectionActor specific messages
	ConnectionHeartbeat struct {
		Timestamp time.Time
	}

	ConnectionMetrics struct {
		BytesSent     int64
		BytesReceived int64
		MessagesSent  int64
	}

	ConnectionReady struct {
		ConnectionID string
		UserID       string
		ConnectedAt  time.Time
	}

	ForwardToConnection struct {
		ConnectionID string
		Message      interface{}
	}

	ForwardToUser struct {
		UserID  string
		Message interface{}
	}

	GetConnectionMetrics struct{}

	ConnectionMetricsResponse struct {
		ConnectionID string
		UserID       string
		ConnectedAt  time.Time
		LastPingAt   time.Time
		LastPongAt   time.Time
		IsHealthy    bool
		Metrics      ConnectionMetrics
		DeviceInfo   *string
		IPAddress    *string
	}

	CloseConnection struct {
		Reason string
	}

	PingTimeout struct{}

	StartPingTicker struct{}

	StopPingTicker struct{}
)

// UserManager specific messages
type (
	GetUserManagerStatusRequest struct{}

	UserManagerStatusResponse struct {
		Timestamp        time.Time
		TotalActiveUsers int
		TotalConnections int
		TotalSessions    int
		IsShuttingDown   bool
		ActiveUsers      []UserActorStatus
		Config           UserManagerConfig
	}

	UserActorStatus struct {
		UserID            string
		SpawnedAt         time.Time
		LastActiveAt      time.Time
		ConnectionCount   int
		SessionCount      int
		IsHealthy         bool
		ShutdownRequested bool
	}

	GetUserManagerMetricsRequest struct{}

	UserManagerMetricsResponse struct {
		Timestamp time.Time
		Metrics   UserManagerMetrics
	}

	CleanupIdleUsersRequest struct{}

	// Internal ticker messages
	HealthCheckTick struct{}

	CleanupTick struct{}
)

package actor

import (
	"log"
	"time"

	"github.com/anthdm/hollywood/actor"
)

/*
ConnectionActor manages a single WebSocket connection for a user.

Key Responsibilities:
- Manage WebSocket connection lifecycle
- Handle heartbeat/ping-pong to detect disconnections
- Forward messages between WebSocket and UserActor
- Track connection metrics and health
- Handle graceful connection shutdown

Design Notes:
- One ConnectionActor per WebSocket connection (browser tab/device)
- A user can have multiple ConnectionActors (multi-device support)
- Each connection has a unique connectionID
- WebSocket integration will be added when WebSocket layer is ready

Relationship:
UserActor (1) -> ConnectionActor (N) -> WebSocket (1)
UserActor (1) -> ChatSessionActor (N)
*/
type ConnectionActor struct {
	connectionID string    // Unique identifier for this WebSocket connection
	userID       string    // ID of the user who owns this connection
	deviceInfo   *string   // Optional device/browser information
	ipAddress    *string   // Optional IP address for security/analytics
	connectedAt  time.Time // When the connection was established
	lastPingAt   time.Time // Last heartbeat received
	lastPongAt   time.Time // Last heartbeat sent

	// Connection metrics
	metrics ConnectionMetrics

	// TODO: WebSocket connection (will be added when WebSocket layer is ready)
	// websocket *websocket.Conn

	// Connection state
	isHealthy   bool
	isClosing   bool
	pingTicker  *time.Ticker
	pingTimeout time.Duration
}

// ConnectionConfig holds configuration for ConnectionActor
type ConnectionConfig struct {
	PingInterval time.Duration // How often to send ping
	PingTimeout  time.Duration // How long to wait for pong
	WriteTimeout time.Duration // WebSocket write timeout
	ReadTimeout  time.Duration // WebSocket read timeout
}

// DefaultConnectionConfig returns sensible defaults
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		PingInterval: 30 * time.Second,
		PingTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  60 * time.Second,
	}
}

// NewConnectionActor creates a new connection actor
func NewConnectionActor(connectionID, userID string, deviceInfo, ipAddress *string) actor.Producer {
	return func() actor.Receiver {
		now := time.Now()
		config := DefaultConnectionConfig()

		return &ConnectionActor{
			connectionID: connectionID,
			userID:       userID,
			deviceInfo:   deviceInfo,
			ipAddress:    ipAddress,
			connectedAt:  now,
			lastPingAt:   now,
			lastPongAt:   now,
			metrics: ConnectionMetrics{
				BytesSent:     0,
				BytesReceived: 0,
				MessagesSent:  0,
			},
			isHealthy:   true,
			isClosing:   false,
			pingTimeout: config.PingTimeout,
		}
	}
}

// Receive handles all messages sent to the connection actor
func (c *ConnectionActor) Receive(ctx *actor.Context) {
	switch msg := ctx.Message().(type) {
	case *actor.Started:
		c.handleStarted(ctx)

	case *actor.Stopped:
		c.handleStopped(ctx)

	case *ConnectionHeartbeat:
		c.handleHeartbeat(ctx, msg)

	case *ForwardToConnection:
		c.handleForwardToConnection(ctx, msg)

	case *GetConnectionMetrics:
		c.handleGetMetrics(ctx, msg)

	case *CloseConnection:
		c.handleCloseConnection(ctx, msg)

	case *PingTimeout:
		c.handlePingTimeout(ctx)

	case *StartPingTicker:
		c.handleStartPingTicker(ctx)

	case *StopPingTicker:
		c.handleStopPingTicker(ctx)

	// TODO: WebSocket messages (will be implemented when WebSocket layer is ready)
	case *WebSocketMessage:
		c.handleWebSocketMessage(ctx, msg)

	default:
		log.Printf("ConnectionActor[%s]: Unknown message type: %T", c.connectionID, msg)
	}
}

// handleStarted initializes the connection and starts heartbeat
func (c *ConnectionActor) handleStarted(ctx *actor.Context) {
	log.Printf("ConnectionActor[%s]: Started for user %s", c.connectionID, c.userID)

	// Start ping ticker for heartbeat
	c.startPingTicker(ctx)

	// Notify parent (UserActor) that connection is ready
	ctx.Send(ctx.Parent(), &ConnectionReady{
		ConnectionID: c.connectionID,
		UserID:       c.userID,
		ConnectedAt:  c.connectedAt,
	})
}

// handleStopped cleans up when connection is stopped
func (c *ConnectionActor) handleStopped(ctx *actor.Context) {
	log.Printf("ConnectionActor[%s]: Stopped", c.connectionID)

	// Clean up ping ticker
	c.stopPingTicker()

	// Notify parent that connection is closed
	ctx.Send(ctx.Parent(), &UserConnectionClosed{
		ConnectionID: c.connectionID,
	})
}

// handleHeartbeat processes ping/pong messages
func (c *ConnectionActor) handleHeartbeat(ctx *actor.Context, msg *ConnectionHeartbeat) {
	c.lastPingAt = msg.Timestamp
	c.isHealthy = true

	log.Printf("ConnectionActor[%s]: Heartbeat received", c.connectionID)

	// Send pong response
	c.lastPongAt = time.Now()
	// TODO: Send pong to WebSocket when implemented
}

// handleForwardToConnection forwards messages to the WebSocket
func (c *ConnectionActor) handleForwardToConnection(ctx *actor.Context, msg *ForwardToConnection) {
	if c.isClosing {
		log.Printf("ConnectionActor[%s]: Dropping message, connection is closing", c.connectionID)
		return
	}

	// TODO: Send message to WebSocket when implemented
	log.Printf("ConnectionActor[%s]: Forwarding message to WebSocket: %T", c.connectionID, msg.Message)

	// Update metrics
	c.metrics.MessagesSent++
	// c.metrics.BytesSent += len(serialized message)
}

// handleWebSocketMessage processes messages received from WebSocket
func (c *ConnectionActor) handleWebSocketMessage(ctx *actor.Context, msg *WebSocketMessage) {
	if c.isClosing {
		return
	}

	// Update metrics
	c.metrics.MessagesSent++
	// c.metrics.BytesReceived += len(msg data)

	// Forward to UserActor
	ctx.Send(ctx.Parent(), &ForwardToUser{
		UserID:  c.userID,
		Message: msg,
	})

	log.Printf("ConnectionActor[%s]: Forwarded WebSocket message to UserActor", c.connectionID)
}

// handleGetMetrics returns current connection metrics
func (c *ConnectionActor) handleGetMetrics(ctx *actor.Context, msg *GetConnectionMetrics) {
	response := &ConnectionMetricsResponse{
		ConnectionID: c.connectionID,
		UserID:       c.userID,
		ConnectedAt:  c.connectedAt,
		LastPingAt:   c.lastPingAt,
		LastPongAt:   c.lastPongAt,
		IsHealthy:    c.isHealthy,
		Metrics:      c.metrics,
		DeviceInfo:   c.deviceInfo,
		IPAddress:    c.ipAddress,
	}

	ctx.Respond(response)
}

// handleCloseConnection initiates graceful connection shutdown
func (c *ConnectionActor) handleCloseConnection(ctx *actor.Context, msg *CloseConnection) {
	log.Printf("ConnectionActor[%s]: Initiating graceful shutdown: %s", c.connectionID, msg.Reason)

	c.isClosing = true

	// Stop heartbeat
	c.stopPingTicker()

	// TODO: Close WebSocket connection when implemented
	// c.websocket.Close()

	// Stop the actor
	// In Hollywood, the actor stops automatically when the receive method returns
	// or when the parent actor terminates ith
	ctx.Engine().Stop(ctx.PID())
}

// handlePingTimeout handles ping timeout (connection assumed dead)
func (c *ConnectionActor) handlePingTimeout(ctx *actor.Context) {
	log.Printf("ConnectionActor[%s]: Ping timeout, connection assumed dead", c.connectionID)

	c.isHealthy = false

	// Close the connection
	ctx.Send(ctx.PID(), &CloseConnection{
		Reason: "ping timeout",
	})
}

// handleStartPingTicker starts the heartbeat ticker
func (c *ConnectionActor) handleStartPingTicker(ctx *actor.Context) {
	c.startPingTicker(ctx)
}

// handleStopPingTicker stops the heartbeat ticker
func (c *ConnectionActor) handleStopPingTicker(ctx *actor.Context) {
	c.stopPingTicker()
}

// startPingTicker starts the periodic heartbeat
func (c *ConnectionActor) startPingTicker(ctx *actor.Context) {
	if c.pingTicker != nil {
		return // Already started
	}

	config := DefaultConnectionConfig()
	c.pingTicker = time.NewTicker(config.PingInterval)

	// Start goroutine to handle ping ticks
	go func() {
		for {
			select {
			case <-c.pingTicker.C:
				if c.isClosing {
					return
				}

				// Send ping
				ctx.Send(ctx.PID(), &ConnectionHeartbeat{
					Timestamp: time.Now(),
				})

				// Schedule timeout check using a goroutine and timer
				go func() {
					timer := time.NewTimer(c.pingTimeout)
					defer timer.Stop()
					select {
					case <-timer.C:
						if !c.isClosing {
							ctx.Send(ctx.PID(), &PingTimeout{})
						}
					case <-ctx.Context().Done():
						return
					}
				}()

			case <-ctx.Context().Done():
				return
			}
		}
	}()
}

// stopPingTicker stops the heartbeat ticker
func (c *ConnectionActor) stopPingTicker() {
	if c.pingTicker != nil {
		c.pingTicker.Stop()
		c.pingTicker = nil
	}
}

// GetConnectionID returns the connection ID
func (c *ConnectionActor) GetConnectionID() string {
	return c.connectionID
}

// GetUserID returns the user ID
func (c *ConnectionActor) GetUserID() string {
	return c.userID
}

// IsHealthy returns the connection health status
func (c *ConnectionActor) IsHealthy() bool {
	return c.isHealthy && !c.isClosing
}

// GetMetrics returns current metrics
func (c *ConnectionActor) GetMetrics() ConnectionMetrics {
	return c.metrics
}

// GetUptime returns how long the connection has been active
func (c *ConnectionActor) GetUptime() time.Duration {
	return time.Since(c.connectedAt)
}

package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kdot/k4-chat/backend/internal/database/models"
)

// DB wraps the database connection pool and provides query methods
type DB struct {
	pool *pgxpool.Pool
}

// Config holds database configuration
type Config struct {
	Host        string
	Port        int
	User        string
	Password    string
	Database    string
	SSLMode     string
	MaxConns    int32
	MinConns    int32
	MaxConnTime time.Duration
	MaxIdleTime time.Duration
	HealthCheck time.Duration
}

// DefaultConfig returns sensible defaults for database configuration
func DefaultConfig() Config {
	return Config{
		Host:        "localhost",
		Port:        5432,
		User:        "postgres",
		Password:    "postgres",
		Database:    "k4chat",
		SSLMode:     "disable",
		MaxConns:    25,
		MinConns:    5,
		MaxConnTime: time.Hour,
		MaxIdleTime: time.Minute * 30,
		HealthCheck: time.Minute,
	}
}

// NewDB creates a new database connection pool
func NewDB(config Config) (*DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.User, config.Password, config.Host, config.Port, config.Database, config.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = config.MaxConns
	poolConfig.MinConns = config.MinConns
	poolConfig.MaxConnLifetime = config.MaxConnTime
	poolConfig.MaxConnIdleTime = config.MaxIdleTime
	poolConfig.HealthCheckPeriod = config.HealthCheck

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Database connection established: %s:%d/%s", config.Host, config.Port, config.Database)

	return &DB{pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
		log.Println("Database connection pool closed")
	}
}

// Health checks database connectivity
func (db *DB) Health(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (db *DB) Stats() *pgxpool.Stat {
	return db.pool.Stat()
}

// ===== USER OPERATIONS =====

// CreateUser creates a new user
func (db *DB) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
	user := &models.User{
		ID:          uuid.New(),
		Email:       req.Email,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		IsActive:    true,
	}

	query := `
		INSERT INTO users (id, email, username, password_hash, display_name, created_at, updated_at, last_active_at, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at, last_active_at`

	err := db.pool.QueryRow(ctx, query,
		user.ID, user.Email, user.Username, req.Password, // TODO: Hash password
		user.DisplayName, user.CreatedAt, user.UpdatedAt, user.CreatedAt, user.IsActive,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt, &user.LastActiveAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, email, username, display_name, avatar_url, created_at, updated_at, last_active_at, is_active
		FROM users WHERE id = $1 AND is_active = true`

	err := db.pool.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.Username, &user.DisplayName, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt, &user.LastActiveAt, &user.IsActive,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by email
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, email, username, password_hash, display_name, avatar_url, created_at, updated_at, last_active_at, is_active
		FROM users WHERE email = $1 AND is_active = true`

	err := db.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.DisplayName, &user.AvatarURL,
		&user.CreatedAt, &user.UpdatedAt, &user.LastActiveAt, &user.IsActive,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// UpdateUserLastActive updates the user's last active timestamp
func (db *DB) UpdateUserLastActive(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET last_active_at = NOW() WHERE id = $1`

	result, err := db.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to update user last active: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrUserNotFound
	}

	return nil
}

// ===== CHAT SESSION OPERATIONS =====

// CreateChatSession creates a new chat session
func (db *DB) CreateChatSession(ctx context.Context, userID uuid.UUID, req models.CreateChatSessionRequest) (*models.ChatSession, error) {
	now := time.Now()
	session := &models.ChatSession{
		ID:               uuid.New(),
		UserID:           userID,
		Title:            req.Title,
		ModelName:        req.ModelName,
		SystemPrompt:     req.SystemPrompt,
		Temperature:      0.7,  // Default
		MaxTokens:        4000, // Default
		Status:           "active",
		CreatedAt:        now,
		UpdatedAt:        now,
		LastInteractedAt: now,
		IsPinned:         false,
		IsFavorite:       false,
		Tags:             []string{},
	}

	// Override defaults if provided
	if req.Temperature != nil {
		session.Temperature = *req.Temperature
	}
	if req.MaxTokens != nil {
		session.MaxTokens = *req.MaxTokens
	}

	query := `
		INSERT INTO chat_sessions (id, user_id, title, model_name, system_prompt, temperature, max_tokens, 
		                          status, created_at, updated_at, last_interacted_at, is_pinned, is_favorite, tags)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at, last_interacted_at`

	err := db.pool.QueryRow(ctx, query,
		session.ID, session.UserID, session.Title, session.ModelName, session.SystemPrompt,
		session.Temperature, session.MaxTokens, session.Status, session.CreatedAt, session.UpdatedAt,
		session.LastInteractedAt, session.IsPinned, session.IsFavorite, session.Tags,
	).Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt, &session.LastInteractedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}

	return session, nil
}

// GetChatSession retrieves a chat session by ID
func (db *DB) GetChatSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (*models.ChatSession, error) {
	session := &models.ChatSession{}
	query := `
		SELECT id, user_id, title, model_name, system_prompt, temperature, max_tokens, 
		       model_config, parent_session_id, branch_label, status, created_at, updated_at, 
		       last_interacted_at, archived_at, is_pinned, is_favorite, tags, extensions
		FROM chat_sessions 
		WHERE id = $1 AND user_id = $2 AND status != 'deleted'`

	err := db.pool.QueryRow(ctx, query, sessionID, userID).Scan(
		&session.ID, &session.UserID, &session.Title, &session.ModelName, &session.SystemPrompt,
		&session.Temperature, &session.MaxTokens, &session.ModelConfig, &session.ParentSessionID,
		&session.BranchLabel, &session.Status, &session.CreatedAt, &session.UpdatedAt,
		&session.LastInteractedAt, &session.ArchivedAt, &session.IsPinned, &session.IsFavorite,
		&session.Tags, &session.Extensions,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrChatSessionNotFound
		}
		return nil, fmt.Errorf("failed to get chat session: %w", err)
	}

	return session, nil
}

// ListChatSessions retrieves all chat sessions for a user
func (db *DB) ListChatSessions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.ChatSession, error) {
	query := `
		SELECT id, user_id, title, model_name, system_prompt, temperature, max_tokens,
		       model_config, parent_session_id, branch_label, status, created_at, updated_at, 
		       last_interacted_at, archived_at, is_pinned, is_favorite, tags, extensions
		FROM chat_sessions 
		WHERE user_id = $1 AND status != 'deleted'
		ORDER BY is_pinned DESC, is_favorite DESC, last_interacted_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := db.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list chat sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.ChatSession
	for rows.Next() {
		var session models.ChatSession
		err := rows.Scan(
			&session.ID, &session.UserID, &session.Title, &session.ModelName, &session.SystemPrompt,
			&session.Temperature, &session.MaxTokens, &session.ModelConfig, &session.ParentSessionID,
			&session.BranchLabel, &session.Status, &session.CreatedAt, &session.UpdatedAt,
			&session.LastInteractedAt, &session.ArchivedAt, &session.IsPinned, &session.IsFavorite,
			&session.Tags, &session.Extensions,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat session: %w", err)
		}
		sessions = append(sessions, session)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chat sessions: %w", err)
	}

	return sessions, nil
}

// ===== MESSAGE OPERATIONS =====

// CreateMessage creates a new message
func (db *DB) CreateMessage(ctx context.Context, sessionID uuid.UUID, req models.CreateMessageRequest) (*models.Message, error) {
	message := &models.Message{
		ID:              uuid.New(),
		ChatSessionID:   sessionID,
		ParentMessageID: req.ParentMessageID,
		Role:            req.Role,
		Content:         req.Content,
		CreatedAt:       time.Now(),
		BranchIndex:     0,
		IsSelected:      true,
	}

	query := `
		INSERT INTO messages (id, chat_session_id, parent_message_id, role, content, created_at, branch_index, is_selected)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`

	err := db.pool.QueryRow(ctx, query,
		message.ID, message.ChatSessionID, message.ParentMessageID, message.Role,
		message.Content, message.CreatedAt, message.BranchIndex, message.IsSelected,
	).Scan(&message.ID, &message.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	return message, nil
}

// GetChatMessages retrieves all messages for a chat session
func (db *DB) GetChatMessages(ctx context.Context, sessionID uuid.UUID, limit, offset int) ([]models.Message, error) {
	query := `
		SELECT id, chat_session_id, parent_message_id, role, content, token_count, 
		       model_name, finish_reason, created_at, branch_index, is_selected
		FROM messages 
		WHERE chat_session_id = $1 AND is_selected = true
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := db.pool.Query(ctx, query, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get chat messages: %w", err)
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var message models.Message
		err := rows.Scan(
			&message.ID, &message.ChatSessionID, &message.ParentMessageID, &message.Role,
			&message.Content, &message.TokenCount, &message.ModelName, &message.FinishReason,
			&message.CreatedAt, &message.BranchIndex, &message.IsSelected,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

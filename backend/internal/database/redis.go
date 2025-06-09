package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Config holds Redis configuration
type RedisConfig struct {
	URI            string
	PoolSize       int
	MinIdleConns   int
	MaxRetries     int
	DialTimeout    time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	PoolTimeout    time.Duration
	IdleTimeout    time.Duration
	MaxConnAge     time.Duration
	CircuitBreaker CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled          bool
	FailureThreshold int
	ResetTimeout     time.Duration
}

// DefaultConfig returns default Redis configuration
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		PoolSize:     10,
		MinIdleConns: 5,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
		IdleTimeout:  5 * time.Minute,
		MaxConnAge:   30 * time.Minute,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:          true,
			FailureThreshold: 5,
			ResetTimeout:     30 * time.Second,
		},
	}
}

// NewClient creates a new Redis client with optimized configuration
func NewClient(cfg *RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	if cfg == nil {
		cfg = DefaultRedisConfig()
	}

	opt, err := redis.ParseURL(cfg.URI)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URI: %w", err)
	}

	// Apply connection pool settings
	opt.PoolSize = cfg.PoolSize
	opt.MinIdleConns = cfg.MinIdleConns
	opt.MaxRetries = cfg.MaxRetries
	opt.DialTimeout = cfg.DialTimeout
	opt.ReadTimeout = cfg.ReadTimeout
	opt.WriteTimeout = cfg.WriteTimeout
	opt.PoolTimeout = cfg.PoolTimeout
	opt.ConnMaxIdleTime = cfg.IdleTimeout
	opt.ConnMaxLifetime = cfg.MaxConnAge

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Add connection pool metrics logging
	go monitorPoolStats(client, logger)

	return client, nil
}

// monitorPoolStats periodically logs Redis connection pool statistics
func monitorPoolStats(client *redis.Client, logger *zap.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		stats := client.PoolStats()
		logger.Info("Redis connection pool stats",
			zap.Uint32("total_conns", stats.TotalConns),
			zap.Uint32("idle_conns", stats.IdleConns),
			zap.Uint32("stale_conns", stats.StaleConns),
		)
	}
}

// BatchProcessor handles batch operations for Redis
type BatchProcessor struct {
	client *redis.Client
	logger *zap.Logger
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(client *redis.Client, logger *zap.Logger) *BatchProcessor {
	return &BatchProcessor{
		client: client,
		logger: logger,
	}
}

// Pipeline executes multiple Redis commands in a pipeline
func (bp *BatchProcessor) Pipeline(ctx context.Context, cmds []redis.Cmder) error {
	pipe := bp.client.Pipeline()
	for _, cmd := range cmds {
		err := pipe.Process(ctx, cmd)
		if err != nil {
			return err
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// MultiExec executes multiple Redis commands in a transaction
func (bp *BatchProcessor) MultiExec(ctx context.Context, fn func(pipe redis.Pipeliner) error) error {
	return bp.client.Watch(ctx, func(tx *redis.Tx) error {
		pipe := tx.Pipeline()
		if err := fn(pipe); err != nil {
			return err
		}
		_, err := pipe.Exec(ctx)
		return err
	})
}

package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/kdot/k4-chat/backend/configs"
)

// NewClient creates a new Redis client from the unified RedisConfig
func NewClient(cfg configs.RedisConfig, logger *zap.Logger) (*redis.Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("redis is disabled in configuration")
	}

	uri := fmt.Sprintf("redis://%s:%s@%s:%d/%d", cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	opt, err := redis.ParseURL(uri)
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

// NewClientFromURI creates a Redis client with a simple URI
func NewClientFromURI(uri string, logger *zap.Logger) (*redis.Client, error) {
	opt, err := redis.ParseURL(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URI: %w", err)
	}

	client := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), opt.DialTimeout)
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

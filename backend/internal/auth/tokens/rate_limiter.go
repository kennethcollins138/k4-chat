package tokens

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter handles rate limiting for token operations
type RateLimiter struct {
	redisClient redis.Client
}

// NewRateLimiter creates a new RateLimiter
func NewRateLimiter(redisClient redis.Client) *RateLimiter {
	return &RateLimiter{
		redisClient: redisClient,
	}
}

// RateLimitConfig defines the rate limit configuration
type RateLimitConfig struct {
	MaxAttempts int           // Maximum number of attempts allowed
	Window      time.Duration // Time window for the rate limit
	KeyPrefix   string        // Redis key prefix for this rate limit
}

// Default rate limit configurations
var (
	RefreshTokenRateLimit = RateLimitConfig{
		MaxAttempts: 10,        // 10 attempts
		Window:      time.Hour, // per hour
		KeyPrefix:   "rate:refresh:",
	}

	AuthRateLimit = RateLimitConfig{
		MaxAttempts: 5,                // 5 attempts
		Window:      15 * time.Minute, // per 15 minutes
		KeyPrefix:   "rate:auth:",
	}

	AdminAuthRateLimit = RateLimitConfig{
		MaxAttempts: 3,                // 3 attempts
		Window:      15 * time.Minute, // per 15 minutes
		KeyPrefix:   "rate:admin:",
	}
)

// CheckRateLimit checks if the rate limit has been exceeded
func (r *RateLimiter) CheckRateLimit(ctx context.Context, key string, config RateLimitConfig) (bool, error) {
	redisKey := fmt.Sprintf("%s%s", config.KeyPrefix, key)

	// Get current count
	count, err := r.redisClient.Get(ctx, redisKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("failed to get rate limit count: %w", err)
	}

	// If count doesn't exist, set it
	if errors.Is(err, redis.Nil) {
		err = r.redisClient.Set(ctx, redisKey, 1, config.Window).Err()
		if err != nil {
			return false, fmt.Errorf("failed to set rate limit count: %w", err)
		}
		return true, nil
	}

	// Check if limit exceeded
	if count >= config.MaxAttempts {
		return false, nil
	}

	// Increment count by setting a new value
	err = r.redisClient.Set(ctx, redisKey, count+1, config.Window).Err()
	if err != nil {
		return false, fmt.Errorf("failed to increment rate limit count: %w", err)
	}

	return true, nil
}

// ResetRateLimit resets the rate limit for a key
func (r *RateLimiter) ResetRateLimit(ctx context.Context, key string, config RateLimitConfig) error {
	redisKey := fmt.Sprintf("%s%s", config.KeyPrefix, key)
	return r.redisClient.Del(ctx, redisKey).Err()
}

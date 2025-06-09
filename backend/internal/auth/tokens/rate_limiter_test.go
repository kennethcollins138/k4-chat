package tokens

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Verify rate limiter was created correctly
	assert.NotNil(t, limiter, "RateLimiter should not be nil")
}

func TestCheckRateLimit_FirstRequest(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 3,
		Window:      time.Minute,
		KeyPrefix:   "test:",
	}

	// Check rate limit for first request
	allowed, err := limiter.CheckRateLimit(ctx, "user1", config)

	// Verify results
	assert.NoError(t, err, "CheckRateLimit should not return an error")
	assert.True(t, allowed, "First request should be allowed")

	// Verify the key was created in Redis with correct value and TTL
	key := "test:user1"
	val, err := client.Get(ctx, key).Int()
	assert.NoError(t, err, "Key should exist in Redis")
	assert.Equal(t, 1, val, "Count should be 1")

	// Check TTL is approximately the window duration
	ttl, err := client.TTL(ctx, key).Result()
	assert.NoError(t, err, "TTL should be retrievable")
	assert.True(t, ttl > 0, "TTL should be set")
	assert.True(t, ttl <= config.Window, "TTL should not exceed window")
}

func TestCheckRateLimit_MultipleRequests(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 3,
		Window:      time.Minute,
		KeyPrefix:   "test:",
	}

	// Make multiple requests
	for i := 1; i <= 5; i++ {
		allowed, err := limiter.CheckRateLimit(ctx, "user2", config)

		// For first 3 requests (i <= 3), should be allowed
		if i <= config.MaxAttempts {
			assert.NoError(t, err, "Request %d should not error", i)
			assert.True(t, allowed, "Request %d should be allowed", i)
		} else {
			// For requests > 3, should be rate limited
			assert.NoError(t, err, "Request %d should not error", i)
			assert.False(t, allowed, "Request %d should be rate limited", i)
		}

		// Verify the key value in Redis
		key := "test:user2"
		val, err := client.Get(ctx, key).Int()
		assert.NoError(t, err, "Key should exist in Redis")

		// Value should be min(i, MaxAttempts+1)
		// We only increment up to MaxAttempts, then stop incrementing
		expectedVal := i
		if i > config.MaxAttempts {
			// After exceeding rate limit, the count doesn't increase anymore
			expectedVal = config.MaxAttempts
		}
		assert.Equal(t, expectedVal, val, "Count should match expected value for request %d", i)
	}
}

func TestCheckRateLimit_DifferentKeys(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 2,
		Window:      time.Minute,
		KeyPrefix:   "test:",
	}

	// Rate limit for user3 - first request
	allowed, err := limiter.CheckRateLimit(ctx, "user3", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Rate limit for user4 - should be independent
	allowed, err = limiter.CheckRateLimit(ctx, "user4", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Second request for user3
	allowed, err = limiter.CheckRateLimit(ctx, "user3", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Third request for user3 - should be rate limited
	allowed, err = limiter.CheckRateLimit(ctx, "user3", config)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Second request for user4 - still allowed
	allowed, err = limiter.CheckRateLimit(ctx, "user4", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Verify counts are independent
	val, err := client.Get(ctx, "test:user3").Int()
	assert.NoError(t, err)
	assert.Equal(t, 2, val)

	val, err = client.Get(ctx, "test:user4").Int()
	assert.NoError(t, err)
	assert.Equal(t, 2, val)
}

func TestResetRateLimit(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 3,
		Window:      time.Minute,
		KeyPrefix:   "test:",
	}

	// Set up a rate limit count
	key := "test:user5"
	err := client.Set(ctx, key, 3, config.Window).Err()
	require.NoError(t, err, "Failed to set up test key")

	// Verify key exists before reset
	exists, err := client.Exists(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), exists, "Key should exist before reset")

	// Reset rate limit
	err = limiter.ResetRateLimit(ctx, "user5", config)
	assert.NoError(t, err, "ResetRateLimit should not return an error")

	// Verify key no longer exists
	exists, err = client.Exists(ctx, key).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), exists, "Key should not exist after reset")

	// Check rate limit again - should be allowed after reset
	allowed, err := limiter.CheckRateLimit(ctx, "user5", config)
	assert.NoError(t, err)
	assert.True(t, allowed, "Should be allowed after reset")
}

func TestRateLimit_RedisFailures(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 3,
		Window:      time.Minute,
		KeyPrefix:   "test:",
	}

	// Set up a test for Redis failures
	t.Run("Redis server failure during check", func(t *testing.T) {
		// Stop miniredis to simulate server failure
		s.Close()

		// Attempt to check rate limit - should return error
		allowed, err := limiter.CheckRateLimit(ctx, "user6", config)
		assert.Error(t, err, "Should error when Redis is unavailable")
		assert.False(t, allowed, "Should not be allowed when Redis fails")

		// Restart miniredis server
		s, err = miniredis.Run()
		require.NoError(t, err, "Failed to restart miniredis")

		// Reconnect the client
		client = redis.NewClient(&redis.Options{
			Addr: s.Addr(),
		})
		limiter = NewRateLimiter(*client)
	})

	t.Run("Redis server failure during reset", func(t *testing.T) {
		// Set up a rate limit key first
		key := "test:user7"
		err := client.Set(ctx, key, 2, config.Window).Err()
		require.NoError(t, err, "Failed to set up test key")

		// Stop miniredis to simulate server failure
		s.Close()

		// Attempt to reset rate limit - should return error
		err = limiter.ResetRateLimit(ctx, "user7", config)
		assert.Error(t, err, "Should error when Redis is unavailable")
	})
}

func TestDefaultRateLimitConfigs(t *testing.T) {
	// Verify the default rate limit configurations
	assert.Equal(t, 10, RefreshTokenRateLimit.MaxAttempts)
	assert.Equal(t, time.Hour, RefreshTokenRateLimit.Window)
	assert.Equal(t, "rate:refresh:", RefreshTokenRateLimit.KeyPrefix)

	assert.Equal(t, 5, AuthRateLimit.MaxAttempts)
	assert.Equal(t, 15*time.Minute, AuthRateLimit.Window)
	assert.Equal(t, "rate:auth:", AuthRateLimit.KeyPrefix)

	assert.Equal(t, 3, AdminAuthRateLimit.MaxAttempts)
	assert.Equal(t, 15*time.Minute, AdminAuthRateLimit.Window)
	assert.Equal(t, "rate:admin:", AdminAuthRateLimit.KeyPrefix)
}

func TestCheckRateLimit_TTLConsistency(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config with a longer window for easier testing
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 5,
		Window:      10 * time.Minute,
		KeyPrefix:   "test:",
	}

	// Make first request
	allowed, err := limiter.CheckRateLimit(ctx, "user8", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Verify TTL
	key := "test:user8"
	ttl1, err := client.TTL(ctx, key).Result()
	assert.NoError(t, err)
	assert.True(t, ttl1 > 0 && ttl1 <= config.Window, "TTL should be positive and not exceed window")

	// Wait a short time
	time.Sleep(1 * time.Second)

	// Make second request
	allowed, err = limiter.CheckRateLimit(ctx, "user8", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Verify TTL was reset
	ttl2, err := client.TTL(ctx, key).Result()
	assert.NoError(t, err)
	assert.True(t, ttl2 > 0 && ttl2 <= config.Window, "TTL should be positive and not exceed window")

	// The TTL should be reset to the full window duration
	assert.True(t, ttl2 >= ttl1-2*time.Second, "TTL should be reset to the full window duration")
}

func TestRateLimitExpiration(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()
	defer client.Close()

	// Create rate limiter
	limiter := NewRateLimiter(*client)

	// Create context and config with a tiny window
	ctx := context.Background()
	config := RateLimitConfig{
		MaxAttempts: 1,
		Window:      10 * time.Millisecond, // Very short window for testing
		KeyPrefix:   "test:",
	}

	// Make request and get rate limited
	allowed, err := limiter.CheckRateLimit(ctx, "user9", config)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Second request should be rate limited
	allowed, err = limiter.CheckRateLimit(ctx, "user9", config)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// Wait for the window to expire
	time.Sleep(20 * time.Millisecond)

	// Fast-forward the time in miniredis
	s.FastForward(20 * time.Millisecond)

	// After expiration, request should be allowed again
	allowed, err = limiter.CheckRateLimit(ctx, "user9", config)
	assert.NoError(t, err)
	assert.True(t, allowed, "Should be allowed after rate limit window expires")
}

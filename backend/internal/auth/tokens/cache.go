package tokens

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TokenCache implements a token pre-validation cache
type TokenCache struct {
	client      *redis.Client
	logger      *zap.Logger
	ttl         time.Duration
	mu          sync.RWMutex
	localCache  map[string]cacheEntry
	maxSize     int
	cleanupTime time.Duration
}

type cacheEntry struct {
	token     string
	claims    *TokenClaims
	expiresAt time.Time
}

// NewTokenCache creates a new token cache
func NewTokenCache(client *redis.Client, logger *zap.Logger, ttl time.Duration) *TokenCache {
	cache := &TokenCache{
		client:      client,
		logger:      logger,
		ttl:         ttl,
		localCache:  make(map[string]cacheEntry),
		maxSize:     1000, // Maximum number of tokens in local cache
		cleanupTime: 5 * time.Minute,
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves token claims from cache
func (c *TokenCache) Get(ctx context.Context, token string) (*TokenClaims, bool) {
	// Try local cache first
	c.mu.RLock()
	if entry, ok := c.localCache[token]; ok {
		if time.Now().Before(entry.expiresAt) {
			c.mu.RUnlock()
			return entry.claims, true
		}
		// Entry expired, remove it
		c.mu.RUnlock()
		c.mu.Lock()
		delete(c.localCache, token)
		c.mu.Unlock()
	} else {
		c.mu.RUnlock()
	}

	// Try Redis cache
	key := "token_cache:" + token
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			c.logger.Error("Failed to get token from Redis cache",
				zap.Error(err),
				zap.String("token", token),
			)
		}
		return nil, false
	}

	var claims TokenClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		c.logger.Error("Failed to unmarshal token claims",
			zap.Error(err),
			zap.String("token", token),
		)
		return nil, false
	}

	// Add to local cache
	c.mu.Lock()
	if len(c.localCache) >= c.maxSize {
		// Remove oldest entry if cache is full
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.localCache {
			if oldestTime.IsZero() || v.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.expiresAt
			}
		}
		if oldestKey != "" {
			delete(c.localCache, oldestKey)
		}
	}

	c.localCache[token] = cacheEntry{
		token:     token,
		claims:    &claims,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return &claims, true
}

// Set stores token claims in cache
func (c *TokenCache) Set(ctx context.Context, token string, claims *TokenClaims) error {
	// Store in Redis
	key := "token_cache:" + token
	data, err := json.Marshal(claims)
	if err != nil {
		return err
	}

	err = c.client.Set(ctx, key, data, c.ttl).Err()
	if err != nil {
		return err
	}

	// Store in local cache
	c.mu.Lock()
	if len(c.localCache) >= c.maxSize {
		// Remove oldest entry if cache is full
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.localCache {
			if oldestTime.IsZero() || v.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.expiresAt
			}
		}
		if oldestKey != "" {
			delete(c.localCache, oldestKey)
		}
	}

	c.localCache[token] = cacheEntry{
		token:     token,
		claims:    claims,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return nil
}

// Invalidate removes a token from cache
func (c *TokenCache) Invalidate(ctx context.Context, token string) error {
	// Remove from Redis
	key := "token_cache:" + token
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return err
	}

	// Remove from local cache
	c.mu.Lock()
	delete(c.localCache, token)
	c.mu.Unlock()

	return nil
}

// cleanup periodically removes expired entries from local cache
func (c *TokenCache) cleanup() {
	ticker := time.NewTicker(c.cleanupTime)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for token, entry := range c.localCache {
			if now.After(entry.expiresAt) {
				delete(c.localCache, token)
			}
		}
		c.mu.Unlock()
	}
}

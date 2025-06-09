package tokens

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// TokenAnalytics represents analytics data for a token
type TokenAnalytics struct {
	UserID      string    `json:"user_id"`
	TokenType   string    `json:"token_type"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Action      string    `json:"action"`
	Timestamp   time.Time `json:"timestamp"`
	DeviceID    string    `json:"device_id"`
	Fingerprint string    `json:"fingerprint"`
	Success     bool      `json:"success"`
	Error       string    `json:"error,omitempty"`
}

// AnalyticsStore handles the storage and retrieval of token analytics
type AnalyticsStore struct {
	redisClient redis.Client
}

// NewAnalyticsStore creates a new AnalyticsStore
func NewAnalyticsStore(redisClient redis.Client) *AnalyticsStore {
	return &AnalyticsStore{
		redisClient: redisClient,
	}
}

// RecordTokenEvent records a token-related event
func (s *AnalyticsStore) RecordTokenEvent(ctx context.Context, analytics *TokenAnalytics) error {
	// Marshal the analytics data
	jsonData, err := json.Marshal(analytics)
	if err != nil {
		return fmt.Errorf("failed to marshal analytics data: %w", err)
	}

	// Create keys for different analytics views
	userKey := fmt.Sprintf("analytics:user:%s", analytics.UserID)
	deviceKey := fmt.Sprintf("analytics:device:%s", analytics.DeviceID)
	actionKey := fmt.Sprintf("analytics:action:%s", analytics.Action)

	// Store the event in Redis with a 30-day TTL
	ttl := 30 * 24 * time.Hour

	// Store in user's history
	err = s.redisClient.SAdd(ctx, userKey, jsonData).Err()
	if err != nil {
		return fmt.Errorf("failed to store user analytics: %w", err)
	}
	err = s.redisClient.Expire(ctx, userKey, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration on user analytics: %w", err)
	}

	// Store in device history
	err = s.redisClient.SAdd(ctx, deviceKey, jsonData).Err()
	if err != nil {
		return fmt.Errorf("failed to store device analytics: %w", err)
	}
	err = s.redisClient.Expire(ctx, deviceKey, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration on device analytics: %w", err)
	}

	// Store in action history
	err = s.redisClient.SAdd(ctx, actionKey, jsonData).Err()
	if err != nil {
		return fmt.Errorf("failed to store action analytics: %w", err)
	}
	err = s.redisClient.Expire(ctx, actionKey, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set expiration on action analytics: %w", err)
	}

	return nil
}

// GetUserAnalytics retrieves analytics for a user
func (s *AnalyticsStore) GetUserAnalytics(ctx context.Context, userID string, startTime, endTime time.Time) ([]*TokenAnalytics, error) {
	key := fmt.Sprintf("analytics:user:%s", userID)
	return s.getAnalyticsInTimeRange(ctx, key, startTime, endTime)
}

// GetDeviceAnalytics retrieves analytics for a device
func (s *AnalyticsStore) GetDeviceAnalytics(ctx context.Context, deviceID string, startTime, endTime time.Time) ([]*TokenAnalytics, error) {
	key := fmt.Sprintf("analytics:device:%s", deviceID)
	return s.getAnalyticsInTimeRange(ctx, key, startTime, endTime)
}

// GetActionAnalytics retrieves analytics for an action
func (s *AnalyticsStore) GetActionAnalytics(ctx context.Context, action string, startTime, endTime time.Time) ([]*TokenAnalytics, error) {
	key := fmt.Sprintf("analytics:action:%s", action)
	return s.getAnalyticsInTimeRange(ctx, key, startTime, endTime)
}

// getAnalyticsInTimeRange retrieves analytics within a time range
func (s *AnalyticsStore) getAnalyticsInTimeRange(ctx context.Context, key string, startTime, endTime time.Time) ([]*TokenAnalytics, error) {
	var analytics []*TokenAnalytics

	// Get all members from the set
	members, err := s.redisClient.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get analytics data: %w", err)
	}

	// Process each member
	for _, member := range members {
		var event TokenAnalytics
		if err := json.Unmarshal([]byte(member), &event); err != nil {
			continue // Skip failed unmarshaling
		}

		// Check if event is in time range
		if event.Timestamp.After(startTime) && event.Timestamp.Before(endTime) {
			analytics = append(analytics, &event)
		}
	}

	return analytics, nil
}

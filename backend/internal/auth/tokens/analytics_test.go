package tokens

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiniredis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	// Create a new miniredis server
	s, err := miniredis.Run()
	require.NoError(t, err, "Failed to create miniredis server")

	// Create a redis client that connects to the miniredis server
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	return s, client
}

func createSampleAnalytics() *TokenAnalytics {
	return &TokenAnalytics{
		UserID:      "test-user-123",
		TokenType:   "access",
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
		Action:      "login",
		Timestamp:   time.Now(),
		DeviceID:    "device-123",
		Fingerprint: "fingerprint-abc",
		Success:     true,
	}
}

func TestNewAnalyticsStore(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)

	// Verify store was created successfully
	assert.NotNil(t, store, "AnalyticsStore should not be nil")

	// Instead of comparing entire client objects, verify client functionality
	err := store.redisClient.Ping(context.Background()).Err()
	assert.NoError(t, err, "Redis client should be operational")

	// Verify the client is connected to the correct server
	assert.Equal(t, s.Addr(), store.redisClient.Options().Addr, "Redis client should be connected to the miniredis server")
}

func TestRecordTokenEvent(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)

	// Create sample analytics event
	ctx := context.Background()
	analytics := createSampleAnalytics()

	// Record the event
	err := store.RecordTokenEvent(ctx, analytics)
	require.NoError(t, err, "RecordTokenEvent should not return an error")

	// Verify data was stored in Redis
	userKey := "analytics:user:" + analytics.UserID
	deviceKey := "analytics:device:" + analytics.DeviceID
	actionKey := "analytics:action:" + analytics.Action

	// Check user key
	members, err := client.SMembers(ctx, userKey).Result()
	require.NoError(t, err, "Failed to get members for user key")
	assert.Equal(t, 1, len(members), "Expected 1 member in user key")

	// Check device key
	members, err = client.SMembers(ctx, deviceKey).Result()
	require.NoError(t, err, "Failed to get members for device key")
	assert.Equal(t, 1, len(members), "Expected 1 member in device key")

	// Check action key
	members, err = client.SMembers(ctx, actionKey).Result()
	require.NoError(t, err, "Failed to get members for action key")
	assert.Equal(t, 1, len(members), "Expected 1 member in action key")

	// Verify TTL was set
	userTTL, err := client.TTL(ctx, userKey).Result()
	require.NoError(t, err, "Failed to get TTL for user key")
	assert.True(t, userTTL > 0, "TTL should be set for user key")

	// Decode and verify stored data
	var storedAnalytics TokenAnalytics
	err = json.Unmarshal([]byte(members[0]), &storedAnalytics)
	require.NoError(t, err, "Failed to unmarshal stored analytics")

	assert.Equal(t, analytics.UserID, storedAnalytics.UserID)
	assert.Equal(t, analytics.Action, storedAnalytics.Action)
	assert.Equal(t, analytics.DeviceID, storedAnalytics.DeviceID)
	assert.Equal(t, analytics.Success, storedAnalytics.Success)
}

func TestGetUserAnalytics(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Create multiple analytics events with different timestamps
	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)
	futureTime := now.Add(2 * time.Hour)

	analytics1 := createSampleAnalytics()
	analytics1.Timestamp = now.Add(-1 * time.Hour) // 1 hour in the past

	analytics2 := createSampleAnalytics()
	analytics2.Timestamp = now

	analytics3 := createSampleAnalytics()
	analytics3.Timestamp = now.Add(1 * time.Hour) // 1 hour in the future

	// Record all events
	require.NoError(t, store.RecordTokenEvent(ctx, analytics1))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics2))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics3))

	// Test getting analytics within time range
	results, err := store.GetUserAnalytics(ctx, analytics1.UserID, pastTime, futureTime)
	require.NoError(t, err, "GetUserAnalytics should not return an error")
	assert.Equal(t, 3, len(results), "Expected 3 analytics events within time range")

	// Test getting analytics with narrower time range
	results, err = store.GetUserAnalytics(ctx, analytics1.UserID, pastTime, now)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results), "Expected 1 analytics event within narrower time range")
	assert.Equal(t, analytics1.Timestamp.Unix(), results[0].Timestamp.Unix(), "Timestamp should match")
}

func TestGetDeviceAnalytics(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Create analytics events for different devices
	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)
	futureTime := now.Add(2 * time.Hour)

	analytics1 := createSampleAnalytics()
	analytics1.DeviceID = "device-1"
	analytics1.Timestamp = now.Add(-1 * time.Hour)

	analytics2 := createSampleAnalytics()
	analytics2.DeviceID = "device-2"
	analytics2.Timestamp = now

	analytics3 := createSampleAnalytics()
	analytics3.DeviceID = "device-1" // Same as analytics1
	analytics3.Timestamp = now.Add(1 * time.Hour)

	// Record all events
	require.NoError(t, store.RecordTokenEvent(ctx, analytics1))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics2))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics3))

	// Test getting analytics for device-1
	results, err := store.GetDeviceAnalytics(ctx, "device-1", pastTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 2, len(results), "Expected 2 analytics events for device-1")

	// Test getting analytics for device-2
	results, err = store.GetDeviceAnalytics(ctx, "device-2", pastTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results), "Expected 1 analytics event for device-2")
}

func TestGetActionAnalytics(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Create analytics events for different actions
	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)
	futureTime := now.Add(2 * time.Hour)

	analytics1 := createSampleAnalytics()
	analytics1.Action = "login"
	analytics1.Timestamp = now.Add(-1 * time.Hour)

	analytics2 := createSampleAnalytics()
	analytics2.Action = "logout"
	analytics2.Timestamp = now

	analytics3 := createSampleAnalytics()
	analytics3.Action = "login" // Same as analytics1
	analytics3.Timestamp = now.Add(1 * time.Hour)

	// Record all events
	require.NoError(t, store.RecordTokenEvent(ctx, analytics1))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics2))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics3))

	// Test getting analytics for login action
	results, err := store.GetActionAnalytics(ctx, "login", pastTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 2, len(results), "Expected 2 analytics events for login action")

	// Test getting analytics for logout action
	results, err = store.GetActionAnalytics(ctx, "logout", pastTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results), "Expected 1 analytics event for logout action")
}

func TestGetAnalyticsInTimeRange(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Create analytics events with different timestamps
	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)
	midTime := now
	futureTime := now.Add(2 * time.Hour)

	analytics1 := createSampleAnalytics()
	analytics1.Timestamp = now.Add(-1 * time.Hour) // Before midTime

	analytics2 := createSampleAnalytics()
	analytics2.Timestamp = now.Add(1 * time.Hour) // After midTime

	// Record events
	require.NoError(t, store.RecordTokenEvent(ctx, analytics1))
	require.NoError(t, store.RecordTokenEvent(ctx, analytics2))

	// Test time filtering - should get both events
	key := "analytics:user:" + analytics1.UserID
	results, err := store.getAnalyticsInTimeRange(ctx, key, pastTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 2, len(results), "Expected 2 events in full time range")

	// Test time filtering - should get only the first event
	results, err = store.getAnalyticsInTimeRange(ctx, key, pastTime, midTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results), "Expected 1 event before midTime")
	assert.True(t, results[0].Timestamp.Before(midTime), "Event should be before midTime")

	// Test time filtering - should get only the second event
	results, err = store.getAnalyticsInTimeRange(ctx, key, midTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results), "Expected 1 event after midTime")
	assert.True(t, results[0].Timestamp.After(midTime), "Event should be after midTime")
}

func TestHandleInvalidJSON(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Create a key with invalid JSON data
	userKey := "analytics:user:test-user"
	invalidJSON := "this is not valid json"

	err := client.SAdd(ctx, userKey, invalidJSON).Err()
	require.NoError(t, err, "Failed to add invalid JSON to set")

	// Test that invalid JSON is skipped during retrieval
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	results, err := store.getAnalyticsInTimeRange(ctx, userKey, pastTime, futureTime)
	require.NoError(t, err, "Function should not error on invalid JSON")
	assert.Equal(t, 0, len(results), "Should not include invalid JSON in results")

	// Add valid JSON and test mixed retrieval
	analytics := createSampleAnalytics()
	jsonData, err := json.Marshal(analytics)
	require.NoError(t, err)

	err = client.SAdd(ctx, userKey, jsonData).Err()
	require.NoError(t, err)

	results, err = store.getAnalyticsInTimeRange(ctx, userKey, pastTime, futureTime)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results), "Should include only valid JSON in results")
}

func TestMultipleStorageForSameEvent(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Create sample analytics
	analytics := createSampleAnalytics()

	// Record the event
	err := store.RecordTokenEvent(ctx, analytics)
	require.NoError(t, err)

	// Verify data was stored in all three sets
	userKey := "analytics:user:" + analytics.UserID
	deviceKey := "analytics:device:" + analytics.DeviceID
	actionKey := "analytics:action:" + analytics.Action

	// Get all members and confirm they're identical
	userMembers, err := client.SMembers(ctx, userKey).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, len(userMembers))

	deviceMembers, err := client.SMembers(ctx, deviceKey).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, len(deviceMembers))

	actionMembers, err := client.SMembers(ctx, actionKey).Result()
	require.NoError(t, err)
	assert.Equal(t, 1, len(actionMembers))

	// Ensure all members are identical JSON objects
	assert.Equal(t, userMembers[0], deviceMembers[0], "User and device entries should be identical")
	assert.Equal(t, userMembers[0], actionMembers[0], "User and action entries should be identical")
}

func TestErrorHandlingDuringRetrieval(t *testing.T) {
	// Set up miniredis
	s, client := setupMiniredis(t)
	defer s.Close()

	// Create analytics store
	store := NewAnalyticsStore(*client)
	ctx := context.Background()

	// Test fetching analytics from non-existent key
	now := time.Now()
	results, err := store.GetUserAnalytics(ctx, "non-existent-user", now.Add(-1*time.Hour), now.Add(1*time.Hour))
	assert.NoError(t, err, "Should not error for non-existent key")
	assert.Equal(t, 0, len(results), "Should return empty results for non-existent key")
}

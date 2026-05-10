package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kube-ops-view/internal/models"

	"github.com/redis/go-redis/v9"
)

// TestRedisStore_Integration tests Redis store with a real Redis instance
// This test requires a Redis server running on localhost:6379
func TestRedisStore_Integration(t *testing.T) {
	// Skip if Redis is not available
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	config := RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1, // Use DB 1 for testing
		TokenTTL: time.Hour,
	}

	store, err := NewRedisStore(config)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()

	// Clean up test data
	defer store.FlushAll()

	t.Run("ClusterOperations", func(t *testing.T) {
		testRedisClusterOperations(t, store)
	})

	t.Run("ClusterStatus", func(t *testing.T) {
		testRedisClusterStatus(t, store)
	})

	t.Run("Events", func(t *testing.T) {
		testRedisEvents(t, store)
	})

	t.Run("ScreenTokens", func(t *testing.T) {
		testRedisScreenTokens(t, store)
	})

	t.Run("ExpiredTokens", func(t *testing.T) {
		testRedisExpiredTokens(t, store)
	})
}

func testRedisClusterOperations(t *testing.T, store *RedisStore) {
	// Test empty store
	ids := store.GetClusterIDs()
	if len(ids) != 0 {
		t.Errorf("Expected empty cluster IDs, got %v", ids)
	}

	// Test cluster not found
	_, err := store.GetClusterData("nonexistent")
	if err != ErrClusterNotFound {
		t.Errorf("Expected ErrClusterNotFound, got %v", err)
	}

	// Create test cluster data
	clusterData := &models.ClusterData{
		ID:             "test-cluster",
		APIServerURL:   "https://test.example.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Add a test node
	clusterData.Nodes["node1"] = &models.Node{
		Name:   "node1",
		Labels: map[string]string{"role": "worker"},
		Pods:   make(map[string]*models.Pod),
	}

	// Test setting cluster data
	err = store.SetClusterData("test-cluster", clusterData)
	if err != nil {
		t.Errorf("Failed to set cluster data: %v", err)
	}

	// Test getting cluster data
	retrieved, err := store.GetClusterData("test-cluster")
	if err != nil {
		t.Errorf("Failed to get cluster data: %v", err)
	}

	if retrieved.ID != clusterData.ID {
		t.Errorf("Expected cluster ID %s, got %s", clusterData.ID, retrieved.ID)
	}

	if retrieved.APIServerURL != clusterData.APIServerURL {
		t.Errorf("Expected API server URL %s, got %s", clusterData.APIServerURL, retrieved.APIServerURL)
	}

	// Test cluster IDs
	ids = store.GetClusterIDs()
	if len(ids) != 1 || ids[0] != "test-cluster" {
		t.Errorf("Expected cluster IDs [test-cluster], got %v", ids)
	}

	// Test deleting cluster
	err = store.DeleteCluster("test-cluster")
	if err != nil {
		t.Errorf("Failed to delete cluster: %v", err)
	}

	// Verify deletion
	ids = store.GetClusterIDs()
	if len(ids) != 0 {
		t.Errorf("Expected empty cluster IDs after deletion, got %v", ids)
	}
}

func testRedisClusterStatus(t *testing.T, store *RedisStore) {
	// Test status not found
	_, err := store.GetClusterStatus("nonexistent")
	if err != ErrClusterNotFound {
		t.Errorf("Expected ErrClusterNotFound, got %v", err)
	}

	// Create test status
	status := &ClusterStatus{
		ID:           "test-cluster",
		Available:    true,
		LastSeen:     time.Now(),
		APIServerURL: "https://test.example.com",
	}

	// Test setting status
	err = store.SetClusterStatus("test-cluster", status)
	if err != nil {
		t.Errorf("Failed to set cluster status: %v", err)
	}

	// Test getting status
	retrieved, err := store.GetClusterStatus("test-cluster")
	if err != nil {
		t.Errorf("Failed to get cluster status: %v", err)
	}

	if retrieved.ID != status.ID {
		t.Errorf("Expected status ID %s, got %s", status.ID, retrieved.ID)
	}

	if retrieved.Available != status.Available {
		t.Errorf("Expected available %v, got %v", status.Available, retrieved.Available)
	}
}

func testRedisEvents(t *testing.T, store *RedisStore) {
	// Subscribe to events
	ch1, err := store.Subscribe()
	if err != nil {
		t.Errorf("Failed to subscribe: %v", err)
	}

	ch2, err := store.Subscribe()
	if err != nil {
		t.Errorf("Failed to subscribe: %v", err)
	}

	// Give some time for subscriptions to be established
	time.Sleep(100 * time.Millisecond)

	// Publish an event
	testData := map[string]string{"test": "data"}
	err = store.PublishEvent(EventTypeClusterUpdate, testData)
	if err != nil {
		t.Errorf("Failed to publish event: %v", err)
	}

	// Verify both subscribers receive the event
	select {
	case event := <-ch1:
		if event.Type != EventTypeClusterUpdate {
			t.Errorf("Expected event type %s, got %s", EventTypeClusterUpdate, event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for event on channel 1")
	}

	select {
	case event := <-ch2:
		if event.Type != EventTypeClusterUpdate {
			t.Errorf("Expected event type %s, got %s", EventTypeClusterUpdate, event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for event on channel 2")
	}

	// Test unsubscribe
	err = store.Unsubscribe(ch1)
	if err != nil {
		t.Errorf("Failed to unsubscribe: %v", err)
	}

	// Publish another event
	err = store.PublishEvent(EventTypeClusterStatus, testData)
	if err != nil {
		t.Errorf("Failed to publish event: %v", err)
	}

	// Only ch2 should receive the event
	select {
	case event := <-ch2:
		if event.Type != EventTypeClusterStatus {
			t.Errorf("Expected event type %s, got %s", EventTypeClusterStatus, event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for event on channel 2")
	}

	// ch1 should be closed
	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("Expected channel 1 to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		// This is expected - channel should be closed
	}
}

func testRedisScreenTokens(t *testing.T, store *RedisStore) {
	// Test creating token
	token, err := store.CreateScreenToken()
	if err != nil {
		t.Errorf("Failed to create token: %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	// Test validating token
	if !store.ValidateScreenToken(token) {
		t.Error("Expected token to be valid")
	}

	// Test redeeming token
	err = store.RedeemScreenToken(token, "127.0.0.1")
	if err != nil {
		t.Errorf("Failed to redeem token: %v", err)
	}

	// Test redeeming already used token
	err = store.RedeemScreenToken(token, "127.0.0.1")
	if err != ErrTokenAlreadyUsed {
		t.Errorf("Expected ErrTokenAlreadyUsed, got %v", err)
	}

	// Test invalid token
	if store.ValidateScreenToken("invalid-token") {
		t.Error("Expected invalid token to be invalid")
	}

	// Test redeeming invalid token
	err = store.RedeemScreenToken("invalid-token", "127.0.0.1")
	if err != ErrTokenNotFound {
		t.Errorf("Expected ErrTokenNotFound, got %v", err)
	}

	// Test deleting token
	newToken, err := store.CreateScreenToken()
	if err != nil {
		t.Errorf("Failed to create token: %v", err)
	}

	err = store.DeleteScreenToken(newToken)
	if err != nil {
		t.Errorf("Failed to delete token: %v", err)
	}

	if store.ValidateScreenToken(newToken) {
		t.Error("Expected deleted token to be invalid")
	}
}

func testRedisExpiredTokens(t *testing.T, store *RedisStore) {
	// Create a store with short TTL for testing
	config := RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
		TokenTTL: 100 * time.Millisecond,
	}

	shortTTLStore, err := NewRedisStore(config)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer shortTTLStore.Close()

	// Create token
	token, err := shortTTLStore.CreateScreenToken()
	if err != nil {
		t.Errorf("Failed to create token: %v", err)
	}

	// Token should be valid initially
	if !shortTTLStore.ValidateScreenToken(token) {
		t.Error("Expected token to be valid")
	}

	// Wait for token to expire
	time.Sleep(200 * time.Millisecond)

	// Token should be invalid now
	if shortTTLStore.ValidateScreenToken(token) {
		t.Error("Expected expired token to be invalid")
	}

	// Redeeming expired token should fail
	err = shortTTLStore.RedeemScreenToken(token, "127.0.0.1")
	if err != ErrTokenExpired {
		t.Errorf("Expected ErrTokenExpired, got %v", err)
	}
}

func TestRedisStore_ConcurrentAccess(t *testing.T) {
	// Skip if Redis is not available
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	config := RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       2, // Use DB 2 for testing
		TokenTTL: time.Hour,
	}

	store, err := NewRedisStore(config)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()
	defer store.FlushAll()

	// Test concurrent cluster operations
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			clusterID := fmt.Sprintf("cluster-%d", id)
			clusterData := &models.ClusterData{
				ID:             clusterID,
				APIServerURL:   fmt.Sprintf("https://cluster-%d.example.com", id),
				Nodes:          make(map[string]*models.Node),
				UnassignedPods: make(map[string]*models.Pod),
				LastUpdate:     time.Now(),
			}

			// Set data
			if err := store.SetClusterData(clusterID, clusterData); err != nil {
				t.Errorf("Failed to set cluster data: %v", err)
			}

			// Get data
			if _, err := store.GetClusterData(clusterID); err != nil {
				t.Errorf("Failed to get cluster data: %v", err)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all clusters were created
	ids := store.GetClusterIDs()
	if len(ids) != 10 {
		t.Errorf("Expected 10 clusters, got %d", len(ids))
	}
}

func TestRedisStore_Ping(t *testing.T) {
	// Skip if Redis is not available
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	config := RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       3,
		TokenTTL: time.Hour,
	}

	store, err := NewRedisStore(config)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}

	// Test ping on open store
	if err := store.Ping(); err != nil {
		t.Errorf("Expected ping to succeed, got %v", err)
	}

	// Test ping on closed store
	store.Close()
	if err := store.Ping(); err != ErrConnectionFailed {
		t.Errorf("Expected ErrConnectionFailed, got %v", err)
	}
}

func TestRedisStore_NilInputs(t *testing.T) {
	// Skip if Redis is not available
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping integration test")
	}

	config := RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       4,
		TokenTTL: time.Hour,
	}

	store, err := NewRedisStore(config)
	if err != nil {
		t.Fatalf("Failed to create Redis store: %v", err)
	}
	defer store.Close()
	defer store.FlushAll()

	// Test nil cluster data
	err = store.SetClusterData("test", nil)
	if err == nil {
		t.Error("Expected error for nil cluster data")
	}

	// Test nil cluster status
	err = store.SetClusterStatus("test", nil)
	if err == nil {
		t.Error("Expected error for nil cluster status")
	}
}

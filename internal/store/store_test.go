package store

import (
	"testing"
	"time"
)

func TestNewStore_Memory(t *testing.T) {
	config := Config{
		Type:     "memory",
		TokenTTL: time.Hour,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Errorf("Failed to create memory store: %v", err)
	}
	defer store.Close()

	// Verify it's a memory store
	if _, ok := store.(*MemoryStore); !ok {
		t.Error("Expected MemoryStore instance")
	}

	// Test ping
	if err := store.Ping(); err != nil {
		t.Errorf("Expected ping to succeed: %v", err)
	}
}

func TestNewStore_DefaultMemory(t *testing.T) {
	config := Config{
		Type: "", // Empty type should default to memory
	}

	store, err := NewStore(config)
	if err != nil {
		t.Errorf("Failed to create default memory store: %v", err)
	}
	defer store.Close()

	// Verify it's a memory store
	if _, ok := store.(*MemoryStore); !ok {
		t.Error("Expected MemoryStore instance")
	}
}

func TestNewStore_Redis(t *testing.T) {
	config := Config{
		Type: "redis",
		Redis: RedisConfig{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		},
		TokenTTL: time.Hour,
	}

	// This test will fail if Redis is not available, which is expected
	store, err := NewStore(config)
	if err != nil {
		// Redis not available, skip test
		t.Skipf("Redis not available: %v", err)
	}
	defer store.Close()

	// Verify it's a Redis store
	if _, ok := store.(*RedisStore); !ok {
		t.Error("Expected RedisStore instance")
	}
}

func TestNewStore_RedisCluster(t *testing.T) {
	config := Config{
		Type: "redis-cluster",
		Redis: RedisConfig{
			Addr:     "localhost:7000,localhost:7001,localhost:7002",
			Password: "",
		},
		TokenTTL: time.Hour,
	}

	// This test will fail if Redis cluster is not available, which is expected
	store, err := NewStore(config)
	if err != nil {
		// Redis cluster not available, skip test
		t.Skipf("Redis cluster not available: %v", err)
	}
	defer store.Close()

	// Verify it's a Redis store
	if _, ok := store.(*RedisStore); !ok {
		t.Error("Expected RedisStore instance")
	}
}

func TestNewStore_InvalidType(t *testing.T) {
	config := Config{
		Type: "invalid",
	}

	_, err := NewStore(config)
	if err == nil {
		t.Error("Expected error for invalid store type")
	}
}

func TestNewStore_RedisNoAddr(t *testing.T) {
	config := Config{
		Type:  "redis",
		Redis: RedisConfig{
			// No address specified
		},
	}

	_, err := NewStore(config)
	if err == nil {
		t.Error("Expected error for Redis store without address")
	}
}

func TestNewStore_RedisClusterSingleAddr(t *testing.T) {
	config := Config{
		Type: "redis-cluster",
		Redis: RedisConfig{
			Addr: "localhost:6379", // Single address for cluster
		},
	}

	_, err := NewStore(config)
	if err == nil {
		t.Error("Expected error for Redis cluster with single address")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Type != "memory" {
		t.Errorf("Expected default type 'memory', got %s", config.Type)
	}

	if config.TokenTTL != 24*time.Hour {
		t.Errorf("Expected default token TTL 24h, got %v", config.TokenTTL)
	}

	if config.Redis.Addr != "localhost:6379" {
		t.Errorf("Expected default Redis addr 'localhost:6379', got %s", config.Redis.Addr)
	}

	if config.Redis.DB != 0 {
		t.Errorf("Expected default Redis DB 0, got %d", config.Redis.DB)
	}
}

func TestStoreInterface_Compliance(t *testing.T) {
	// Test that both implementations satisfy the Store interface
	var _ Store = (*MemoryStore)(nil)
	var _ Store = (*RedisStore)(nil)
}

func TestStoreOperations_Memory(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	testStoreOperations(t, store)
}

func testStoreOperations(t *testing.T, store Store) {
	// Test basic operations that should work on any store implementation

	// Test ping
	if err := store.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}

	// Test cluster IDs on empty store
	ids := store.GetClusterIDs()
	if len(ids) != 0 {
		t.Errorf("Expected empty cluster IDs, got %v", ids)
	}

	// Test getting non-existent cluster
	_, err := store.GetClusterData("nonexistent")
	if err != ErrClusterNotFound {
		t.Errorf("Expected ErrClusterNotFound, got %v", err)
	}

	// Test getting non-existent status
	_, err = store.GetClusterStatus("nonexistent")
	if err != ErrClusterNotFound {
		t.Errorf("Expected ErrClusterNotFound, got %v", err)
	}

	// Test creating screen token
	token, err := store.CreateScreenToken()
	if err != nil {
		t.Errorf("Failed to create screen token: %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}

	// Test validating token
	if !store.ValidateScreenToken(token) {
		t.Error("Expected token to be valid")
	}

	// Test invalid token
	if store.ValidateScreenToken("invalid") {
		t.Error("Expected invalid token to be invalid")
	}

	// Test event subscription
	ch, err := store.Subscribe()
	if err != nil {
		t.Errorf("Failed to subscribe: %v", err)
	}

	// Test publishing event
	err = store.PublishEvent(EventTypeClusterUpdate, map[string]string{"test": "data"})
	if err != nil {
		t.Errorf("Failed to publish event: %v", err)
	}

	// Test unsubscribe
	err = store.Unsubscribe(ch)
	if err != nil {
		t.Errorf("Failed to unsubscribe: %v", err)
	}
}

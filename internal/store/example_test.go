package store_test

import (
	"fmt"
	"log"
	"time"

	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

func ExampleNewStore_memory() {
	// Create a memory store with default configuration
	config := store.Config{
		Type:     "memory",
		TokenTTL: 24 * time.Hour,
	}

	s, err := store.NewStore(config)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Create some cluster data
	clusterData := &models.ClusterData{
		ID:             "example-cluster",
		APIServerURL:   "https://k8s.example.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Store the cluster data
	err = s.SetClusterData("example-cluster", clusterData)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve cluster IDs
	ids := s.GetClusterIDs()
	fmt.Printf("Cluster IDs: %v\n", ids)

	// Output: Cluster IDs: [example-cluster]
}

func ExampleMemoryStore_events() {
	// Create a memory store
	s := store.NewMemoryStore()
	defer s.Close()

	// Subscribe to events
	eventCh, err := s.Subscribe()
	if err != nil {
		log.Fatal(err)
	}

	// Publish an event in a goroutine
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.PublishEvent(store.EventTypeClusterUpdate, map[string]string{
			"cluster": "example",
			"status":  "updated",
		})
	}()

	// Wait for the event
	select {
	case event := <-eventCh:
		fmt.Printf("Received event: %s\n", event.Type)
	case <-time.After(time.Second):
		fmt.Println("Timeout waiting for event")
	}

	// Unsubscribe
	s.Unsubscribe(eventCh)

	// Output: Received event: cluster_update
}

func ExampleMemoryStore_screenTokens() {
	// Create a memory store with 1-hour token TTL
	s := store.NewMemoryStoreWithTTL(time.Hour)
	defer s.Close()

	// Create a screen token
	token, err := s.CreateScreenToken()
	if err != nil {
		log.Fatal(err)
	}

	// Validate the token
	if s.ValidateScreenToken(token) {
		fmt.Println("Token is valid")
	}

	// Redeem the token
	err = s.RedeemScreenToken(token, "192.168.1.100")
	if err != nil {
		log.Fatal(err)
	}

	// Try to redeem again (should fail)
	err = s.RedeemScreenToken(token, "192.168.1.100")
	if err == store.ErrTokenAlreadyUsed {
		fmt.Println("Token already used")
	}

	// Output:
	// Token is valid
	// Token already used
}

func ExampleRedisStore() {
	// Create a Redis store configuration
	config := store.RedisConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		TokenTTL: 24 * time.Hour,
	}

	// This example assumes Redis is running
	var s store.Store
	redisStore, err := store.NewRedisStore(config)
	if err != nil {
		// Redis not available, use memory store instead
		fmt.Println("Redis not available, using memory store")
		s = store.NewMemoryStore()
	} else {
		s = redisStore
	}
	defer s.Close()

	// Test connection
	if err := s.Ping(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Store is ready")
	// Output:
	// Redis not available, using memory store
	// Store is ready
}

func ExampleClusterStatus() {
	s := store.NewMemoryStore()
	defer s.Close()

	// Create cluster status
	status := &store.ClusterStatus{
		ID:           "prod-cluster",
		Available:    true,
		LastSeen:     time.Now(),
		APIServerURL: "https://prod-k8s.example.com",
	}

	// Store the status
	err := s.SetClusterStatus("prod-cluster", status)
	if err != nil {
		log.Fatal(err)
	}

	// Retrieve the status
	retrieved, err := s.GetClusterStatus("prod-cluster")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Cluster %s is available: %v\n", retrieved.ID, retrieved.Available)
	// Output: Cluster prod-cluster is available: true
}

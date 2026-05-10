package events_test

import (
	"fmt"
	"log"
	"time"

	"kube-ops-view/internal/events"
	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// ExamplePublisher demonstrates how to use the EventPublisher
func ExamplePublisher() {
	// Create a memory store
	storeConfig := store.DefaultConfig()
	memoryStore, err := store.NewStore(storeConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer memoryStore.Close()

	// Create event publisher
	publisher := events.NewPublisher(memoryStore)
	defer publisher.Close()

	// Subscribe to all cluster events
	eventChan, err := publisher.Subscribe(nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create sample cluster data
	clusterData := &models.ClusterData{
		ID:           "example-cluster",
		APIServerURL: "https://example-cluster.k8s.local",
		Nodes: map[string]*models.Node{
			"node-1": {
				Name:   "node-1",
				Labels: map[string]string{"role": "worker"},
				Status: models.NodeStatus{Ready: true},
				Pods:   make(map[string]*models.Pod),
			},
		},
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Publish cluster update
	err = publisher.PublishClusterUpdate("example-cluster", clusterData)
	if err != nil {
		log.Fatal(err)
	}

	// Receive the event
	select {
	case event := <-eventChan:
		fmt.Printf("Received event: %s for cluster %s\n", event.Type, event.ClusterID)
	case <-time.After(time.Second):
		fmt.Println("No event received")
	}

	// Output: Received event: cluster_update for cluster example-cluster
}

// ExampleSSEHandler demonstrates how to set up SSE handler
func ExampleSSEHandler() {
	// Create store and publisher
	storeConfig := store.DefaultConfig()
	memoryStore, err := store.NewStore(storeConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer memoryStore.Close()

	publisher := events.NewPublisher(memoryStore)
	defer publisher.Close()

	// Configure SSE handler
	config := events.SSEConfig{
		PingInterval:  30 * time.Second,
		ClientTimeout: 5 * time.Minute,
		MaxClients:    1000,
		BufferSize:    100,
		EnableCORS:    true,
	}

	handler := events.NewSSEHandler(publisher, config)
	defer handler.Close()

	// Get health check information
	health := handler.HealthCheck()
	fmt.Printf("SSE Handler - Connected clients: %v, Max clients: %v\n",
		health["connected_clients"], health["max_clients"])

	// Output: SSE Handler - Connected clients: 0, Max clients: 1000
}

// ExamplePublisher_filtering demonstrates cluster-based event filtering
func ExamplePublisher_filtering() {
	// Create store and publisher
	storeConfig := store.DefaultConfig()
	memoryStore, err := store.NewStore(storeConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer memoryStore.Close()

	publisher := events.NewPublisher(memoryStore)
	defer publisher.Close()

	// Subscribe to specific clusters only
	eventChan, err := publisher.Subscribe([]string{"cluster-1", "cluster-2"})
	if err != nil {
		log.Fatal(err)
	}

	// Publish events for different clusters
	clusters := []string{"cluster-1", "cluster-2", "cluster-3"}

	for _, clusterID := range clusters {
		clusterData := &models.ClusterData{
			ID:             clusterID,
			APIServerURL:   fmt.Sprintf("https://%s.k8s.local", clusterID),
			Nodes:          make(map[string]*models.Node),
			UnassignedPods: make(map[string]*models.Pod),
			LastUpdate:     time.Now(),
		}

		err = publisher.PublishClusterUpdate(clusterID, clusterData)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Count received events (should only get cluster-1 and cluster-2)
	receivedCount := 0
	timeout := time.After(100 * time.Millisecond)

	for {
		select {
		case event := <-eventChan:
			receivedCount++
			fmt.Printf("Received filtered event for cluster: %s\n", event.ClusterID)
		case <-timeout:
			fmt.Printf("Total filtered events received: %d out of %d published\n", receivedCount, len(clusters))
			return
		}
	}

	// Output:
	// Received filtered event for cluster: cluster-1
	// Received filtered event for cluster: cluster-2
	// Total filtered events received: 2 out of 3 published
}

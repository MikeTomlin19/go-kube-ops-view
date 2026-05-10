package events

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"

	"github.com/gin-gonic/gin"
)

// TestEventStreamingIntegration tests the complete event streaming pipeline
func TestEventStreamingIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create components
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	config := SSEConfig{
		PingInterval:  200 * time.Millisecond,
		ClientTimeout: 5 * time.Second,
		MaxClients:    10,
		BufferSize:    10,
	}

	handler := NewSSEHandler(publisher, config)
	defer handler.Close()

	// Create test server
	router := gin.New()
	router.GET("/events", handler.HandleSSE)
	server := httptest.NewServer(router)
	defer server.Close()

	// Create SSE client
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", server.URL+"/events", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	// Verify response headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type: text/event-stream, got: %s", resp.Header.Get("Content-Type"))
	}

	// Read events from stream
	scanner := bufio.NewScanner(resp.Body)
	events := make([]EventData, 0)
	eventChan := make(chan EventData, 10)

	// Start reading events in goroutine
	go func() {
		defer close(eventChan)
		eventName := ""
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				eventName = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if eventData, ok := parseTestSSEEvent(eventName, jsonData); ok {
					eventChan <- eventData
				}
			}
		}
	}()

	// Wait for connection event
	select {
	case event := <-eventChan:
		if event.Type != "connection" {
			t.Errorf("Expected connection event, got: %s", event.Type)
		}
		events = append(events, event)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection event")
	}

	// Publish cluster update
	clusterData := &models.ClusterData{
		ID:           "test-cluster",
		APIServerURL: "https://test-cluster.example.com",
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

	err = publisher.PublishClusterUpdate("test-cluster", clusterData)
	if err != nil {
		t.Fatalf("Failed to publish cluster update: %v", err)
	}

	// Wait for cluster update event
	select {
	case event := <-eventChan:
		if event.Type != store.EventTypeClusterUpdate {
			t.Errorf("Expected cluster update event, got: %s", event.Type)
		}
		if event.ClusterID != "test-cluster" {
			t.Errorf("Expected cluster ID 'test-cluster', got: %s", event.ClusterID)
		}
		events = append(events, event)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for cluster update event")
	}

	// Publish cluster status
	status := &store.ClusterStatus{
		ID:           "test-cluster",
		Available:    true,
		LastSeen:     time.Now(),
		APIServerURL: "https://test-cluster.example.com",
	}

	err = publisher.PublishClusterStatus("test-cluster", status)
	if err != nil {
		t.Fatalf("Failed to publish cluster status: %v", err)
	}

	// Wait for cluster status event
	select {
	case event := <-eventChan:
		if event.Type != store.EventTypeClusterStatus {
			t.Errorf("Expected cluster status event, got: %s", event.Type)
		}
		events = append(events, event)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for cluster status event")
	}

	// Wait for ping event
	select {
	case event := <-eventChan:
		if event.Type != "ping" {
			t.Errorf("Expected ping event, got: %s", event.Type)
		}
		events = append(events, event)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for ping event")
	}

	// Verify we received all expected events
	expectedEventTypes := []string{"connection", store.EventTypeClusterUpdate, store.EventTypeClusterStatus, "ping"}
	if len(events) < len(expectedEventTypes) {
		t.Errorf("Expected at least %d events, got %d", len(expectedEventTypes), len(events))
	}

	for i, expectedType := range expectedEventTypes {
		if i < len(events) && events[i].Type != expectedType {
			t.Errorf("Event %d: expected type %s, got %s", i, expectedType, events[i].Type)
		}
	}
}

// TestEventStreamingWithFiltering tests event streaming with cluster filtering
func TestEventStreamingWithFiltering(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	config := SSEConfig{
		PingInterval:  time.Hour, // Disable pings for this test
		ClientTimeout: 5 * time.Second,
		MaxClients:    10,
		BufferSize:    10,
	}

	handler := NewSSEHandler(publisher, config)
	defer handler.Close()

	router := gin.New()
	router.GET("/events", handler.HandleSSE)
	server := httptest.NewServer(router)
	defer server.Close()

	// Create SSE client with cluster filter
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", server.URL+"/events?clusters=cluster-1,cluster-3", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Accept", "text/event-stream")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE: %v", err)
	}
	defer resp.Body.Close()

	// Read events
	scanner := bufio.NewScanner(resp.Body)
	eventChan := make(chan EventData, 10)

	go func() {
		defer close(eventChan)
		eventName := ""
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "event: ") {
				eventName = strings.TrimPrefix(line, "event: ")
			} else if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if eventData, ok := parseTestSSEEvent(eventName, jsonData); ok {
					eventChan <- eventData
				}
			}
		}
	}()

	// Wait for connection event
	select {
	case <-eventChan:
		// Connection event received
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for connection event")
	}

	// Publish events for different clusters
	testClusters := []struct {
		clusterID     string
		shouldReceive bool
	}{
		{"cluster-1", true},
		{"cluster-2", false},
		{"cluster-3", true},
		{"cluster-4", false},
	}

	for _, tc := range testClusters {
		clusterData := &models.ClusterData{
			ID:             tc.clusterID,
			APIServerURL:   "https://" + tc.clusterID + ".example.com",
			Nodes:          make(map[string]*models.Node),
			UnassignedPods: make(map[string]*models.Pod),
			LastUpdate:     time.Now(),
		}

		err = publisher.PublishClusterUpdate(tc.clusterID, clusterData)
		if err != nil {
			t.Fatalf("Failed to publish cluster update for %s: %v", tc.clusterID, err)
		}

		if tc.shouldReceive {
			select {
			case event := <-eventChan:
				if event.ClusterID != tc.clusterID {
					t.Errorf("Expected cluster ID %s, got %s", tc.clusterID, event.ClusterID)
				}
			case <-time.After(time.Second):
				t.Errorf("Timeout waiting for event for cluster %s", tc.clusterID)
			}
		} else {
			// Should not receive event for filtered clusters
			select {
			case event := <-eventChan:
				t.Errorf("Should not have received event for cluster %s, got event for %s", tc.clusterID, event.ClusterID)
			case <-time.After(200 * time.Millisecond):
				// Expected timeout
			}
		}
	}
}

// TestMultipleSSEClients tests multiple concurrent SSE clients
func TestMultipleSSEClients(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	config := SSEConfig{
		PingInterval:  time.Hour, // Disable pings
		ClientTimeout: 5 * time.Second,
		MaxClients:    10,
		BufferSize:    10,
	}

	handler := NewSSEHandler(publisher, config)
	defer handler.Close()

	router := gin.New()
	router.GET("/events", handler.HandleSSE)
	server := httptest.NewServer(router)
	defer server.Close()

	// Create multiple clients
	numClients := 3
	clients := make([]*http.Response, numClients)
	eventChans := make([]chan EventData, numClients)

	for i := 0; i < numClients; i++ {
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequest("GET", server.URL+"/events", nil)
		if err != nil {
			t.Fatalf("Failed to create request %d: %v", i, err)
		}

		req.Header.Set("Accept", "text/event-stream")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to connect client %d to SSE: %v", i, err)
		}
		defer resp.Body.Close()

		clients[i] = resp
		eventChans[i] = make(chan EventData, 10)

		// Start reading events
		go func(idx int) {
			defer close(eventChans[idx])
			scanner := bufio.NewScanner(clients[idx].Body)
			eventName := ""
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "event: ") {
					eventName = strings.TrimPrefix(line, "event: ")
				} else if strings.HasPrefix(line, "data: ") {
					jsonData := strings.TrimPrefix(line, "data: ")
					if eventData, ok := parseTestSSEEvent(eventName, jsonData); ok {
						eventChans[idx] <- eventData
					}
				}
			}
		}(i)
	}

	// Wait for all clients to connect
	for i := 0; i < numClients; i++ {
		select {
		case <-eventChans[i]:
			// Connection event received
		case <-time.After(2 * time.Second):
			t.Fatalf("Timeout waiting for connection event from client %d", i)
		}
	}

	// Verify all clients are connected
	if count := handler.GetClientCount(); count != numClients {
		t.Errorf("Expected %d connected clients, got %d", numClients, count)
	}

	// Publish an event
	clusterData := &models.ClusterData{
		ID:             "broadcast-test",
		APIServerURL:   "https://broadcast-test.example.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	err := publisher.PublishClusterUpdate("broadcast-test", clusterData)
	if err != nil {
		t.Fatalf("Failed to publish cluster update: %v", err)
	}

	// Verify all clients receive the event
	for i := 0; i < numClients; i++ {
		select {
		case event := <-eventChans[i]:
			if event.Type != store.EventTypeClusterUpdate {
				t.Errorf("Client %d: expected cluster update event, got %s", i, event.Type)
			}
			if event.ClusterID != "broadcast-test" {
				t.Errorf("Client %d: expected cluster ID 'broadcast-test', got %s", i, event.ClusterID)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("Client %d: timeout waiting for cluster update event", i)
		}
	}
}

func parseTestSSEEvent(eventName, jsonData string) (EventData, bool) {
	eventType := testSSEEventType(eventName)
	if eventType == "bootstrap_end" {
		return EventData{}, false
	}

	event := EventData{Type: eventType}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
		return event, true
	}

	switch eventType {
	case store.EventTypeClusterUpdate:
		if id, ok := payload["id"].(string); ok {
			event.ClusterID = id
		}
	case store.EventTypeClusterStatus:
		if id, ok := payload["cluster_id"].(string); ok {
			event.ClusterID = id
		}
	}

	return event, true
}

func testSSEEventType(eventName string) string {
	switch eventName {
	case "clusterupdate":
		return store.EventTypeClusterUpdate
	case "clusterdelta":
		return store.EventTypeClusterDelta
	case "clusterstatus":
		return store.EventTypeClusterStatus
	case "clusterdelete":
		return store.EventTypeClusterDelete
	case "bootstrapend":
		return "bootstrap_end"
	default:
		return eventName
	}
}

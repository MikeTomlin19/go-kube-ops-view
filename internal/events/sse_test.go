package events

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSSEHandler_HandleSSE(t *testing.T) {
	// Set gin to test mode
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

	// Test SSE header setting
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/events", nil)

	handler.setSSEHeaders(c)

	// Verify response headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("Expected Content-Type: text/event-stream")
	}

	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Error("Expected Cache-Control: no-cache")
	}

	if w.Header().Get("Connection") != "keep-alive" {
		t.Error("Expected Connection: keep-alive")
	}

	// Clean up
	handler.Close()
}

func TestSSEHandler_ClusterFiltering(t *testing.T) {
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

	// Test parseClusterFilter method directly
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/events?clusters=cluster-1,cluster-2", nil)

	result := handler.parseClusterFilter(c)

	if len(result) != 2 {
		t.Errorf("Expected 2 cluster IDs, got %d", len(result))
	}

	expectedClusters := map[string]bool{"cluster-1": true, "cluster-2": true}
	for _, clusterID := range result {
		if !expectedClusters[clusterID] {
			t.Errorf("Unexpected cluster ID: %s", clusterID)
		}
	}

	handler.Close()
}

func TestSSEHandler_MaxClients(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	config := SSEConfig{
		PingInterval:  time.Hour,
		ClientTimeout: 5 * time.Second,
		MaxClients:    5, // Set a specific limit
		BufferSize:    10,
	}

	handler := NewSSEHandler(publisher, config)

	// Test the max clients logic by checking the config
	if handler.config.MaxClients != 5 {
		t.Errorf("Expected MaxClients to be 5, got %d", handler.config.MaxClients)
	}

	// Test that client count starts at 0
	if count := handler.GetClientCount(); count != 0 {
		t.Errorf("Expected 0 clients initially, got %d", count)
	}

	handler.Close()
}

func TestSSEHandler_BroadcastMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	config := SSEConfig{
		PingInterval:  time.Hour,
		ClientTimeout: 5 * time.Second,
		MaxClients:    10,
		BufferSize:    10,
	}

	handler := NewSSEHandler(publisher, config)
	defer handler.Close()

	// Test that broadcast doesn't panic with no clients
	handler.BroadcastMessage("announcement", map[string]string{
		"message": "System maintenance in 5 minutes",
	})

	// Verify no clients are connected
	if count := handler.GetClientCount(); count != 0 {
		t.Errorf("Expected 0 connected clients, got %d", count)
	}
}

func TestSSEHandler_HealthCheck(t *testing.T) {
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	config := SSEConfig{
		PingInterval:  30 * time.Second,
		ClientTimeout: 5 * time.Minute,
		MaxClients:    100,
		BufferSize:    50,
	}

	handler := NewSSEHandler(publisher, config)
	defer handler.Close()

	health := handler.HealthCheck()

	expectedFields := []string{"connected_clients", "max_clients", "ping_interval", "client_timeout"}
	for _, field := range expectedFields {
		if _, exists := health[field]; !exists {
			t.Errorf("Health check missing field: %s", field)
		}
	}

	if health["connected_clients"] != 0 {
		t.Errorf("Expected 0 connected clients, got %v", health["connected_clients"])
	}

	if health["max_clients"] != 100 {
		t.Errorf("Expected max_clients 100, got %v", health["max_clients"])
	}
}

func TestSSEHandler_parseClusterFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	handler := NewSSEHandler(publisher, SSEConfig{})

	testCases := []struct {
		query    string
		expected []string
	}{
		{"", nil},
		{"cluster-1", []string{"cluster-1"}},
		{"cluster-1,cluster-2", []string{"cluster-1", "cluster-2"}},
		{"cluster-1, cluster-2 , cluster-3", []string{"cluster-1", "cluster-2", "cluster-3"}},
		{"cluster-1,,cluster-2", []string{"cluster-1", "cluster-2"}},
	}

	for _, tc := range testCases {
		// Create mock gin context
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Properly encode the URL
		url := "/events"
		if tc.query != "" {
			url += "?clusters=" + strings.ReplaceAll(tc.query, " ", "%20")
		}
		c.Request = httptest.NewRequest("GET", url, nil)

		result := handler.parseClusterFilter(c)

		if len(result) != len(tc.expected) {
			t.Errorf("Query '%s': expected %d clusters, got %d", tc.query, len(tc.expected), len(result))
			continue
		}

		for i, expected := range tc.expected {
			if i >= len(result) || result[i] != expected {
				t.Errorf("Query '%s': expected cluster %d to be '%s', got '%s'", tc.query, i, expected, result[i])
			}
		}
	}
}

func TestSSEHandler_generateClientID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	handler := NewSSEHandler(publisher, SSEConfig{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/events", nil)

	id1 := handler.generateClientID(c)
	time.Sleep(time.Millisecond) // Ensure different timestamp
	id2 := handler.generateClientID(c)

	if id1 == id2 {
		t.Error("Generated client IDs should be unique")
	}

	if !strings.Contains(id1, "-") {
		t.Error("Client ID should contain timestamp separator")
	}
}

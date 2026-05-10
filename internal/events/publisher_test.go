package events

import (
	"strings"
	"testing"
	"time"

	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// MockStore implements store.Store interface for testing
type MockStore struct {
	events      []store.Event
	subscribers []chan store.Event
}

func NewMockStore() *MockStore {
	return &MockStore{
		events:      make([]store.Event, 0),
		subscribers: make([]chan store.Event, 0),
	}
}

func (m *MockStore) GetClusterIDs() []string                                              { return nil }
func (m *MockStore) GetClusterData(clusterID string) (*models.ClusterData, error)         { return nil, nil }
func (m *MockStore) SetClusterData(clusterID string, data *models.ClusterData) error      { return nil }
func (m *MockStore) GetClusterStatus(clusterID string) (*store.ClusterStatus, error)      { return nil, nil }
func (m *MockStore) SetClusterStatus(clusterID string, status *store.ClusterStatus) error { return nil }
func (m *MockStore) DeleteCluster(clusterID string) error                                 { return nil }
func (m *MockStore) CreateScreenToken() (string, error)                                   { return "", nil }
func (m *MockStore) RedeemScreenToken(token, remoteAddr string) error                     { return nil }
func (m *MockStore) ValidateScreenToken(token string) bool                                { return false }
func (m *MockStore) DeleteScreenToken(token string) error                                 { return nil }
func (m *MockStore) Close() error                                                         { return nil }
func (m *MockStore) Ping() error                                                          { return nil }

func (m *MockStore) PublishEvent(eventType string, data interface{}) error {
	event := store.Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Extract cluster ID from data if it's cluster data
	if clusterData, ok := data.(*models.ClusterData); ok {
		event.ClusterID = clusterData.ID
	} else if status, ok := data.(*store.ClusterStatus); ok {
		event.ClusterID = status.ID
	} else if deleteData, ok := data.(map[string]string); ok {
		if clusterID, exists := deleteData["cluster_id"]; exists {
			event.ClusterID = clusterID
		}
	}

	m.events = append(m.events, event)

	// Broadcast to subscribers
	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
	return nil
}

func (m *MockStore) Subscribe() (<-chan store.Event, error) {
	ch := make(chan store.Event, 10)
	m.subscribers = append(m.subscribers, ch)
	return ch, nil
}

func (m *MockStore) Unsubscribe(ch <-chan store.Event) error {
	// Find and remove channel
	for i, subscriber := range m.subscribers {
		if subscriber == ch {
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			close(subscriber)
			break
		}
	}
	return nil
}

func TestPublisher_PublishClusterUpdate(t *testing.T) {
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	clusterData := &models.ClusterData{
		ID:           "test-cluster",
		APIServerURL: "https://test-cluster.example.com",
		Nodes:        make(map[string]*models.Node),
		LastUpdate:   time.Now(),
	}

	err := publisher.PublishClusterUpdate("test-cluster", clusterData)
	if err != nil {
		t.Fatalf("Failed to publish cluster update: %v", err)
	}

	// Verify event was stored
	if len(mockStore.events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(mockStore.events))
	}

	event := mockStore.events[0]
	if event.Type != store.EventTypeClusterUpdate {
		t.Errorf("Expected event type %s, got %s", store.EventTypeClusterUpdate, event.Type)
	}

	if event.ClusterID != "test-cluster" {
		t.Errorf("Expected cluster ID 'test-cluster', got %s", event.ClusterID)
	}
}

func TestPublisher_Subscribe(t *testing.T) {
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	// Subscribe to all clusters
	events1, err := publisher.Subscribe(nil)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Subscribe to specific cluster
	events2, err := publisher.Subscribe([]string{"cluster-1"})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Verify subscriber count
	if count := publisher.GetSubscriberCount(); count != 2 {
		t.Errorf("Expected 2 subscribers, got %d", count)
	}

	// Publish event for cluster-1
	clusterData := &models.ClusterData{
		ID:           "cluster-1",
		APIServerURL: "https://cluster-1.example.com",
		Nodes:        make(map[string]*models.Node),
		LastUpdate:   time.Now(),
	}

	err = publisher.PublishClusterUpdate("cluster-1", clusterData)
	if err != nil {
		t.Fatalf("Failed to publish cluster update: %v", err)
	}

	// Both subscribers should receive the event
	select {
	case event := <-events1:
		if event.ClusterID != "cluster-1" {
			t.Errorf("Expected cluster ID 'cluster-1', got %s", event.ClusterID)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event on events1")
	}

	select {
	case event := <-events2:
		if event.ClusterID != "cluster-1" {
			t.Errorf("Expected cluster ID 'cluster-1', got %s", event.ClusterID)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event on events2")
	}

	// Publish event for cluster-2
	clusterData2 := &models.ClusterData{
		ID:           "cluster-2",
		APIServerURL: "https://cluster-2.example.com",
		Nodes:        make(map[string]*models.Node),
		LastUpdate:   time.Now(),
	}

	err = publisher.PublishClusterUpdate("cluster-2", clusterData2)
	if err != nil {
		t.Fatalf("Failed to publish cluster update: %v", err)
	}

	// Only events1 (subscribed to all) should receive the event
	select {
	case event := <-events1:
		if event.ClusterID != "cluster-2" {
			t.Errorf("Expected cluster ID 'cluster-2', got %s", event.ClusterID)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for event on events1")
	}

	// events2 should not receive the event (filtered out)
	select {
	case <-events2:
		t.Error("events2 should not have received event for cluster-2")
	case <-time.After(100 * time.Millisecond):
		// Expected timeout
	}
}

func TestPublisher_ClusterFiltering(t *testing.T) {
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	// Subscribe to multiple specific clusters
	events, err := publisher.Subscribe([]string{"cluster-1", "cluster-3"})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	testCases := []struct {
		clusterID     string
		shouldReceive bool
	}{
		{"cluster-1", true},
		{"cluster-2", false},
		{"cluster-3", true},
		{"cluster-4", false},
	}

	for _, tc := range testCases {
		clusterData := &models.ClusterData{
			ID:           tc.clusterID,
			APIServerURL: "https://" + tc.clusterID + ".example.com",
			Nodes:        make(map[string]*models.Node),
			LastUpdate:   time.Now(),
		}

		err = publisher.PublishClusterUpdate(tc.clusterID, clusterData)
		if err != nil {
			t.Fatalf("Failed to publish cluster update for %s: %v", tc.clusterID, err)
		}

		if tc.shouldReceive {
			select {
			case event := <-events:
				if event.ClusterID != tc.clusterID {
					t.Errorf("Expected cluster ID '%s', got %s", tc.clusterID, event.ClusterID)
				}
			case <-time.After(time.Second):
				t.Errorf("Timeout waiting for event for cluster %s", tc.clusterID)
			}
		} else {
			select {
			case <-events:
				t.Errorf("Should not have received event for cluster %s", tc.clusterID)
			case <-time.After(100 * time.Millisecond):
				// Expected timeout
			}
		}
	}
}

func TestPublisher_Unsubscribe(t *testing.T) {
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)
	defer publisher.Close()

	// Subscribe
	events, err := publisher.Subscribe(nil)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Verify subscriber count
	if count := publisher.GetSubscriberCount(); count != 1 {
		t.Errorf("Expected 1 subscriber, got %d", count)
	}

	// Unsubscribe
	err = publisher.Unsubscribe(events)
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Verify subscriber count
	if count := publisher.GetSubscriberCount(); count != 0 {
		t.Errorf("Expected 0 subscribers, got %d", count)
	}

	// Publish event - should not panic
	clusterData := &models.ClusterData{
		ID:           "test-cluster",
		APIServerURL: "https://test-cluster.example.com",
		Nodes:        make(map[string]*models.Node),
		LastUpdate:   time.Now(),
	}

	err = publisher.PublishClusterUpdate("test-cluster", clusterData)
	if err != nil {
		t.Fatalf("Failed to publish cluster update after unsubscribe: %v", err)
	}
}

func TestPublisher_Close(t *testing.T) {
	mockStore := NewMockStore()
	publisher := NewPublisher(mockStore)

	// Subscribe
	events, err := publisher.Subscribe(nil)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Close publisher
	err = publisher.Close()
	if err != nil {
		t.Fatalf("Failed to close publisher: %v", err)
	}

	// Verify channel is closed
	select {
	case _, ok := <-events:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for channel to close")
	}

	// Verify subscriber count is zero
	if count := publisher.GetSubscriberCount(); count != 0 {
		t.Errorf("Expected 0 subscribers after close, got %d", count)
	}

	// Publishing after close should return error
	clusterData := &models.ClusterData{
		ID:           "test-cluster",
		APIServerURL: "https://test-cluster.example.com",
		Nodes:        make(map[string]*models.Node),
		LastUpdate:   time.Now(),
	}

	err = publisher.PublishClusterUpdate("test-cluster", clusterData)
	if err == nil {
		t.Error("Expected error when publishing after close")
	}
}

func TestFormatSSEEvent(t *testing.T) {
	event := store.Event{
		Type:      store.EventTypeClusterUpdate,
		Data:      map[string]string{"test": "data"},
		Timestamp: time.Now(),
		ClusterID: "test-cluster",
	}

	sseData, err := FormatSSEEvent(event)
	if err != nil {
		t.Fatalf("Failed to format SSE event: %v", err)
	}

	sseString := string(sseData)
	if !strings.HasPrefix(sseString, "event: clusterupdate\n") {
		t.Error("SSE data should start with clusterupdate event name")
	}

	if !strings.Contains(sseString, "data: ") {
		t.Error("SSE data should contain a data field")
	}

	if !strings.HasSuffix(sseString, "\n\n") {
		t.Error("SSE data should end with double newline")
	}

	// Verify JSON content
	jsonStart := strings.Index(sseString, "{")
	jsonEnd := strings.LastIndex(sseString, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		t.Error("SSE data should contain JSON")
	}
}

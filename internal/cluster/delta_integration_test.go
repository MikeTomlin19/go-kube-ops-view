package cluster

import (
	"testing"
	"time"

	"kube-ops-view/internal/delta"
	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// Mock implementations for testing

type mockStore struct {
	clusters map[string]*models.ClusterData
	statuses map[string]store.ClusterStatus
}

func newMockStore() *mockStore {
	return &mockStore{
		clusters: make(map[string]*models.ClusterData),
		statuses: make(map[string]store.ClusterStatus),
	}
}

func (m *mockStore) SetClusterData(clusterID string, data *models.ClusterData) error {
	m.clusters[clusterID] = data
	return nil
}

func (m *mockStore) GetClusterData(clusterID string) (*models.ClusterData, error) {
	data, exists := m.clusters[clusterID]
	if !exists {
		return nil, nil
	}
	return data, nil
}

func (m *mockStore) SetClusterStatus(clusterID string, status *ClusterStatus) error {
	// Convert to store.ClusterStatus for internal storage
	storeStatus := store.ClusterStatus{
		ID:           status.ID,
		Available:    status.Available,
		LastSeen:     status.LastSeen,
		ErrorMessage: status.Error,
		APIServerURL: "", // Not available in cluster.ClusterStatus
	}
	m.statuses[clusterID] = storeStatus
	return nil
}

func (m *mockStore) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	storeStatus, exists := m.statuses[clusterID]
	if !exists {
		return nil, nil
	}
	// Convert back to cluster.ClusterStatus
	status := &ClusterStatus{
		ID:        storeStatus.ID,
		Available: storeStatus.Available,
		LastSeen:  storeStatus.LastSeen,
		Error:     storeStatus.ErrorMessage,
	}
	return status, nil
}

type mockEventPublisher struct {
	updates  []mockUpdate
	statuses []mockStatus
}

type mockUpdate struct {
	clusterID string
	data      *models.ClusterData
}

type mockStatus struct {
	clusterID string
	status    *ClusterStatus
}

func (m *mockEventPublisher) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	m.updates = append(m.updates, mockUpdate{
		clusterID: clusterID,
		data:      data,
	})
	return nil
}

func (m *mockEventPublisher) PublishClusterStatus(clusterID string, status *ClusterStatus) error {
	m.statuses = append(m.statuses, mockStatus{
		clusterID: clusterID,
		status:    status,
	})
	return nil
}

type mockDeltaEventPublisher struct {
	events []mockDeltaEvent
}

type mockDeltaEvent struct {
	eventType string
	data      interface{}
}

func (m *mockDeltaEventPublisher) PublishEvent(eventType string, data interface{}) error {
	m.events = append(m.events, mockDeltaEvent{
		eventType: eventType,
		data:      data,
	})
	return nil
}

func TestDeltaAwareEventPublisher_PublishClusterUpdate(t *testing.T) {
	basePublisher := &mockEventPublisher{}
	deltaEventPublisher := &mockDeltaEventPublisher{}
	// store := newMockStore()

	deltaManager := delta.NewDeltaManager(deltaEventPublisher, newMockStoreForDelta(), delta.DefaultDeltaOptions())
	publisher := NewDeltaAwareEventPublisher(basePublisher, deltaManager)

	clusterID := "test-cluster"

	// Create test cluster data
	clusterData := &models.ClusterData{
		ID:           clusterID,
		APIServerURL: "https://api.test-cluster.com",
		Nodes: map[string]*models.Node{
			"node1": {
				Name: "node1",
				Labels: map[string]string{
					"kubernetes.io/hostname": "node1",
				},
				Status: models.NodeStatus{
					Ready: true,
				},
				Pods: map[string]*models.Pod{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
						Phase:     "Running",
						Ready:     true,
					},
				},
			},
		},
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// First update should trigger full update event
	err := publisher.PublishClusterUpdate(clusterID, clusterData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have published one event through delta manager
	if len(deltaEventPublisher.events) != 1 {
		t.Fatalf("Expected 1 delta event, got %d", len(deltaEventPublisher.events))
	}

	// Second update with changes should trigger delta event
	updatedData := clusterData.Clone()
	updatedData.Nodes["node1"].Pods["pod2"] = &models.Pod{
		Name:      "pod2",
		Namespace: "default",
		Phase:     "Running",
		Ready:     true,
	}
	updatedData.LastUpdate = time.Now()

	err = publisher.PublishClusterUpdate(clusterID, updatedData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have published two events now
	if len(deltaEventPublisher.events) != 2 {
		t.Fatalf("Expected 2 delta events, got %d", len(deltaEventPublisher.events))
	}

	// Base publisher should not have been called (delta manager handles publishing)
	if len(basePublisher.updates) != 0 {
		t.Errorf("Expected 0 base publisher updates, got %d", len(basePublisher.updates))
	}
}

func TestDeltaAwareEventPublisher_PublishClusterStatus(t *testing.T) {
	basePublisher := &mockEventPublisher{}
	deltaEventPublisher := &mockDeltaEventPublisher{}
	// store := newMockStore()

	deltaManager := delta.NewDeltaManager(deltaEventPublisher, newMockStoreForDelta(), delta.DefaultDeltaOptions())
	publisher := NewDeltaAwareEventPublisher(basePublisher, deltaManager)

	clusterID := "test-cluster"
	status := &ClusterStatus{
		ID:        clusterID,
		Available: true,
		LastSeen:  time.Now(),
	}

	// Status updates should go directly to base publisher
	err := publisher.PublishClusterStatus(clusterID, status)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Should have called base publisher for status
	if len(basePublisher.statuses) != 1 {
		t.Fatalf("Expected 1 status update, got %d", len(basePublisher.statuses))
	}

	if basePublisher.statuses[0].clusterID != clusterID {
		t.Errorf("Expected cluster ID %s, got %s", clusterID, basePublisher.statuses[0].clusterID)
	}

	// Delta event publisher should not have been called for status
	if len(deltaEventPublisher.events) != 0 {
		t.Errorf("Expected 0 delta events for status, got %d", len(deltaEventPublisher.events))
	}
}

func TestDeltaAwareQueryEngine_ClearClusterDelta(t *testing.T) {
	// store := newMockStore()
	basePublisher := &mockEventPublisher{}
	deltaEventPublisher := &mockDeltaEventPublisher{}

	deltaManager := delta.NewDeltaManager(deltaEventPublisher, newMockStoreForDelta(), delta.DefaultDeltaOptions())

	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}

	queryEngine := NewQueryEngine(config, newMockStore(), basePublisher, &NoOpMetricsRecorder{})
	deltaAwareEngine := SetupDeltaIntegration(queryEngine, deltaManager)

	clusterID := "test-cluster"

	// Add some delta data
	clusterData := &models.ClusterData{
		ID:             clusterID,
		APIServerURL:   "https://api.test-cluster.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Process an update to create delta data
	err := deltaManager.ProcessClusterUpdate(clusterID, clusterData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify cluster is in delta manager
	storedClusters := deltaManager.GetStoredClusters()
	found := false
	for _, id := range storedClusters {
		if id == clusterID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected cluster %s to be stored in delta manager", clusterID)
	}

	// Clear delta data
	deltaAwareEngine.ClearClusterDelta(clusterID)

	// Verify cluster is removed from delta manager
	storedClusters = deltaManager.GetStoredClusters()
	for _, id := range storedClusters {
		if id == clusterID {
			t.Fatalf("Expected cluster %s to be removed from delta manager", clusterID)
		}
	}
}

func TestDeltaAwareQueryEngine_GetDeltaStats(t *testing.T) {
	// store := newMockStore()
	basePublisher := &mockEventPublisher{}
	deltaEventPublisher := &mockDeltaEventPublisher{}

	deltaManager := delta.NewDeltaManager(deltaEventPublisher, newMockStoreForDelta(), delta.DefaultDeltaOptions())

	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}

	queryEngine := NewQueryEngine(config, newMockStore(), basePublisher, &NoOpMetricsRecorder{})
	deltaAwareEngine := SetupDeltaIntegration(queryEngine, deltaManager)

	// Add some test data
	clusterData := &models.ClusterData{
		ID:             "test-cluster-1",
		APIServerURL:   "https://api.test-cluster-1.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	deltaManager.ProcessClusterUpdate("test-cluster-1", clusterData)

	// Get stats
	stats := deltaAwareEngine.GetDeltaStats()

	// Verify stats structure
	if storedClusters, ok := stats["stored_clusters"].([]string); ok {
		if len(storedClusters) != 1 {
			t.Errorf("Expected 1 stored cluster, got %d", len(storedClusters))
		}
		if storedClusters[0] != "test-cluster-1" {
			t.Errorf("Expected cluster test-cluster-1, got %s", storedClusters[0])
		}
	} else {
		t.Errorf("Expected stored_clusters to be []string")
	}

	if clusterCount, ok := stats["cluster_count"].(int); ok {
		if clusterCount != 1 {
			t.Errorf("Expected cluster count 1, got %d", clusterCount)
		}
	} else {
		t.Errorf("Expected cluster_count to be int")
	}

	if _, ok := stats["delta_options"].(delta.DeltaOptions); !ok {
		t.Errorf("Expected delta_options to be DeltaOptions")
	}
}

func TestDeltaAwareQueryEngine_SetDeltaOptions(t *testing.T) {
	// store := newMockStore()
	basePublisher := &mockEventPublisher{}
	deltaEventPublisher := &mockDeltaEventPublisher{}

	deltaManager := delta.NewDeltaManager(deltaEventPublisher, newMockStoreForDelta(), delta.DefaultDeltaOptions())

	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}

	queryEngine := NewQueryEngine(config, newMockStore(), basePublisher, &NoOpMetricsRecorder{})
	deltaAwareEngine := SetupDeltaIntegration(queryEngine, deltaManager)

	// Set new options
	newOptions := delta.DeltaOptions{
		ArrayAlign:     true,
		CompareLengths: true,
		Minimal:        false,
		MaxDepth:       10,
	}

	deltaAwareEngine.SetDeltaOptions(newOptions)

	// Verify options were updated
	currentOptions := deltaManager.GetOptions()
	if currentOptions.ArrayAlign != true {
		t.Errorf("Expected ArrayAlign to be true, got %v", currentOptions.ArrayAlign)
	}
	if currentOptions.CompareLengths != true {
		t.Errorf("Expected CompareLengths to be true, got %v", currentOptions.CompareLengths)
	}
	if currentOptions.Minimal != false {
		t.Errorf("Expected Minimal to be false, got %v", currentOptions.Minimal)
	}
	if currentOptions.MaxDepth != 10 {
		t.Errorf("Expected MaxDepth to be 10, got %v", currentOptions.MaxDepth)
	}
}

func TestSetupDeltaIntegration(t *testing.T) {
	// store := newMockStore()
	basePublisher := &mockEventPublisher{}
	deltaEventPublisher := &mockDeltaEventPublisher{}

	deltaManager := delta.NewDeltaManager(deltaEventPublisher, newMockStoreForDelta(), delta.DefaultDeltaOptions())

	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}

	queryEngine := NewQueryEngine(config, newMockStore(), basePublisher, &NoOpMetricsRecorder{})

	// Setup delta integration
	deltaAwareEngine := SetupDeltaIntegration(queryEngine, deltaManager)

	// Verify the engine was wrapped correctly
	if deltaAwareEngine.QueryEngine != queryEngine {
		t.Errorf("Expected wrapped query engine to be the same instance")
	}

	if deltaAwareEngine.deltaManager != deltaManager {
		t.Errorf("Expected delta manager to be set correctly")
	}

	// Verify publisher was wrapped
	// The publisher should now be a DeltaAwareEventPublisher
	if _, ok := queryEngine.GetPublisher().(*DeltaAwareEventPublisher); !ok {
		t.Errorf("Expected publisher to be wrapped with DeltaAwareEventPublisher")
	}
}
func (m *mockStore) GetClusterIDs() []string {
	ids := make([]string, 0, len(m.clusters))
	for id := range m.clusters {
		ids = append(ids, id)
	}
	return ids
}

func (m *mockStore) DeleteCluster(clusterID string) error {
	delete(m.clusters, clusterID)
	delete(m.statuses, clusterID)
	return nil
}

func (m *mockStore) PublishEvent(eventType string, data interface{}) error {
	return nil
}

func (m *mockStore) Subscribe() (<-chan store.Event, error) {
	return nil, nil
}

func (m *mockStore) Unsubscribe(ch <-chan store.Event) error {
	return nil
}

func (m *mockStore) CreateScreenToken() (string, error) {
	return "mock-token", nil
}

func (m *mockStore) RedeemScreenToken(token, remoteAddr string) error {
	return nil
}

func (m *mockStore) ValidateScreenToken(token string) bool {
	return token == "mock-token"
}

func (m *mockStore) DeleteScreenToken(token string) error {
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func (m *mockStore) Ping() error {
	return nil
}

// mockStoreForDelta implements store.Store interface for delta manager
type mockStoreForDelta struct {
	clusters map[string]*models.ClusterData
}

func newMockStoreForDelta() *mockStoreForDelta {
	return &mockStoreForDelta{
		clusters: make(map[string]*models.ClusterData),
	}
}

func (m *mockStoreForDelta) GetClusterIDs() []string {
	ids := make([]string, 0, len(m.clusters))
	for id := range m.clusters {
		ids = append(ids, id)
	}
	return ids
}

func (m *mockStoreForDelta) GetClusterData(clusterID string) (*models.ClusterData, error) {
	data, exists := m.clusters[clusterID]
	if !exists {
		return nil, nil
	}
	return data, nil
}

func (m *mockStoreForDelta) SetClusterData(clusterID string, data *models.ClusterData) error {
	m.clusters[clusterID] = data
	return nil
}

func (m *mockStoreForDelta) GetClusterStatus(clusterID string) (*store.ClusterStatus, error) {
	return nil, nil
}

func (m *mockStoreForDelta) SetClusterStatus(clusterID string, status *store.ClusterStatus) error {
	return nil
}

func (m *mockStoreForDelta) DeleteCluster(clusterID string) error {
	delete(m.clusters, clusterID)
	return nil
}

func (m *mockStoreForDelta) PublishEvent(eventType string, data interface{}) error {
	return nil
}

func (m *mockStoreForDelta) Subscribe() (<-chan store.Event, error) {
	return nil, nil
}

func (m *mockStoreForDelta) Unsubscribe(ch <-chan store.Event) error {
	return nil
}

func (m *mockStoreForDelta) CreateScreenToken() (string, error) {
	return "mock-token", nil
}

func (m *mockStoreForDelta) RedeemScreenToken(token, remoteAddr string) error {
	return nil
}

func (m *mockStoreForDelta) ValidateScreenToken(token string) bool {
	return token == "mock-token"
}

func (m *mockStoreForDelta) DeleteScreenToken(token string) error {
	return nil
}

func (m *mockStoreForDelta) Close() error {
	return nil
}

func (m *mockStoreForDelta) Ping() error {
	return nil
}

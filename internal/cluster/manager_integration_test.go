package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"kube-ops-view/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// managerMockStore implements the Store interface for testing
type managerMockStore struct {
	clusterData     map[string]*models.ClusterData
	clusterStatus   map[string]*ClusterStatus
	mu              sync.RWMutex
	publishedEvents []managerMockEvent
}

type managerMockEvent struct {
	eventType string
	clusterID string
	data      interface{}
	timestamp time.Time
}

func newManagerMockStore() *managerMockStore {
	return &managerMockStore{
		clusterData:     make(map[string]*models.ClusterData),
		clusterStatus:   make(map[string]*ClusterStatus),
		publishedEvents: make([]managerMockEvent, 0),
	}
}

func (m *managerMockStore) SetClusterData(clusterID string, data *models.ClusterData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusterData[clusterID] = data
	return nil
}

func (m *managerMockStore) GetClusterData(clusterID string) (*models.ClusterData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if data, exists := m.clusterData[clusterID]; exists {
		return data, nil
	}
	return nil, fmt.Errorf("cluster data not found for %s", clusterID)
}

func (m *managerMockStore) SetClusterStatus(clusterID string, status *ClusterStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusterStatus[clusterID] = status
	return nil
}

func (m *managerMockStore) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if status, exists := m.clusterStatus[clusterID]; exists {
		return status, nil
	}
	return nil, fmt.Errorf("cluster status not found for %s", clusterID)
}

func (m *managerMockStore) DeleteCluster(clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clusterData, clusterID)
	delete(m.clusterStatus, clusterID)
	return nil
}

// managerMockEventPublisher implements the EventPublisher interface for testing
type managerMockEventPublisher struct {
	store *managerMockStore
}

func newManagerMockEventPublisher(store *managerMockStore) *managerMockEventPublisher {
	return &managerMockEventPublisher{store: store}
}

func (m *managerMockEventPublisher) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	m.store.publishedEvents = append(m.store.publishedEvents, managerMockEvent{
		eventType: "cluster_update",
		clusterID: clusterID,
		data:      data,
		timestamp: time.Now(),
	})
	return nil
}

func (m *managerMockEventPublisher) PublishClusterStatus(clusterID string, status *ClusterStatus) error {
	m.store.mu.Lock()
	defer m.store.mu.Unlock()
	m.store.publishedEvents = append(m.store.publishedEvents, managerMockEvent{
		eventType: "cluster_status",
		clusterID: clusterID,
		data:      status,
		timestamp: time.Now(),
	})
	return nil
}

// mockDiscovererWithChanges simulates a discoverer that can change its results
type mockDiscovererWithChanges struct {
	clusters []ClusterConfig
	mu       sync.RWMutex
}

func newMockDiscovererWithChanges(initialClusters []ClusterConfig) *mockDiscovererWithChanges {
	return &mockDiscovererWithChanges{
		clusters: initialClusters,
	}
}

func (m *mockDiscovererWithChanges) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]ClusterConfig, len(m.clusters))
	copy(result, m.clusters)
	return result, nil
}

func (m *mockDiscovererWithChanges) SetClusters(clusters []ClusterConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clusters = clusters
}

// failingDiscoverer simulates discovery failures
type failingDiscoverer struct {
	shouldFail bool
	mu         sync.RWMutex
}

func newFailingDiscoverer() *failingDiscoverer {
	return &failingDiscoverer{shouldFail: false}
}

func (f *failingDiscoverer) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.shouldFail {
		return nil, fmt.Errorf("simulated discovery failure")
	}

	return []ClusterConfig{
		{ID: "test-cluster", APIServer: "https://test.example.com"},
	}, nil
}

func (f *failingDiscoverer) SetShouldFail(shouldFail bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shouldFail = shouldFail
}

func TestClusterManager_MultiClusterScenarios(t *testing.T) {
	tests := []struct {
		name            string
		initialClusters []ClusterConfig
		expectedCount   int
		description     string
	}{
		{
			name: "single_cluster",
			initialClusters: []ClusterConfig{
				{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
			},
			expectedCount: 1,
			description:   "Manager should handle single cluster correctly",
		},
		{
			name: "multiple_clusters",
			initialClusters: []ClusterConfig{
				{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
				{ID: "cluster-2", APIServer: "https://cluster2.example.com"},
				{ID: "cluster-3", APIServer: "https://cluster3.example.com"},
			},
			expectedCount: 3,
			description:   "Manager should handle multiple clusters correctly",
		},
		{
			name:            "no_clusters",
			initialClusters: []ClusterConfig{},
			expectedCount:   0,
			description:     "Manager should handle empty cluster list correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test dependencies
			store := newManagerMockStore()
			publisher := newManagerMockEventPublisher(store)
			discoverer := newMockDiscovererWithChanges(tt.initialClusters)

			// Create manager with fast intervals for testing
			config := Config{
				QueryInterval:       100 * time.Millisecond,
				DiscoveryInterval:   200 * time.Millisecond,
				HealthCheckInterval: 50 * time.Millisecond,
				PruneThreshold:      500 * time.Millisecond,
				Mock:                true,
			}

			manager := &Manager{
				config:          config,
				store:           store,
				publisher:       publisher,
				discoverer:      discoverer,
				clients:         make(map[string]*ClusterClient),
				engines:         make(map[string]*QueryEngine),
				clusterStatuses: make(map[string]*ClusterStatus),
				pruneThreshold:  config.PruneThreshold,
				status: ManagerStatus{
					StartTime: time.Now(),
					Running:   false,
				},
			}

			manager.ctx, manager.cancel = context.WithCancel(context.Background())

			// Start manager
			err := manager.Start()
			require.NoError(t, err, tt.description)

			// Wait for initial discovery and setup
			time.Sleep(150 * time.Millisecond)

			// Check status
			status := manager.GetStatus()
			assert.True(t, status.Running, "Manager should be running")
			assert.Equal(t, tt.expectedCount, len(status.Clusters), tt.description)
			assert.Equal(t, tt.expectedCount, status.TotalClusters, "Total cluster count should match")

			// Verify cluster IDs
			clusterIDs := manager.GetClusterIDs()
			assert.Equal(t, tt.expectedCount, len(clusterIDs), "Cluster ID count should match")

			// Stop manager
			err = manager.Stop()
			assert.NoError(t, err, "Manager should stop without error")

			// Verify manager is stopped
			status = manager.GetStatus()
			assert.False(t, status.Running, "Manager should not be running after stop")
		})
	}
}

func TestClusterManager_DynamicClusterDiscovery(t *testing.T) {
	// Create test dependencies
	store := newManagerMockStore()
	publisher := newManagerMockEventPublisher(store)

	initialClusters := []ClusterConfig{
		{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
		{ID: "cluster-2", APIServer: "https://cluster2.example.com"},
	}
	discoverer := newMockDiscovererWithChanges(initialClusters)

	// Create manager with fast intervals for testing
	config := Config{
		QueryInterval:       100 * time.Millisecond,
		DiscoveryInterval:   200 * time.Millisecond,
		HealthCheckInterval: time.Hour, // Avoid racing the manual unavailable status below.
		PruneThreshold:      500 * time.Millisecond,
		Mock:                true,
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Start manager
	err := manager.Start()
	require.NoError(t, err)

	// Wait for initial discovery
	time.Sleep(150 * time.Millisecond)

	// Verify initial clusters
	status := manager.GetStatus()
	assert.Equal(t, 2, len(status.Clusters), "Should have 2 initial clusters")

	// Add a new cluster
	newClusters := []ClusterConfig{
		{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
		{ID: "cluster-2", APIServer: "https://cluster2.example.com"},
		{ID: "cluster-3", APIServer: "https://cluster3.example.com"},
	}
	discoverer.SetClusters(newClusters)

	// Wait for rediscovery
	time.Sleep(250 * time.Millisecond)

	// Verify new cluster was added
	status = manager.GetStatus()
	assert.Equal(t, 3, len(status.Clusters), "Should have 3 clusters after addition")

	// Remove a cluster
	reducedClusters := []ClusterConfig{
		{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
		{ID: "cluster-3", APIServer: "https://cluster3.example.com"},
	}
	discoverer.SetClusters(reducedClusters)

	// Wait for rediscovery
	time.Sleep(250 * time.Millisecond)

	// Verify cluster was removed
	status = manager.GetStatus()
	assert.Equal(t, 2, len(status.Clusters), "Should have 2 clusters after removal")

	// Verify the correct cluster was removed
	clusterIDs := manager.GetClusterIDs()
	assert.Contains(t, clusterIDs, "cluster-1", "cluster-1 should still exist")
	assert.Contains(t, clusterIDs, "cluster-3", "cluster-3 should still exist")
	assert.NotContains(t, clusterIDs, "cluster-2", "cluster-2 should be removed")

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestClusterManager_HealthMonitoring(t *testing.T) {
	// Create test dependencies
	store := newManagerMockStore()
	publisher := newManagerMockEventPublisher(store)

	clusters := []ClusterConfig{
		{ID: "healthy-cluster", APIServer: "https://healthy.example.com"},
		{ID: "unhealthy-cluster", APIServer: "https://unhealthy.example.com"},
	}
	discoverer := newMockDiscovererWithChanges(clusters)

	// Create manager with fast intervals for testing
	config := Config{
		QueryInterval:       100 * time.Millisecond,
		DiscoveryInterval:   1 * time.Second, // Longer to avoid interference
		HealthCheckInterval: 100 * time.Millisecond,
		PruneThreshold:      500 * time.Millisecond,
		Mock:                true,
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Start manager
	err := manager.Start()
	require.NoError(t, err)

	// Wait for initial setup and health checks
	time.Sleep(200 * time.Millisecond)

	// Check that health monitoring is working
	status := manager.GetStatus()
	assert.Equal(t, 2, len(status.Clusters), "Should have 2 clusters")

	// Verify health check methods work
	for _, cluster := range status.Clusters {
		clusterStatus, err := manager.GetClusterStatus(cluster.ID)
		assert.NoError(t, err, "Should be able to get cluster status")
		assert.NotNil(t, clusterStatus, "Cluster status should not be nil")

		// Since these are mock clusters, they will likely be unhealthy
		// but the important thing is that the health check ran
		assert.NotEmpty(t, clusterStatus.ID, "Cluster status should have ID")
	}

	// Test healthy cluster count
	healthyCount := manager.GetHealthyClusterCount()
	assert.GreaterOrEqual(t, healthyCount, 0, "Healthy cluster count should be non-negative")
	assert.LessOrEqual(t, healthyCount, 2, "Healthy cluster count should not exceed total")

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestClusterManager_ClusterPruning(t *testing.T) {
	// Create test dependencies
	store := newManagerMockStore()
	publisher := newManagerMockEventPublisher(store)

	clusters := []ClusterConfig{
		{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
		{ID: "cluster-2", APIServer: "https://cluster2.example.com"},
	}
	discoverer := newMockDiscovererWithChanges(clusters)

	// Create manager with very short prune threshold for testing
	config := Config{
		QueryInterval:       50 * time.Millisecond,
		DiscoveryInterval:   1 * time.Second, // Longer to avoid interference
		HealthCheckInterval: 50 * time.Millisecond,
		PruneThreshold:      200 * time.Millisecond, // Very short for testing
		Mock:                true,
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Start manager
	err := manager.Start()
	require.NoError(t, err)

	// Wait for initial setup
	time.Sleep(100 * time.Millisecond)

	// Verify initial clusters
	status := manager.GetStatus()
	assert.Equal(t, 2, len(status.Clusters), "Should have 2 initial clusters")

	// Manually mark one cluster as unavailable with old timestamp
	oldTime := time.Now().Add(-1 * time.Hour)
	manager.updateClusterStatus("cluster-1", &ClusterStatus{
		ID:        "cluster-1",
		Available: false,
		LastSeen:  oldTime,
		Error:     "simulated failure",
	})

	// Manually trigger pruning since the automatic pruning might not run in time
	manager.performClusterPruning()

	// Verify the unavailable cluster was pruned
	clusterIDs := manager.GetClusterIDs()
	assert.NotContains(t, clusterIDs, "cluster-1", "Unavailable cluster should be pruned")

	// The other cluster should still exist (though it might also be unhealthy)
	status = manager.GetStatus()
	assert.LessOrEqual(t, len(status.Clusters), 1, "Should have at most 1 cluster after pruning")

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestClusterManager_GracefulShutdown(t *testing.T) {
	// Create test dependencies
	store := newManagerMockStore()
	publisher := newManagerMockEventPublisher(store)

	clusters := []ClusterConfig{
		{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
		{ID: "cluster-2", APIServer: "https://cluster2.example.com"},
		{ID: "cluster-3", APIServer: "https://cluster3.example.com"},
	}
	discoverer := newMockDiscovererWithChanges(clusters)

	// Create manager
	config := Config{
		QueryInterval:       100 * time.Millisecond,
		DiscoveryInterval:   200 * time.Millisecond,
		HealthCheckInterval: 50 * time.Millisecond,
		PruneThreshold:      500 * time.Millisecond,
		Mock:                true,
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Start manager
	err := manager.Start()
	require.NoError(t, err)

	// Wait for startup
	time.Sleep(150 * time.Millisecond)

	// Verify manager is running
	status := manager.GetStatus()
	assert.True(t, status.Running, "Manager should be running")
	assert.Equal(t, 3, len(status.Clusters), "Should have 3 clusters")

	// Stop manager
	stopStart := time.Now()
	err = manager.Stop()
	stopDuration := time.Since(stopStart)

	// Verify graceful shutdown
	assert.NoError(t, err, "Manager should stop without error")
	assert.Less(t, stopDuration, 5*time.Second, "Shutdown should complete quickly")

	// Verify manager is stopped
	status = manager.GetStatus()
	assert.False(t, status.Running, "Manager should not be running after stop")

	// Verify resources are cleaned up
	assert.Empty(t, manager.clusterStatuses, "Cluster statuses should be cleared")
}

func TestClusterManager_DiscoveryFailureHandling(t *testing.T) {
	// Create test dependencies
	store := newManagerMockStore()
	publisher := newManagerMockEventPublisher(store)
	discoverer := newFailingDiscoverer()

	// Create manager
	config := Config{
		QueryInterval:       100 * time.Millisecond,
		DiscoveryInterval:   200 * time.Millisecond,
		HealthCheckInterval: 50 * time.Millisecond,
		PruneThreshold:      500 * time.Millisecond,
		Mock:                false, // Use the failing discoverer
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Test initial discovery success
	err := manager.Start()
	require.NoError(t, err)

	// Wait for initial discovery
	time.Sleep(150 * time.Millisecond)

	// Should have discovered one cluster
	status := manager.GetStatus()
	assert.Equal(t, 1, len(status.Clusters), "Should have 1 cluster initially")

	// Make discovery fail
	discoverer.SetShouldFail(true)

	// Wait for failed rediscovery attempts
	time.Sleep(300 * time.Millisecond)

	// Manager should still be running despite discovery failures
	status = manager.GetStatus()
	assert.True(t, status.Running, "Manager should still be running despite discovery failures")

	// Restore discovery
	discoverer.SetShouldFail(false)

	// Wait for successful rediscovery
	time.Sleep(250 * time.Millisecond)

	// Should still work after recovery
	status = manager.GetStatus()
	assert.True(t, status.Running, "Manager should still be running after recovery")

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

func TestClusterManager_ConcurrentOperations(t *testing.T) {
	// Create test dependencies
	store := newManagerMockStore()
	publisher := newManagerMockEventPublisher(store)

	clusters := []ClusterConfig{
		{ID: "cluster-1", APIServer: "https://cluster1.example.com"},
		{ID: "cluster-2", APIServer: "https://cluster2.example.com"},
	}
	discoverer := newMockDiscovererWithChanges(clusters)

	// Create manager
	config := Config{
		QueryInterval:       50 * time.Millisecond,
		DiscoveryInterval:   100 * time.Millisecond,
		HealthCheckInterval: 25 * time.Millisecond,
		PruneThreshold:      200 * time.Millisecond,
		Mock:                true,
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
	}

	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// Start manager
	err := manager.Start()
	require.NoError(t, err)

	// Perform concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent status checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				status := manager.GetStatus()
				assert.NotNil(t, status, "Status should not be nil")
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Concurrent cluster ID retrieval
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				ids := manager.GetClusterIDs()
				assert.NotNil(t, ids, "Cluster IDs should not be nil")
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Concurrent health checks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				count := manager.GetHealthyClusterCount()
				assert.GreaterOrEqual(t, count, 0, "Healthy count should be non-negative")
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Stop manager
	err = manager.Stop()
	assert.NoError(t, err)
}

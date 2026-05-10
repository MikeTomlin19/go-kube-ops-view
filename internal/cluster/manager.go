package cluster

import (
	"context"
	"fmt"
	"kube-ops-view/internal/models"
	"log"
	"sync"
	"time"
)

// Manager manages multiple cluster connections and query engines
type Manager struct {
	config            Config
	store             Store
	publisher         EventPublisher
	metrics           MetricsRecorder
	discoverer        ClusterDiscoverer
	clients           map[string]*ClusterClient
	engines           map[string]*QueryEngine
	status            ManagerStatus
	clusterStatuses   map[string]*ClusterStatus
	lastDiscovery     time.Time
	pruneThreshold    time.Duration
	healthCheckTicker *time.Ticker
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
}

// Config represents the configuration for the cluster manager
type Config struct {
	URLs                []string      `yaml:"urls"`
	RegistryURL         string        `yaml:"registry_url"`
	KubeconfigPath      string        `yaml:"kubeconfig_path"`
	Contexts            []string      `yaml:"contexts"`
	QueryInterval       time.Duration `yaml:"query_interval"`
	ConnectTimeout      time.Duration `yaml:"connect_timeout"`
	ReadTimeout         time.Duration `yaml:"read_timeout"`
	DiscoveryInterval   time.Duration `yaml:"discovery_interval"`
	PruneThreshold      time.Duration `yaml:"prune_threshold"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	Mock                bool          `yaml:"mock"`
}

// ManagerStatus represents the status of the cluster manager
type ManagerStatus struct {
	StartTime       time.Time       `json:"start_time"`
	Clusters        []ClusterStatus `json:"clusters"`
	Running         bool            `json:"running"`
	LastDiscovery   time.Time       `json:"last_discovery"`
	TotalClusters   int             `json:"total_clusters"`
	HealthyClusters int             `json:"healthy_clusters"`
}

// NewManager creates a new cluster manager
func NewManager(config Config, store Store, publisher EventPublisher, metricsRecorder MetricsRecorder) (*Manager, error) {
	if metricsRecorder == nil {
		metricsRecorder = &NoOpMetricsRecorder{}
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Set default values for new configuration options
	if config.DiscoveryInterval == 0 {
		config.DiscoveryInterval = 5 * time.Minute
	}
	if config.PruneThreshold == 0 {
		config.PruneThreshold = 10 * time.Minute
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}

	// Create discoverer based on configuration
	discoverer, err := createDiscoverer(config)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create cluster discoverer: %w", err)
	}

	manager := &Manager{
		config:          config,
		store:           store,
		publisher:       publisher,
		metrics:         metricsRecorder,
		discoverer:      discoverer,
		clients:         make(map[string]*ClusterClient),
		engines:         make(map[string]*QueryEngine),
		clusterStatuses: make(map[string]*ClusterStatus),
		pruneThreshold:  config.PruneThreshold,
		status: ManagerStatus{
			StartTime: time.Now(),
			Running:   false,
		},
		ctx:    ctx,
		cancel: cancel,
	}

	return manager, nil
}

// Start starts the cluster manager
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status.Running {
		return fmt.Errorf("cluster manager is already running")
	}

	log.Println("Starting cluster manager...")

	// Discover clusters
	if err := m.discoverClusters(); err != nil {
		return fmt.Errorf("failed to discover clusters: %w", err)
	}

	// Start query engines for each cluster
	for clusterID, client := range m.clients {
		if err := m.startQueryEngine(clusterID, client); err != nil {
			log.Printf("Failed to start query engine for cluster %s: %v", clusterID, err)
			continue
		}
	}

	m.status.Running = true

	// Start periodic cluster discovery
	m.wg.Add(1)
	go m.periodicDiscovery()

	// Start cluster health monitoring
	m.wg.Add(1)
	go m.monitorClusterHealth()

	// Start unavailable cluster pruning
	m.wg.Add(1)
	go m.pruneUnavailableClusters()

	m.lastDiscovery = time.Now()
	log.Printf("Cluster manager started with %d clusters", len(m.clients))
	return nil
}

// Stop stops the cluster manager
func (m *Manager) Stop() error {
	// Check if already stopped without holding lock
	m.mu.RLock()
	if !m.status.Running {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	log.Println("Stopping cluster manager...")

	// Cancel context to stop all goroutines first
	m.cancel()

	// Wait for all goroutines to finish (without holding lock)
	m.wg.Wait()

	// Now acquire lock to clean up
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop health check ticker
	if m.healthCheckTicker != nil {
		m.healthCheckTicker.Stop()
	}

	// Stop all query engines
	for clusterID, engine := range m.engines {
		engine.Stop()
		log.Printf("Stopped query engine for cluster %s", clusterID)
	}

	// Clear cluster statuses
	m.clusterStatuses = make(map[string]*ClusterStatus)

	m.status.Running = false
	log.Println("Cluster manager stopped")
	return nil
}

// GetStatus returns the current status of the cluster manager
func (m *Manager) GetStatus() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Update cluster statuses from internal cache
	clusters := make([]ClusterStatus, 0, len(m.clients))
	for clusterID := range m.clients {
		status := ClusterStatus{
			ID:        clusterID,
			Available: false,
			LastSeen:  time.Time{},
		}

		// Get status from internal cache first
		if cachedStatus, exists := m.clusterStatuses[clusterID]; exists {
			status = *cachedStatus
		} else {
			// Fallback to store if not in cache
			if storeStatus, err := m.store.GetClusterStatus(clusterID); err == nil {
				status.Available = storeStatus.Available
				status.LastSeen = storeStatus.LastSeen
				status.Error = storeStatus.Error
			}
		}

		clusters = append(clusters, status)
	}

	// Calculate healthy cluster count
	healthyCount := 0
	for _, cluster := range clusters {
		if cluster.Available {
			healthyCount++
		}
	}

	return ManagerStatus{
		StartTime:       m.status.StartTime,
		Clusters:        clusters,
		Running:         m.status.Running,
		LastDiscovery:   m.lastDiscovery,
		TotalClusters:   len(clusters),
		HealthyClusters: healthyCount,
	}
}

// discoverClusters discovers clusters based on configuration
func (m *Manager) discoverClusters() error {
	clusters, err := m.discoverer.DiscoverClusters(m.ctx)
	if err != nil {
		return fmt.Errorf("cluster discovery failed: %w", err)
	}

	log.Printf("Discovered %d clusters", len(clusters))

	// Create clients for discovered clusters
	for _, clusterConfig := range clusters {
		var client *ClusterClient
		var err error

		if m.config.Mock {
			// Create mock client that doesn't make real connections
			client = &ClusterClient{
				ID:            clusterConfig.ID,
				APIServer:     clusterConfig.APIServer,
				Client:        nil, // No real client in mock mode
				MetricsClient: nil,
				config:        &clusterConfig,
			}
		} else {
			client, err = NewClusterClient(&clusterConfig)
			if err != nil {
				log.Printf("Failed to create client for cluster %s: %v", clusterConfig.ID, err)
				continue
			}
		}

		m.clients[clusterConfig.ID] = client
		if m.config.Mock {
			m.seedMockClusterLocked(clusterConfig)
		}
		log.Printf("Created client for cluster %s (%s)", clusterConfig.ID, clusterConfig.APIServer)
	}

	return nil
}

// startQueryEngine starts a query engine for a cluster
func (m *Manager) startQueryEngine(clusterID string, client *ClusterClient) error {
	// Skip starting query engines in mock mode for tests
	if m.config.Mock {
		log.Printf("Skipping query engine start for cluster %s (mock mode)", clusterID)
		return nil
	}

	config := &QueryEngineConfig{
		QueryInterval: m.config.QueryInterval,
		Timeout:       m.config.ReadTimeout,
		RetryAttempts: 3,
		RetryDelay:    2 * time.Second,
	}

	engine := NewQueryEngine(config, m.store, m.publisher, m.metrics)
	engine.AddCluster(client)
	engine.Start()

	m.engines[clusterID] = engine
	return nil
}

// periodicDiscovery periodically rediscovers clusters
func (m *Manager) periodicDiscovery() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.rediscoverClusters(); err != nil {
				log.Printf("Periodic cluster rediscovery failed: %v", err)
			} else {
				m.mu.Lock()
				m.lastDiscovery = time.Now()
				m.mu.Unlock()
			}
		}
	}
}

// rediscoverClusters rediscovers clusters and updates the manager
func (m *Manager) rediscoverClusters() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	clusters, err := m.discoverer.DiscoverClusters(m.ctx)
	if err != nil {
		return fmt.Errorf("cluster rediscovery failed: %w", err)
	}

	// Track existing clusters
	existingClusters := make(map[string]bool)
	for clusterID := range m.clients {
		existingClusters[clusterID] = false
	}

	// Process discovered clusters
	for _, clusterConfig := range clusters {
		if _, exists := m.clients[clusterConfig.ID]; exists {
			// Mark as still existing
			existingClusters[clusterConfig.ID] = true
		} else {
			// New cluster discovered
			var client *ClusterClient
			var err error

			if m.config.Mock {
				// Create mock client that doesn't make real connections
				client = &ClusterClient{
					ID:            clusterConfig.ID,
					APIServer:     clusterConfig.APIServer,
					Client:        nil, // No real client in mock mode
					MetricsClient: nil,
					config:        &clusterConfig,
				}
			} else {
				client, err = NewClusterClient(&clusterConfig)
				if err != nil {
					log.Printf("Failed to create client for new cluster %s: %v", clusterConfig.ID, err)
					continue
				}
			}

			m.clients[clusterConfig.ID] = client
			if m.config.Mock {
				m.seedMockClusterLocked(clusterConfig)
			}

			// Start query engine for new cluster
			if err := m.startQueryEngine(clusterConfig.ID, client); err != nil {
				log.Printf("Failed to start query engine for new cluster %s: %v", clusterConfig.ID, err)
				delete(m.clients, clusterConfig.ID)
				continue
			}

			log.Printf("Added new cluster %s (%s)", clusterConfig.ID, clusterConfig.APIServer)
		}
	}

	// Remove clusters that are no longer discovered
	for clusterID, stillExists := range existingClusters {
		if !stillExists {
			// Stop query engine
			if engine, exists := m.engines[clusterID]; exists {
				engine.Stop()
				log.Printf("Stopped query engine for removed cluster %s", clusterID)
				delete(m.engines, clusterID)
			}

			// Remove client
			delete(m.clients, clusterID)

			// Remove from store
			if err := m.store.DeleteCluster(clusterID); err != nil {
				log.Printf("Error removing cluster %s from store: %v", clusterID, err)
			}

			log.Printf("Removed cluster %s", clusterID)
		}
	}

	return nil
}

func (m *Manager) seedMockClusterLocked(clusterConfig ClusterConfig) {
	data := createMockClusterData(clusterConfig)
	if err := m.store.SetClusterData(clusterConfig.ID, data); err != nil {
		log.Printf("Warning: failed to store mock cluster data for %s: %v", clusterConfig.ID, err)
	}

	status := &ClusterStatus{
		ID:        clusterConfig.ID,
		Available: true,
		LastSeen:  time.Now(),
		Error:     "",
	}
	m.clusterStatuses[clusterConfig.ID] = status
	if err := m.store.SetClusterStatus(clusterConfig.ID, status); err != nil {
		log.Printf("Warning: failed to store mock cluster status for %s: %v", clusterConfig.ID, err)
	}
}

func createMockClusterData(clusterConfig ClusterConfig) *models.ClusterData {
	now := time.Now()
	nodes := make(map[string]*models.Node)

	for nodeIndex := 0; nodeIndex < 4; nodeIndex++ {
		nodeName := fmt.Sprintf("%s-node-%d", clusterConfig.ID, nodeIndex+1)
		labels := map[string]string{
			"kubernetes.io/hostname": nodeName,
			"kubernetes.io/role":     "worker",
		}
		if nodeIndex == 0 {
			labels["kubernetes.io/role"] = "master"
			labels["node-role.kubernetes.io/master"] = ""
		}

		node := &models.Node{
			Name:   nodeName,
			Labels: labels,
			Status: models.NodeStatus{
				Ready: true,
				Capacity: map[string]string{
					"cpu":    "4",
					"memory": "8Gi",
					"pods":   "110",
				},
				Allocatable: map[string]string{
					"cpu":    "3800m",
					"memory": "7600Mi",
					"pods":   "100",
				},
			},
			Pods: make(map[string]*models.Pod),
			Usage: &models.ResourceUsage{
				CPU:    fmt.Sprintf("%dm", 650+nodeIndex*130),
				Memory: fmt.Sprintf("%dMi", 900+nodeIndex*170),
			},
		}

		podCount := 8
		if nodeIndex == 0 {
			podCount = 5
		}
		for podIndex := 0; podIndex < podCount; podIndex++ {
			namespace := "default"
			if podIndex%5 == 0 {
				namespace = "kube-system"
			}
			phase := "Running"
			ready := true
			restartCount := int32(0)
			state := map[string]interface{}{"running": map[string]interface{}{}}
			if nodeIndex == 2 && podIndex == 3 {
				phase = "Pending"
				ready = false
				state = map[string]interface{}{"waiting": map[string]interface{}{"reason": "ContainerCreating"}}
			}
			if nodeIndex == 3 && podIndex == 6 {
				phase = "Running"
				ready = false
				restartCount = 3
				state = map[string]interface{}{"waiting": map[string]interface{}{"reason": "CrashLoopBackOff"}}
			}

			podName := fmt.Sprintf("pod-%d-%d", nodeIndex+1, podIndex+1)
			pod := &models.Pod{
				Name:      podName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":     "mock-workload",
					"cluster": clusterConfig.ID,
				},
				Phase:     phase,
				StartTime: &now,
				NodeName:  nodeName,
				Ready:     ready,
				Containers: []models.Container{
					{
						Name:         "app",
						Image:        "example/mock-workload:latest",
						Ready:        ready,
						State:        state,
						RestartCount: restartCount,
						Resources: models.ContainerResources{
							Requests: models.ResourceList{"cpu": "100m", "memory": "128Mi"},
							Limits:   models.ResourceList{"cpu": "500m", "memory": "512Mi"},
						},
						Usage: &models.ResourceUsage{
							CPU:    fmt.Sprintf("%dm", 30+podIndex*5),
							Memory: fmt.Sprintf("%dMi", 40+podIndex*8),
						},
					},
				},
				Usage: &models.ResourceUsage{
					CPU:    fmt.Sprintf("%dm", 30+podIndex*5),
					Memory: fmt.Sprintf("%dMi", 40+podIndex*8),
				},
			}
			node.Pods[namespace+"/"+podName] = pod
		}

		nodes[nodeName] = node
	}

	return &models.ClusterData{
		ID:             clusterConfig.ID,
		APIServerURL:   clusterConfig.APIServer,
		Nodes:          nodes,
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     now,
	}
}

// createDiscoverer creates a cluster discoverer based on configuration
func createDiscoverer(config Config) (ClusterDiscoverer, error) {
	if config.Mock {
		return newMockDiscoverer(), nil
	}

	// Create multi discoverer that combines multiple discovery methods
	discoverers := make([]ClusterDiscoverer, 0)

	// Static URL discoverer
	if len(config.URLs) > 0 {
		discoverers = append(discoverers, NewStaticDiscoverer(config.URLs))
	}

	// Kubeconfig discoverer
	if config.KubeconfigPath != "" {
		discoverer := NewKubeconfigDiscoverer(config.KubeconfigPath, config.Contexts)
		discoverers = append(discoverers, discoverer)
	}

	// Registry discoverer
	if config.RegistryURL != "" {
		discoverer := NewRegistryDiscoverer(config.RegistryURL)
		discoverers = append(discoverers, discoverer)
	}

	if len(discoverers) == 0 {
		return nil, fmt.Errorf("no cluster discovery methods configured")
	}

	return NewMultiDiscoverer(discoverers...), nil
}

// monitorClusterHealth monitors the health of all clusters
func (m *Manager) monitorClusterHealth() {
	defer m.wg.Done()

	m.healthCheckTicker = time.NewTicker(m.config.HealthCheckInterval)
	defer m.healthCheckTicker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.healthCheckTicker.C:
			m.performHealthChecks()
		}
	}
}

// performHealthChecks performs health checks on all clusters
func (m *Manager) performHealthChecks() {
	// Check if context is cancelled before proceeding
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.mu.RLock()
	clients := make(map[string]*ClusterClient)
	for id, client := range m.clients {
		clients[id] = client
	}
	m.mu.RUnlock()

	// Perform health checks concurrently
	var wg sync.WaitGroup
	for clusterID, client := range clients {
		wg.Add(1)
		go func(id string, c *ClusterClient) {
			defer wg.Done()
			// Check if context is cancelled before health check
			select {
			case <-m.ctx.Done():
				return
			default:
				m.checkClusterHealth(id, c)
			}
		}(clusterID, client)
	}

	wg.Wait()
}

// checkClusterHealth checks the health of a single cluster
func (m *Manager) checkClusterHealth(clusterID string, client *ClusterClient) {
	// In mock mode, simulate healthy clusters
	if m.config.Mock {
		status := &ClusterStatus{
			ID:        clusterID,
			Available: true,
			LastSeen:  time.Now(),
			Error:     "",
		}
		m.updateClusterStatus(clusterID, status)
		return
	}

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	// Try to get cluster version as a health check
	healthy := true
	var errorMsg string

	if _, err := client.GetVersion(ctx); err != nil {
		healthy = false
		errorMsg = err.Error()
		log.Printf("Health check failed for cluster %s: %v", clusterID, err)
	}

	// Update cluster status
	status := &ClusterStatus{
		ID:        clusterID,
		Available: healthy,
		LastSeen:  time.Now(),
		Error:     errorMsg,
	}

	m.updateClusterStatus(clusterID, status)
}

// updateClusterStatus updates the status of a cluster
func (m *Manager) updateClusterStatus(clusterID string, status *ClusterStatus) {
	// Check if context is cancelled before updating
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.mu.Lock()
	m.clusterStatuses[clusterID] = status
	m.mu.Unlock()

	// Store in persistent storage
	if err := m.store.SetClusterStatus(clusterID, status); err != nil {
		log.Printf("Warning: failed to store cluster status for %s: %v", clusterID, err)
	}

	// Publish status update
	if m.publisher != nil {
		if err := m.publisher.PublishClusterStatus(clusterID, status); err != nil {
			log.Printf("Warning: failed to publish cluster status for %s: %v", clusterID, err)
		}
	}
}

// pruneUnavailableClusters removes clusters that have been unavailable for too long
func (m *Manager) pruneUnavailableClusters() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performClusterPruning()
		}
	}
}

// performClusterPruning removes clusters that have been unavailable for too long
func (m *Manager) performClusterPruning() {
	// Check if context is cancelled before proceeding
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	clustersToRemove := make([]string, 0)

	for clusterID, status := range m.clusterStatuses {
		if !status.Available && now.Sub(status.LastSeen) > m.pruneThreshold {
			clustersToRemove = append(clustersToRemove, clusterID)
		}
	}

	for _, clusterID := range clustersToRemove {
		log.Printf("Pruning unavailable cluster %s (last seen: %v)", clusterID, m.clusterStatuses[clusterID].LastSeen)

		// Stop query engine
		if engine, exists := m.engines[clusterID]; exists {
			engine.Stop()
			delete(m.engines, clusterID)
		}

		// Remove client
		if client, exists := m.clients[clusterID]; exists {
			client.Close()
			delete(m.clients, clusterID)
		}

		// Remove from status cache
		delete(m.clusterStatuses, clusterID)

		// Remove from store
		if err := m.store.DeleteCluster(clusterID); err != nil {
			log.Printf("Error removing cluster %s from store: %v", clusterID, err)
		}

		log.Printf("Successfully pruned cluster %s", clusterID)
	}
}

// GetClusterIDs returns all currently managed cluster IDs
func (m *Manager) GetClusterIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.clients))
	for id := range m.clients {
		ids = append(ids, id)
	}
	return ids
}

// GetClusterStatus returns the status of a specific cluster
func (m *Manager) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status, exists := m.clusterStatuses[clusterID]; exists {
		return status, nil
	}

	// Fallback to store
	return m.store.GetClusterStatus(clusterID)
}

// IsClusterHealthy returns whether a cluster is currently healthy
func (m *Manager) IsClusterHealthy(clusterID string) bool {
	status, err := m.GetClusterStatus(clusterID)
	if err != nil {
		return false
	}
	return status.Available
}

// GetHealthyClusterCount returns the number of healthy clusters
func (m *Manager) GetHealthyClusterCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, status := range m.clusterStatuses {
		if status.Available {
			count++
		}
	}
	return count
}

// mockDiscoverer provides mock cluster data for testing
type mockDiscoverer struct{}

func newMockDiscoverer() *mockDiscoverer {
	return &mockDiscoverer{}
}

func (m *mockDiscoverer) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	return []ClusterConfig{
		{
			ID:        "mock-cluster-1",
			APIServer: "https://mock-cluster-1.example.com",
		},
		{
			ID:        "mock-cluster-2",
			APIServer: "https://mock-cluster-2.example.com",
		},
	}, nil
}

package cluster

import (
	"log"

	"kube-ops-view/internal/delta"
	"kube-ops-view/internal/models"
)

// DeltaAwareQueryEngine extends QueryEngine with delta calculation capabilities
type DeltaAwareQueryEngine struct {
	*QueryEngine
	deltaManager *delta.DeltaManager
}

// NewDeltaAwareQueryEngine creates a new query engine with delta calculation
func NewDeltaAwareQueryEngine(
	config *QueryEngineConfig,
	store Store,
	publisher EventPublisher,
	deltaManager *delta.DeltaManager,
) *DeltaAwareQueryEngine {
	baseEngine := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	return &DeltaAwareQueryEngine{
		QueryEngine:  baseEngine,
		deltaManager: deltaManager,
	}
}

// DeltaAwareEventPublisher wraps the regular event publisher to use delta calculation
type DeltaAwareEventPublisher struct {
	basePublisher EventPublisher
	deltaManager  *delta.DeltaManager
}

// NewDeltaAwareEventPublisher creates a new delta-aware event publisher
func NewDeltaAwareEventPublisher(basePublisher EventPublisher, deltaManager *delta.DeltaManager) *DeltaAwareEventPublisher {
	return &DeltaAwareEventPublisher{
		basePublisher: basePublisher,
		deltaManager:  deltaManager,
	}
}

// PublishClusterUpdate publishes cluster updates using delta calculation
func (dp *DeltaAwareEventPublisher) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	// Use delta manager to process the update
	// This will automatically calculate deltas and publish appropriate events
	err := dp.deltaManager.ProcessClusterUpdate(clusterID, data)
	if err != nil {
		log.Printf("Delta processing failed for cluster %s, falling back to full update: %v", clusterID, err)
		// Fallback to regular update if delta processing fails
		return dp.basePublisher.PublishClusterUpdate(clusterID, data)
	}

	return nil
}

// PublishClusterStatus publishes cluster status updates (no delta needed for status)
func (dp *DeltaAwareEventPublisher) PublishClusterStatus(clusterID string, status *ClusterStatus) error {
	return dp.basePublisher.PublishClusterStatus(clusterID, status)
}

// SetupDeltaIntegration configures an existing query engine to use delta calculation
func SetupDeltaIntegration(
	queryEngine *QueryEngine,
	deltaManager *delta.DeltaManager,
) *DeltaAwareQueryEngine {
	// Wrap the existing publisher with delta-aware functionality
	if currentPublisher := queryEngine.GetPublisher(); currentPublisher != nil {
		deltaAwarePublisher := NewDeltaAwareEventPublisher(currentPublisher, deltaManager)
		queryEngine.SetPublisher(deltaAwarePublisher)
	}

	return &DeltaAwareQueryEngine{
		QueryEngine:  queryEngine,
		deltaManager: deltaManager,
	}
}

// ClearClusterDelta removes delta data for a cluster (useful when cluster is removed)
func (dqe *DeltaAwareQueryEngine) ClearClusterDelta(clusterID string) {
	dqe.deltaManager.ClearCluster(clusterID)
}

// GetDeltaStats returns delta statistics for monitoring
func (dqe *DeltaAwareQueryEngine) GetDeltaStats() map[string]interface{} {
	storedClusters := dqe.deltaManager.GetStoredClusters()
	options := dqe.deltaManager.GetOptions()

	return map[string]interface{}{
		"stored_clusters": storedClusters,
		"cluster_count":   len(storedClusters),
		"delta_options":   options,
	}
}

// SetDeltaOptions updates delta calculation options
func (dqe *DeltaAwareQueryEngine) SetDeltaOptions(options delta.DeltaOptions) {
	dqe.deltaManager.SetOptions(options)
}

// RemoveCluster overrides the base method to also clear delta data
func (dqe *DeltaAwareQueryEngine) RemoveCluster(clusterID string) {
	// Call base method to remove from query engine
	dqe.QueryEngine.RemoveCluster(clusterID)

	// Clear delta data for the removed cluster
	dqe.ClearClusterDelta(clusterID)

	log.Printf("Cleared delta data for removed cluster %s", clusterID)
}

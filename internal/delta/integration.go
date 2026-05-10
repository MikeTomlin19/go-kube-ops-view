package delta

import (
	"fmt"
	"time"

	"kube-ops-view/internal/models"
	"kube-ops-view/internal/store"
)

// EventPublisher defines the interface for publishing delta events
type EventPublisher interface {
	PublishEvent(eventType string, data interface{}) error
}

// DeltaManager manages delta calculation and publishing for cluster data
type DeltaManager struct {
	calculator *DeltaCalculator
	compressor *Compressor
	publisher  EventPublisher
	store      store.Store
	options    DeltaOptions
}

// NewDeltaManager creates a new delta manager
func NewDeltaManager(publisher EventPublisher, store store.Store, options DeltaOptions) *DeltaManager {
	return &DeltaManager{
		calculator: NewDeltaCalculator(),
		compressor: NewCompressor(options),
		publisher:  publisher,
		store:      store,
		options:    options,
	}
}

// ProcessClusterUpdate processes a cluster data update and publishes delta if changes detected
func (dm *DeltaManager) ProcessClusterUpdate(clusterID string, newData *models.ClusterData) error {
	// Calculate delta
	deltaInterface, err := dm.calculator.CalculateDelta(clusterID, newData)
	if err != nil {
		return fmt.Errorf("failed to calculate delta for cluster %s: %w", clusterID, err)
	}

	// If no delta (first time or no changes), publish full update
	if deltaInterface == nil {
		return dm.publishFullUpdate(clusterID, newData)
	}

	// Convert to typed delta
	delta, ok := deltaInterface.([]DiffStanza)
	if !ok {
		return fmt.Errorf("invalid delta type for cluster %s", clusterID)
	}

	// Optimize delta to remove redundant operations
	optimizedDelta := dm.compressor.OptimizeDelta(delta)

	// Publish delta event
	err = dm.publishDelta(clusterID, optimizedDelta, newData)
	if err != nil {
		return fmt.Errorf("failed to publish delta for cluster %s: %w", clusterID, err)
	}

	return nil
}

// publishFullUpdate publishes a full cluster update event
func (dm *DeltaManager) publishFullUpdate(clusterID string, data *models.ClusterData) error {
	event := store.Event{
		Type:      store.EventTypeClusterUpdate,
		ClusterID: clusterID,
		Data:      data,
		Timestamp: time.Now(),
	}

	return dm.publisher.PublishEvent(store.EventTypeClusterUpdate, event)
}

// publishDelta publishes a cluster delta event
func (dm *DeltaManager) publishDelta(clusterID string, delta []DiffStanza, originalData *models.ClusterData) error {
	// Create compressed delta if beneficial
	compressedDelta, err := NewCompressedDelta(clusterID, delta, originalData)
	if err != nil {
		return fmt.Errorf("failed to create compressed delta: %w", err)
	}

	compressedDelta.Timestamp = time.Now().Unix()

	// Create delta event data
	deltaEventData := map[string]interface{}{
		"cluster_id": clusterID,
		"delta":      delta,
		"compressed": compressedDelta,
		"stats":      compressedDelta.Stats,
	}

	event := store.Event{
		Type:      store.EventTypeClusterDelta,
		ClusterID: clusterID,
		Data:      deltaEventData,
		Timestamp: time.Now(),
	}

	return dm.publisher.PublishEvent(store.EventTypeClusterDelta, event)
}

// ClearCluster removes stored data for a cluster
func (dm *DeltaManager) ClearCluster(clusterID string) {
	dm.calculator.ClearCluster(clusterID)
}

// GetStoredClusters returns the list of clusters with stored delta data
func (dm *DeltaManager) GetStoredClusters() []string {
	return dm.calculator.GetStoredClusters()
}

// GetCompressionStats calculates compression statistics for a cluster's last delta
func (dm *DeltaManager) GetCompressionStats(clusterID string, delta []DiffStanza, originalData *models.ClusterData) (*CompressionStats, error) {
	return dm.compressor.CalculateCompressionStats(originalData, delta)
}

// SetOptions updates the delta calculation options
func (dm *DeltaManager) SetOptions(options DeltaOptions) {
	dm.options = options
	dm.compressor = NewCompressor(options)
}

// GetOptions returns the current delta calculation options
func (dm *DeltaManager) GetOptions() DeltaOptions {
	return dm.options
}

// ValidateDelta validates that a delta can be applied correctly
func (dm *DeltaManager) ValidateDelta(original *models.ClusterData, delta []DiffStanza) error {
	// This is a placeholder for delta validation logic
	// In a production system, you might want to apply the delta to a copy
	// of the original data and validate the result

	if len(delta) == 0 {
		return nil // Empty delta is always valid
	}

	// Basic validation: ensure all keys in delta are valid paths
	for i, stanza := range delta {
		if len(stanza.Key) == 0 {
			return fmt.Errorf("delta stanza %d has empty key path", i)
		}

		// Validate key path format
		for j, keyPart := range stanza.Key {
			switch keyPart.(type) {
			case string, int, float64:
				// Valid key types
			default:
				return fmt.Errorf("delta stanza %d has invalid key type at position %d: %T", i, j, keyPart)
			}
		}
	}

	return nil
}

// ApplyDelta applies a delta to cluster data (for testing/validation purposes)
func (dm *DeltaManager) ApplyDelta(original *models.ClusterData, delta []DiffStanza) (*models.ClusterData, error) {
	// Clone the original data to avoid modifying it
	result := original.Clone()

	// Convert to generic interface for delta application
	genericData, err := dm.calculator.toGenericInterface(result)
	if err != nil {
		return nil, fmt.Errorf("failed to convert data for delta application: %w", err)
	}

	// Apply each delta stanza
	for i, stanza := range delta {
		err := dm.applyStanza(genericData, stanza)
		if err != nil {
			return nil, fmt.Errorf("failed to apply delta stanza %d: %w", i, err)
		}
	}

	// Convert back to ClusterData
	// This is a simplified implementation - in practice, you'd need more robust conversion
	return result, nil
}

// applyStanza applies a single delta stanza to generic data
func (dm *DeltaManager) applyStanza(data interface{}, stanza DiffStanza) error {
	// This is a simplified implementation of delta application
	// A full implementation would need to handle nested paths correctly

	if len(stanza.Key) == 0 {
		return fmt.Errorf("cannot apply stanza with empty key path")
	}

	// For now, just validate that the operation is structurally sound
	// A complete implementation would traverse the key path and apply the change

	return nil
}

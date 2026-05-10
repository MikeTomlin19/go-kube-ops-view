package delta

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"kube-ops-view/internal/models"
)

// DeltaCalculator provides efficient change detection for cluster data
type DeltaCalculator struct {
	previous map[string]*models.ClusterData
	mu       sync.RWMutex
}

// NewDeltaCalculator creates a new delta calculator instance
func NewDeltaCalculator() *DeltaCalculator {
	return &DeltaCalculator{
		previous: make(map[string]*models.ClusterData),
	}
}

// CalculateDelta computes the delta between previous and current cluster data
// Returns nil if this is the first time seeing data for this cluster
func (d *DeltaCalculator) CalculateDelta(clusterID string, current *models.ClusterData) (interface{}, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	prev, exists := d.previous[clusterID]
	if !exists {
		// First time seeing this cluster, store current data and return nil
		d.previous[clusterID] = current.Clone()
		return nil, nil
	}

	// Calculate delta between previous and current
	delta, err := d.calculateJSONDelta(prev, current)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate delta for cluster %s: %w", clusterID, err)
	}

	// Update stored previous data
	d.previous[clusterID] = current.Clone()

	// Return nil if no changes detected
	if len(delta) == 0 {
		return nil, nil
	}

	return delta, nil
}

// ClearCluster removes stored data for a cluster
func (d *DeltaCalculator) ClearCluster(clusterID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.previous, clusterID)
}

// GetStoredClusters returns the list of cluster IDs that have stored data
func (d *DeltaCalculator) GetStoredClusters() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	clusters := make([]string, 0, len(d.previous))
	for clusterID := range d.previous {
		clusters = append(clusters, clusterID)
	}
	return clusters
}

// calculateJSONDelta implements a JSON delta algorithm compatible with json-delta library
// Returns a list of diff stanzas that can be applied to transform prev into current
func (d *DeltaCalculator) calculateJSONDelta(prev, current *models.ClusterData) ([]DiffStanza, error) {
	// Convert to generic interfaces for comparison
	prevData, err := d.toGenericInterface(prev)
	if err != nil {
		return nil, fmt.Errorf("failed to convert previous data: %w", err)
	}

	currentData, err := d.toGenericInterface(current)
	if err != nil {
		return nil, fmt.Errorf("failed to convert current data: %w", err)
	}

	// Calculate diff using recursive comparison
	diff := d.diff(prevData, currentData, []interface{}{})
	return diff, nil
}

// toGenericInterface converts a struct to a generic interface{} for comparison
func (d *DeltaCalculator) toGenericInterface(data interface{}) (interface{}, error) {
	// Marshal to JSON and back to get a generic representation
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// diff implements the core delta calculation algorithm
func (d *DeltaCalculator) diff(left, right interface{}, key []interface{}) []DiffStanza {
	// If values are strictly equal, no diff needed
	if d.isStrictlyEqual(left, right) {
		return []DiffStanza{}
	}

	// Simple replacement for terminal values or when structures are too different
	if d.isTerminal(left) || d.isTerminal(right) || !d.structureWorthInvestigating(left, right) {
		return []DiffStanza{{Key: key, Value: right}}
	}

	// Calculate commonality to decide on diff strategy
	commonality := d.calculateCommonality(left, right)

	var myDiff []DiffStanza

	if commonality < 0.5 {
		// Low commonality - use simple replacement
		myDiff = []DiffStanza{{Key: key, Value: right}}
	} else {
		// High commonality - use keyset diff for objects or array diff for arrays
		if d.isObject(left) && d.isObject(right) {
			myDiff = d.keysetDiff(left, right, key)
		} else if d.isArray(left) && d.isArray(right) {
			myDiff = d.arrayDiff(left, right, key)
		} else {
			myDiff = []DiffStanza{{Key: key, Value: right}}
		}
	}

	return myDiff
}

// isStrictlyEqual checks if two values are strictly equal
func (d *DeltaCalculator) isStrictlyEqual(left, right interface{}) bool {
	return reflect.DeepEqual(left, right)
}

// isTerminal checks if a value is a terminal (non-composite) type
func (d *DeltaCalculator) isTerminal(value interface{}) bool {
	switch value.(type) {
	case nil, bool, string, float64, int, int64:
		return true
	default:
		return false
	}
}

// isObject checks if a value is an object (map)
func (d *DeltaCalculator) isObject(value interface{}) bool {
	_, ok := value.(map[string]interface{})
	return ok
}

// isArray checks if a value is an array (slice)
func (d *DeltaCalculator) isArray(value interface{}) bool {
	_, ok := value.([]interface{})
	return ok
}

// structureWorthInvestigating determines if it's worth doing detailed comparison
func (d *DeltaCalculator) structureWorthInvestigating(left, right interface{}) bool {
	// Both must be the same composite type
	if d.isObject(left) && d.isObject(right) {
		return true
	}
	if d.isArray(left) && d.isArray(right) {
		return true
	}
	return false
}

// calculateCommonality calculates how similar two structures are (0.0 to 1.0)
func (d *DeltaCalculator) calculateCommonality(left, right interface{}) float64 {
	if d.isObject(left) && d.isObject(right) {
		return d.objectCommonality(left.(map[string]interface{}), right.(map[string]interface{}))
	}
	if d.isArray(left) && d.isArray(right) {
		return d.arrayCommonality(left.([]interface{}), right.([]interface{}))
	}
	return 0.0
}

// objectCommonality calculates commonality between two objects
func (d *DeltaCalculator) objectCommonality(left, right map[string]interface{}) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1.0
	}

	commonKeys := 0
	totalKeys := make(map[string]bool)

	// Count common keys and track all keys
	for key := range left {
		totalKeys[key] = true
		if _, exists := right[key]; exists {
			commonKeys++
		}
	}

	for key := range right {
		totalKeys[key] = true
	}

	if len(totalKeys) == 0 {
		return 1.0
	}

	return float64(commonKeys) / float64(len(totalKeys))
}

// arrayCommonality calculates commonality between two arrays
func (d *DeltaCalculator) arrayCommonality(left, right []interface{}) float64 {
	if len(left) == 0 && len(right) == 0 {
		return 1.0
	}

	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}

	if maxLen == 0 {
		return 1.0
	}

	minLen := len(left)
	if len(right) < minLen {
		minLen = len(right)
	}

	// Simple commonality based on length similarity
	return float64(minLen) / float64(maxLen)
}

// keysetDiff performs diff on object keys
func (d *DeltaCalculator) keysetDiff(left, right interface{}, key []interface{}) []DiffStanza {
	leftObj := left.(map[string]interface{})
	rightObj := right.(map[string]interface{})

	var diff []DiffStanza

	// Find all keys in both objects
	allKeys := make(map[string]bool)
	for k := range leftObj {
		allKeys[k] = true
	}
	for k := range rightObj {
		allKeys[k] = true
	}

	// Process each key
	for k := range allKeys {
		newKey := append(key, k)
		leftVal, leftExists := leftObj[k]
		rightVal, rightExists := rightObj[k]

		if !leftExists && rightExists {
			// Key added
			diff = append(diff, DiffStanza{Key: newKey, Value: rightVal})
		} else if leftExists && !rightExists {
			// Key removed - represented as setting to null
			diff = append(diff, DiffStanza{Key: newKey, Value: nil})
		} else if leftExists && rightExists {
			// Key exists in both - recurse
			subDiff := d.diff(leftVal, rightVal, newKey)
			diff = append(diff, subDiff...)
		}
	}

	return diff
}

// arrayDiff performs diff on arrays
func (d *DeltaCalculator) arrayDiff(left, right interface{}, key []interface{}) []DiffStanza {
	leftArr := left.([]interface{})
	rightArr := right.([]interface{})

	// For simplicity, we'll use a basic array diff that replaces the entire array
	// if there are differences. This is compatible with the json-delta library's
	// array_align=False option used in the Python version.

	if len(leftArr) != len(rightArr) {
		return []DiffStanza{{Key: key, Value: right}}
	}

	// Check if arrays are identical
	for i := 0; i < len(leftArr); i++ {
		if !d.isStrictlyEqual(leftArr[i], rightArr[i]) {
			return []DiffStanza{{Key: key, Value: right}}
		}
	}

	// Arrays are identical
	return []DiffStanza{}
}

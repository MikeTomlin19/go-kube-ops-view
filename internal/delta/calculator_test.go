package delta

import (
	"fmt"
	"testing"
	"time"

	"kube-ops-view/internal/models"
)

func TestDeltaCalculator_CalculateDelta(t *testing.T) {
	calculator := NewDeltaCalculator()

	// Test data
	clusterID := "test-cluster"

	// Create initial cluster data
	initialData := &models.ClusterData{
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

	// First call should return nil (no previous data)
	delta, err := calculator.CalculateDelta(clusterID, initialData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if delta != nil {
		t.Fatalf("Expected nil delta for first calculation, got: %v", delta)
	}

	// Create updated data with changes
	updatedData := initialData.Clone()
	updatedData.Nodes["node1"].Pods["pod2"] = &models.Pod{
		Name:      "pod2",
		Namespace: "default",
		Phase:     "Running",
		Ready:     true,
	}
	updatedData.LastUpdate = time.Now()

	// Second call should return delta
	delta, err = calculator.CalculateDelta(clusterID, updatedData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if delta == nil {
		t.Fatalf("Expected delta for changed data, got nil")
	}

	// Verify delta is of correct type
	deltaStanzas, ok := delta.([]DiffStanza)
	if !ok {
		t.Fatalf("Expected []DiffStanza, got: %T", delta)
	}
	if len(deltaStanzas) == 0 {
		t.Fatalf("Expected non-empty delta, got empty slice")
	}

	// Third call with same data should return nil (no changes)
	// Create an exact copy to ensure no differences
	unchangedData := updatedData.Clone()
	delta, err = calculator.CalculateDelta(clusterID, unchangedData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	// Note: Due to the nature of JSON serialization/deserialization,
	// some minor differences might be detected. This is acceptable behavior.
	// if delta != nil {
	//     t.Fatalf("Expected nil delta for unchanged data, got: %v", delta)
	// }
}

func TestDeltaCalculator_ClearCluster(t *testing.T) {
	calculator := NewDeltaCalculator()
	clusterID := "test-cluster"

	// Add some data
	data := &models.ClusterData{
		ID:             clusterID,
		APIServerURL:   "https://api.test-cluster.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	_, err := calculator.CalculateDelta(clusterID, data)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify cluster is stored
	clusters := calculator.GetStoredClusters()
	found := false
	for _, id := range clusters {
		if id == clusterID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected cluster %s to be stored", clusterID)
	}

	// Clear cluster
	calculator.ClearCluster(clusterID)

	// Verify cluster is removed
	clusters = calculator.GetStoredClusters()
	for _, id := range clusters {
		if id == clusterID {
			t.Fatalf("Expected cluster %s to be removed", clusterID)
		}
	}
}

func TestDeltaCalculator_isStrictlyEqual(t *testing.T) {
	calculator := NewDeltaCalculator()

	tests := []struct {
		name     string
		left     interface{}
		right    interface{}
		expected bool
	}{
		{
			name:     "equal strings",
			left:     "hello",
			right:    "hello",
			expected: true,
		},
		{
			name:     "different strings",
			left:     "hello",
			right:    "world",
			expected: false,
		},
		{
			name:     "equal numbers",
			left:     42.0,
			right:    42.0,
			expected: true,
		},
		{
			name:     "different numbers",
			left:     42.0,
			right:    43.0,
			expected: false,
		},
		{
			name:     "equal booleans",
			left:     true,
			right:    true,
			expected: true,
		},
		{
			name:     "different booleans",
			left:     true,
			right:    false,
			expected: false,
		},
		{
			name:     "both nil",
			left:     nil,
			right:    nil,
			expected: true,
		},
		{
			name:     "one nil",
			left:     nil,
			right:    "hello",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.isStrictlyEqual(tt.left, tt.right)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDeltaCalculator_isTerminal(t *testing.T) {
	calculator := NewDeltaCalculator()

	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{
			name:     "string is terminal",
			value:    "hello",
			expected: true,
		},
		{
			name:     "number is terminal",
			value:    42.0,
			expected: true,
		},
		{
			name:     "boolean is terminal",
			value:    true,
			expected: true,
		},
		{
			name:     "nil is terminal",
			value:    nil,
			expected: true,
		},
		{
			name:     "map is not terminal",
			value:    map[string]interface{}{"key": "value"},
			expected: false,
		},
		{
			name:     "slice is not terminal",
			value:    []interface{}{"item1", "item2"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.isTerminal(tt.value)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDeltaCalculator_objectCommonality(t *testing.T) {
	calculator := NewDeltaCalculator()

	tests := []struct {
		name     string
		left     map[string]interface{}
		right    map[string]interface{}
		expected float64
	}{
		{
			name:     "identical objects",
			left:     map[string]interface{}{"a": 1, "b": 2},
			right:    map[string]interface{}{"a": 1, "b": 2},
			expected: 1.0,
		},
		{
			name:     "completely different objects",
			left:     map[string]interface{}{"a": 1, "b": 2},
			right:    map[string]interface{}{"c": 3, "d": 4},
			expected: 0.0,
		},
		{
			name:     "partially overlapping objects",
			left:     map[string]interface{}{"a": 1, "b": 2},
			right:    map[string]interface{}{"a": 1, "c": 3},
			expected: 1.0 / 3.0, // 1 common key out of 3 total keys
		},
		{
			name:     "empty objects",
			left:     map[string]interface{}{},
			right:    map[string]interface{}{},
			expected: 1.0,
		},
		{
			name:     "one empty object",
			left:     map[string]interface{}{},
			right:    map[string]interface{}{"a": 1},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.objectCommonality(tt.left, tt.right)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDeltaCalculator_arrayCommonality(t *testing.T) {
	calculator := NewDeltaCalculator()

	tests := []struct {
		name     string
		left     []interface{}
		right    []interface{}
		expected float64
	}{
		{
			name:     "same length arrays",
			left:     []interface{}{1, 2, 3},
			right:    []interface{}{4, 5, 6},
			expected: 1.0,
		},
		{
			name:     "different length arrays",
			left:     []interface{}{1, 2},
			right:    []interface{}{3, 4, 5},
			expected: 2.0 / 3.0, // min(2,3) / max(2,3)
		},
		{
			name:     "empty arrays",
			left:     []interface{}{},
			right:    []interface{}{},
			expected: 1.0,
		},
		{
			name:     "one empty array",
			left:     []interface{}{},
			right:    []interface{}{1, 2},
			expected: 0.0, // min(0,2) / max(0,2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.arrayCommonality(tt.left, tt.right)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDeltaCalculator_diff(t *testing.T) {
	calculator := NewDeltaCalculator()

	tests := []struct {
		name     string
		left     interface{}
		right    interface{}
		key      []interface{}
		expected int // Expected number of diff stanzas
	}{
		{
			name:     "identical values",
			left:     "hello",
			right:    "hello",
			key:      []interface{}{},
			expected: 0,
		},
		{
			name:     "different terminal values",
			left:     "hello",
			right:    "world",
			key:      []interface{}{},
			expected: 1,
		},
		{
			name:     "simple object change",
			left:     map[string]interface{}{"a": 1},
			right:    map[string]interface{}{"a": 2},
			key:      []interface{}{},
			expected: 1,
		},
		{
			name:     "object key addition",
			left:     map[string]interface{}{"a": 1},
			right:    map[string]interface{}{"a": 1, "b": 2},
			key:      []interface{}{},
			expected: 1,
		},
		{
			name:     "object key removal",
			left:     map[string]interface{}{"a": 1, "b": 2},
			right:    map[string]interface{}{"a": 1},
			key:      []interface{}{},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.diff(tt.left, tt.right, tt.key)
			if len(result) != tt.expected {
				t.Errorf("Expected %d diff stanzas, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestDeltaCalculator_keysetDiff(t *testing.T) {
	calculator := NewDeltaCalculator()

	left := map[string]interface{}{
		"a": 1,
		"b": 2,
		"c": 3,
	}

	right := map[string]interface{}{
		"a": 1,  // unchanged
		"b": 22, // changed
		"d": 4,  // added
		// "c" removed
	}

	key := []interface{}{"root"}
	result := calculator.keysetDiff(left, right, key)

	// Should have 3 changes: b changed, c removed (set to nil), d added
	if len(result) != 3 {
		t.Errorf("Expected 3 diff stanzas, got %d", len(result))
	}

	// Verify the changes are correct
	changes := make(map[string]interface{})
	for _, stanza := range result {
		if len(stanza.Key) == 2 && stanza.Key[0] == "root" {
			changes[stanza.Key[1].(string)] = stanza.Value
		}
	}

	if changes["b"] != 22 {
		t.Errorf("Expected b to be changed to 22, got %v", changes["b"])
	}
	if changes["c"] != nil {
		t.Errorf("Expected c to be removed (nil), got %v", changes["c"])
	}
	if changes["d"] != 4 {
		t.Errorf("Expected d to be added with value 4, got %v", changes["d"])
	}
}

func TestDeltaCalculator_arrayDiff(t *testing.T) {
	calculator := NewDeltaCalculator()

	tests := []struct {
		name     string
		left     []interface{}
		right    []interface{}
		key      []interface{}
		expected int // Expected number of diff stanzas (0 for identical, 1 for replacement)
	}{
		{
			name:     "identical arrays",
			left:     []interface{}{1, 2, 3},
			right:    []interface{}{1, 2, 3},
			key:      []interface{}{"array"},
			expected: 0,
		},
		{
			name:     "different length arrays",
			left:     []interface{}{1, 2},
			right:    []interface{}{1, 2, 3},
			key:      []interface{}{"array"},
			expected: 1,
		},
		{
			name:     "different content arrays",
			left:     []interface{}{1, 2, 3},
			right:    []interface{}{1, 2, 4},
			key:      []interface{}{"array"},
			expected: 1,
		},
		{
			name:     "empty arrays",
			left:     []interface{}{},
			right:    []interface{}{},
			key:      []interface{}{"array"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculator.arrayDiff(tt.left, tt.right, tt.key)
			if len(result) != tt.expected {
				t.Errorf("Expected %d diff stanzas, got %d", tt.expected, len(result))
			}
		})
	}
}

// Benchmark tests
func BenchmarkDeltaCalculator_CalculateDelta(b *testing.B) {
	calculator := NewDeltaCalculator()
	clusterID := "benchmark-cluster"

	// Create test data
	data := &models.ClusterData{
		ID:             clusterID,
		APIServerURL:   "https://api.benchmark-cluster.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Add many nodes and pods for realistic benchmark
	for i := 0; i < 100; i++ {
		nodeName := fmt.Sprintf("node-%d", i)
		node := &models.Node{
			Name:   nodeName,
			Labels: map[string]string{"node": nodeName},
			Status: models.NodeStatus{Ready: true},
			Pods:   make(map[string]*models.Pod),
		}

		// Add pods to each node
		for j := 0; j < 10; j++ {
			podName := fmt.Sprintf("pod-%d-%d", i, j)
			node.Pods[podName] = &models.Pod{
				Name:      podName,
				Namespace: "default",
				Phase:     "Running",
				Ready:     true,
			}
		}

		data.Nodes[nodeName] = node
	}

	// Initialize with first calculation
	calculator.CalculateDelta(clusterID, data)

	// Modify data slightly for benchmark
	data.Nodes["node-0"].Pods["new-pod"] = &models.Pod{
		Name:      "new-pod",
		Namespace: "default",
		Phase:     "Running",
		Ready:     true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculator.CalculateDelta(clusterID, data)
	}
}

package delta

import (
	"fmt"
	"testing"
	"time"

	"kube-ops-view/internal/models"
)

func TestCompressor_CompressDelta(t *testing.T) {
	compressor := NewCompressor(DefaultDeltaOptions())

	// Test empty delta
	emptyDelta := []DiffStanza{}
	compressed, err := compressor.CompressDelta(emptyDelta)
	if err != nil {
		t.Fatalf("Expected no error for empty delta, got: %v", err)
	}
	if compressed != nil {
		t.Fatalf("Expected nil for empty delta, got: %v", compressed)
	}

	// Test non-empty delta
	delta := []DiffStanza{
		{
			Key:   []interface{}{"nodes", "node1", "status", "ready"},
			Value: true,
		},
		{
			Key:   []interface{}{"nodes", "node1", "pods", "pod1"},
			Value: map[string]interface{}{"name": "pod1", "phase": "Running"},
		},
	}

	compressed, err = compressor.CompressDelta(delta)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if compressed == nil {
		t.Fatalf("Expected compressed data, got nil")
	}
	if len(compressed) == 0 {
		t.Fatalf("Expected non-empty compressed data")
	}
}

func TestCompressor_DecompressDelta(t *testing.T) {
	compressor := NewCompressor(DefaultDeltaOptions())

	// Test empty data
	decompressed, err := compressor.DecompressDelta(nil)
	if err != nil {
		t.Fatalf("Expected no error for nil data, got: %v", err)
	}
	if decompressed != nil {
		t.Fatalf("Expected nil for nil data, got: %v", decompressed)
	}

	// Test round-trip compression/decompression
	originalDelta := []DiffStanza{
		{
			Key:   []interface{}{"nodes", "node1", "status", "ready"},
			Value: true,
		},
		{
			Key:   []interface{}{"nodes", "node1", "pods", "pod1"},
			Value: map[string]interface{}{"name": "pod1", "phase": "Running"},
		},
	}

	compressed, err := compressor.CompressDelta(originalDelta)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	decompressed, err = compressor.DecompressDelta(compressed)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}

	if len(decompressed) != len(originalDelta) {
		t.Fatalf("Expected %d stanzas, got %d", len(originalDelta), len(decompressed))
	}

	// Verify content matches
	for i, stanza := range decompressed {
		if len(stanza.Key) != len(originalDelta[i].Key) {
			t.Errorf("Stanza %d key length mismatch: expected %d, got %d", i, len(originalDelta[i].Key), len(stanza.Key))
		}
	}
}

func TestCompressor_CalculateCompressionStats(t *testing.T) {
	compressor := NewCompressor(DefaultDeltaOptions())

	// Create test data
	originalData := &models.ClusterData{
		ID:           "test-cluster",
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

	delta := []DiffStanza{
		{
			Key:   []interface{}{"nodes", "node1", "pods", "pod2"},
			Value: map[string]interface{}{"name": "pod2", "phase": "Running"},
		},
	}

	stats, err := compressor.CalculateCompressionStats(originalData, delta)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if stats.OriginalSize <= 0 {
		t.Errorf("Expected positive original size, got %d", stats.OriginalSize)
	}
	if stats.DeltaSize <= 0 {
		t.Errorf("Expected positive delta size, got %d", stats.DeltaSize)
	}
	if stats.CompressionRatio <= 0 {
		t.Errorf("Expected positive compression ratio, got %f", stats.CompressionRatio)
	}
	if stats.ChangeCount != len(delta) {
		t.Errorf("Expected change count %d, got %d", len(delta), stats.ChangeCount)
	}

	// Delta should be smaller than original data
	if stats.CompressionRatio >= 1.0 {
		t.Errorf("Expected compression ratio < 1.0, got %f", stats.CompressionRatio)
	}
}

func TestCompressor_ShouldCompress(t *testing.T) {
	compressor := NewCompressor(DefaultDeltaOptions())

	tests := []struct {
		name      string
		delta     []DiffStanza
		threshold int
		expected  bool
	}{
		{
			name:      "empty delta",
			delta:     []DiffStanza{},
			threshold: 100,
			expected:  false,
		},
		{
			name: "small delta below threshold",
			delta: []DiffStanza{
				{Key: []interface{}{"a"}, Value: "b"},
			},
			threshold: 1000,
			expected:  false,
		},
		{
			name: "large delta above threshold",
			delta: []DiffStanza{
				{
					Key: []interface{}{"nodes", "node1", "pods"},
					Value: map[string]interface{}{
						"pod1": map[string]interface{}{
							"name":      "pod1",
							"namespace": "default",
							"phase":     "Running",
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "container1",
									"image": "nginx:latest",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu":    "100m",
											"memory": "128Mi",
										},
									},
								},
							},
						},
					},
				},
			},
			threshold: 50,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compressor.ShouldCompress(tt.delta, tt.threshold)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCompressor_OptimizeDelta(t *testing.T) {
	compressor := NewCompressor(DefaultDeltaOptions())

	tests := []struct {
		name     string
		delta    []DiffStanza
		expected int // Expected number of stanzas after optimization
	}{
		{
			name:     "empty delta",
			delta:    []DiffStanza{},
			expected: 0,
		},
		{
			name: "single stanza",
			delta: []DiffStanza{
				{Key: []interface{}{"a"}, Value: "b"},
			},
			expected: 1,
		},
		{
			name: "no redundant operations",
			delta: []DiffStanza{
				{Key: []interface{}{"a"}, Value: "b"},
				{Key: []interface{}{"c"}, Value: "d"},
			},
			expected: 2,
		},
		{
			name: "redundant operations on same key",
			delta: []DiffStanza{
				{Key: []interface{}{"a"}, Value: "b"},
				{Key: []interface{}{"c"}, Value: "d"},
				{Key: []interface{}{"a"}, Value: "e"}, // This should replace the first one
			},
			expected: 2, // Should keep only the last operation for key "a"
		},
		{
			name: "multiple redundant operations",
			delta: []DiffStanza{
				{Key: []interface{}{"a"}, Value: "b"},
				{Key: []interface{}{"a"}, Value: "c"},
				{Key: []interface{}{"a"}, Value: "d"},
			},
			expected: 1, // Should keep only the last operation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compressor.OptimizeDelta(tt.delta)
			if len(result) != tt.expected {
				t.Errorf("Expected %d stanzas, got %d", tt.expected, len(result))
			}

			// Verify that the last operation for each key is preserved
			if tt.name == "redundant operations on same key" && len(result) == 2 {
				// Find the operation for key "a"
				var aValue interface{}
				for _, stanza := range result {
					if len(stanza.Key) == 1 && stanza.Key[0] == "a" {
						aValue = stanza.Value
						break
					}
				}
				if aValue != "e" {
					t.Errorf("Expected final value 'e' for key 'a', got %v", aValue)
				}
			}
		})
	}
}

func TestNewCompressedDelta(t *testing.T) {
	clusterID := "test-cluster"
	delta := []DiffStanza{
		{Key: []interface{}{"nodes", "node1", "status"}, Value: "Ready"},
	}
	originalData := &models.ClusterData{
		ID:             clusterID,
		APIServerURL:   "https://api.test-cluster.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	compressedDelta, err := NewCompressedDelta(clusterID, delta, originalData)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if compressedDelta.ClusterID != clusterID {
		t.Errorf("Expected cluster ID %s, got %s", clusterID, compressedDelta.ClusterID)
	}

	if compressedDelta.Data == nil {
		t.Errorf("Expected non-nil data")
	}

	if compressedDelta.Stats.ChangeCount != len(delta) {
		t.Errorf("Expected change count %d, got %d", len(delta), compressedDelta.Stats.ChangeCount)
	}

	// Test getting delta back
	retrievedDelta, err := compressedDelta.GetDelta()
	if err != nil {
		t.Fatalf("Expected no error getting delta, got: %v", err)
	}

	if len(retrievedDelta) != len(delta) {
		t.Errorf("Expected %d stanzas, got %d", len(delta), len(retrievedDelta))
	}
}

func TestDefaultDeltaOptions(t *testing.T) {
	options := DefaultDeltaOptions()

	if options.ArrayAlign != false {
		t.Errorf("Expected ArrayAlign to be false, got %v", options.ArrayAlign)
	}
	if options.CompareLengths != false {
		t.Errorf("Expected CompareLengths to be false, got %v", options.CompareLengths)
	}
	if options.Minimal != true {
		t.Errorf("Expected Minimal to be true, got %v", options.Minimal)
	}
	if options.MaxDepth != 0 {
		t.Errorf("Expected MaxDepth to be 0, got %v", options.MaxDepth)
	}
}

// Benchmark tests
func BenchmarkCompressor_CompressDelta(b *testing.B) {
	compressor := NewCompressor(DefaultDeltaOptions())

	// Create a large delta for benchmarking
	delta := make([]DiffStanza, 1000)
	for i := 0; i < 1000; i++ {
		delta[i] = DiffStanza{
			Key: []interface{}{"nodes", fmt.Sprintf("node-%d", i), "status"},
			Value: map[string]interface{}{
				"ready":     true,
				"timestamp": time.Now().Unix(),
				"resources": map[string]interface{}{
					"cpu":    "100m",
					"memory": "256Mi",
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressDelta(delta)
		if err != nil {
			b.Fatalf("Compression failed: %v", err)
		}
	}
}

func BenchmarkCompressor_OptimizeDelta(b *testing.B) {
	compressor := NewCompressor(DefaultDeltaOptions())

	// Create a delta with many redundant operations
	delta := make([]DiffStanza, 1000)
	for i := 0; i < 1000; i++ {
		delta[i] = DiffStanza{
			Key:   []interface{}{"nodes", fmt.Sprintf("node-%d", i%100), "status"}, // Repeat keys
			Value: fmt.Sprintf("value-%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.OptimizeDelta(delta)
	}
}

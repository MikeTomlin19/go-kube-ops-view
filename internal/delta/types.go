package delta

// DiffStanza represents a single change operation in a delta
// This format is compatible with the json-delta library format
type DiffStanza struct {
	Key   []interface{} `json:"key"`   // Path to the changed value (e.g., ["nodes", "node1", "status"])
	Value interface{}   `json:"value"` // New value (nil for deletions)
}

// DeltaResult represents the result of a delta calculation
type DeltaResult struct {
	ClusterID  string       `json:"cluster_id"`
	Delta      []DiffStanza `json:"delta"`
	HasChanges bool         `json:"has_changes"`
}

// CompressionStats provides statistics about delta compression
type CompressionStats struct {
	OriginalSize     int     `json:"original_size"`     // Size of full cluster data in bytes
	DeltaSize        int     `json:"delta_size"`        // Size of delta in bytes
	CompressionRatio float64 `json:"compression_ratio"` // Ratio of delta size to original size
	ChangeCount      int     `json:"change_count"`      // Number of changes in the delta
}

// DeltaOptions configures delta calculation behavior
type DeltaOptions struct {
	// ArrayAlign determines whether to align arrays for better diffs
	// Setting to false improves performance but may produce larger diffs
	ArrayAlign bool `json:"array_align"`

	// CompareLengths determines whether to compare array lengths
	// Setting to false improves performance for large arrays
	CompareLengths bool `json:"compare_lengths"`

	// Minimal determines whether to optimize for minimal delta size
	// Setting to true uses more CPU but produces smaller deltas
	Minimal bool `json:"minimal"`

	// MaxDepth limits the depth of recursive comparison
	// 0 means no limit
	MaxDepth int `json:"max_depth"`
}

// DefaultDeltaOptions returns the default options compatible with the Python version
func DefaultDeltaOptions() DeltaOptions {
	return DeltaOptions{
		ArrayAlign:     false, // Matches Python version for performance
		CompareLengths: false, // Matches Python version for performance
		Minimal:        true,  // Optimize for smaller deltas
		MaxDepth:       0,     // No depth limit
	}
}

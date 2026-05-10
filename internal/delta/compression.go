package delta

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
)

// Compressor provides delta compression functionality
type Compressor struct {
	options DeltaOptions
}

// NewCompressor creates a new delta compressor with the given options
func NewCompressor(options DeltaOptions) *Compressor {
	return &Compressor{
		options: options,
	}
}

// CompressDelta compresses a delta using gzip compression
func (c *Compressor) CompressDelta(delta []DiffStanza) ([]byte, error) {
	if len(delta) == 0 {
		return nil, nil
	}

	// Marshal delta to JSON
	jsonData, err := json.Marshal(delta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal delta: %w", err)
	}

	// Compress using gzip
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)

	_, err = gzipWriter.Write(jsonData)
	if err != nil {
		gzipWriter.Close()
		return nil, fmt.Errorf("failed to compress delta: %w", err)
	}

	err = gzipWriter.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// DecompressDelta decompresses a gzip-compressed delta
func (c *Compressor) DecompressDelta(compressedData []byte) ([]DiffStanza, error) {
	if len(compressedData) == 0 {
		return nil, nil
	}

	// Decompress using gzip
	reader, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()

	decompressedData, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress data: %w", err)
	}

	// Unmarshal JSON
	var delta []DiffStanza
	err = json.Unmarshal(decompressedData, &delta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal delta: %w", err)
	}

	return delta, nil
}

// CalculateCompressionStats calculates compression statistics for a delta
func (c *Compressor) CalculateCompressionStats(originalData interface{}, delta []DiffStanza) (*CompressionStats, error) {
	// Calculate original data size
	originalJSON, err := json.Marshal(originalData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal original data: %w", err)
	}

	// Calculate delta size
	deltaJSON, err := json.Marshal(delta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal delta: %w", err)
	}

	originalSize := len(originalJSON)
	deltaSize := len(deltaJSON)

	var compressionRatio float64
	if originalSize > 0 {
		compressionRatio = float64(deltaSize) / float64(originalSize)
	}

	return &CompressionStats{
		OriginalSize:     originalSize,
		DeltaSize:        deltaSize,
		CompressionRatio: compressionRatio,
		ChangeCount:      len(delta),
	}, nil
}

// ShouldCompress determines whether compression would be beneficial
func (c *Compressor) ShouldCompress(delta []DiffStanza, threshold int) bool {
	if len(delta) == 0 {
		return false
	}

	// Estimate delta size
	deltaJSON, err := json.Marshal(delta)
	if err != nil {
		return false
	}

	// Compress if delta is larger than threshold
	return len(deltaJSON) > threshold
}

// OptimizeDelta optimizes a delta by removing redundant operations
func (c *Compressor) OptimizeDelta(delta []DiffStanza) []DiffStanza {
	if len(delta) <= 1 {
		return delta
	}

	// Create a map to track the latest operation for each key path
	keyMap := make(map[string]int)
	optimized := make([]DiffStanza, 0, len(delta))

	for _, stanza := range delta {
		keyStr := c.keyToString(stanza.Key)

		// Check if this key path has been seen before
		if prevIndex, exists := keyMap[keyStr]; exists {
			// Remove the previous operation for this key path
			optimized = c.removeIndex(optimized, prevIndex)
			// Update indices in keyMap
			for k, v := range keyMap {
				if v > prevIndex {
					keyMap[k] = v - 1
				}
			}
		}

		// Add current operation
		optimized = append(optimized, stanza)
		keyMap[keyStr] = len(optimized) - 1
	}

	return optimized
}

// keyToString converts a key path to a string for comparison
func (c *Compressor) keyToString(key []interface{}) string {
	keyJSON, _ := json.Marshal(key)
	return string(keyJSON)
}

// removeIndex removes an element at the specified index from a slice
func (c *Compressor) removeIndex(slice []DiffStanza, index int) []DiffStanza {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return append(slice[:index], slice[index+1:]...)
}

// CompressedDelta represents a compressed delta with metadata
type CompressedDelta struct {
	Data       []byte           `json:"data"`       // Compressed delta data
	Stats      CompressionStats `json:"stats"`      // Compression statistics
	Compressed bool             `json:"compressed"` // Whether data is compressed
	ClusterID  string           `json:"cluster_id"` // Associated cluster ID
	Timestamp  int64            `json:"timestamp"`  // Unix timestamp
}

// NewCompressedDelta creates a new compressed delta
func NewCompressedDelta(clusterID string, delta []DiffStanza, originalData interface{}) (*CompressedDelta, error) {
	compressor := NewCompressor(DefaultDeltaOptions())

	// Calculate stats
	stats, err := compressor.CalculateCompressionStats(originalData, delta)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate compression stats: %w", err)
	}

	// Determine if compression is beneficial (compress if delta > 1KB)
	shouldCompress := compressor.ShouldCompress(delta, 1024)

	var data []byte
	if shouldCompress {
		// Compress the delta
		data, err = compressor.CompressDelta(delta)
		if err != nil {
			return nil, fmt.Errorf("failed to compress delta: %w", err)
		}
	} else {
		// Store as plain JSON
		data, err = json.Marshal(delta)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal delta: %w", err)
		}
	}

	return &CompressedDelta{
		Data:       data,
		Stats:      *stats,
		Compressed: shouldCompress,
		ClusterID:  clusterID,
		Timestamp:  0, // Will be set by caller
	}, nil
}

// GetDelta extracts the delta from compressed data
func (cd *CompressedDelta) GetDelta() ([]DiffStanza, error) {
	if cd.Compressed {
		compressor := NewCompressor(DefaultDeltaOptions())
		return compressor.DecompressDelta(cd.Data)
	}

	var delta []DiffStanza
	err := json.Unmarshal(cd.Data, &delta)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal delta: %w", err)
	}

	return delta, nil
}

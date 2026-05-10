package cluster

import "time"

// MetricsRecorder defines the interface for recording cluster-related metrics
type MetricsRecorder interface {
	// RecordClusterQuery records metrics for a cluster query operation
	RecordClusterQuery(clusterID, operation string, duration time.Duration, err error)

	// UpdateClusterStats updates cluster statistics metrics
	UpdateClusterStats(clusterID string, nodes, totalPods, runningPods, pendingPods, errorPods int)
}

// NoOpMetricsRecorder is a no-op implementation of MetricsRecorder
type NoOpMetricsRecorder struct{}

func (n *NoOpMetricsRecorder) RecordClusterQuery(clusterID, operation string, duration time.Duration, err error) {
	// No-op
}

func (n *NoOpMetricsRecorder) UpdateClusterStats(clusterID string, nodes, totalPods, runningPods, pendingPods, errorPods int) {
	// No-op
}

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	Registry *prometheus.Registry

	// HTTP metrics
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPActiveConnections prometheus.Gauge

	// Cluster metrics
	ClustersTotal        prometheus.Gauge
	ClustersAvailable    prometheus.Gauge
	ClusterQueryDuration *prometheus.HistogramVec
	ClusterQueryErrors   *prometheus.CounterVec
	ClusterLastQueryTime *prometheus.GaugeVec
	ClusterNodesTotal    *prometheus.GaugeVec
	ClusterPodsTotal     *prometheus.GaugeVec
	ClusterPodsRunning   *prometheus.GaugeVec
	ClusterPodsPending   *prometheus.GaugeVec
	ClusterPodsError     *prometheus.GaugeVec

	// SSE metrics
	SSEActiveConnections prometheus.Gauge
	SSEEventsPublished   *prometheus.CounterVec
	SSEConnectionErrors  prometheus.Counter

	// Storage metrics
	StorageOperations     *prometheus.CounterVec
	StorageOperationTime  *prometheus.HistogramVec
	StorageConnectionPool prometheus.Gauge

	// Authentication metrics
	AuthRequests      *prometheus.CounterVec
	AuthTokensActive  prometheus.Gauge
	AuthSessionsTotal prometheus.Gauge

	// Application metrics
	ApplicationInfo      *prometheus.GaugeVec
	ApplicationUptime    prometheus.Gauge
	ApplicationStartTime prometheus.Gauge

	// Resource usage metrics
	GoRoutinesActive prometheus.Gauge
	MemoryUsage      prometheus.Gauge
	CPUUsage         prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	return NewMetricsWithRegistry(prometheus.NewRegistry())
}

// NewMetricsWithRegistry creates and registers all Prometheus metrics in the
// provided registry.
func NewMetricsWithRegistry(registry *prometheus.Registry) *Metrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	factory := promauto.With(registry)

	return &Metrics{
		Registry: registry,

		// HTTP metrics
		HTTPRequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kube_ops_view_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "kube_ops_view_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPActiveConnections: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_http_active_connections",
				Help: "Number of active HTTP connections",
			},
		),

		// Cluster metrics
		ClustersTotal: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_clusters_total",
				Help: "Total number of configured clusters",
			},
		),
		ClustersAvailable: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_clusters_available",
				Help: "Number of available clusters",
			},
		),
		ClusterQueryDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "kube_ops_view_cluster_query_duration_seconds",
				Help:    "Time spent querying cluster data",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"cluster_id", "operation"},
		),
		ClusterQueryErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kube_ops_view_cluster_query_errors_total",
				Help: "Total number of cluster query errors",
			},
			[]string{"cluster_id", "operation", "error_type"},
		),
		ClusterLastQueryTime: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cluster_last_query_timestamp",
				Help: "Timestamp of last successful cluster query",
			},
			[]string{"cluster_id"},
		),
		ClusterNodesTotal: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cluster_nodes_total",
				Help: "Total number of nodes in cluster",
			},
			[]string{"cluster_id"},
		),
		ClusterPodsTotal: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cluster_pods_total",
				Help: "Total number of pods in cluster",
			},
			[]string{"cluster_id"},
		),
		ClusterPodsRunning: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cluster_pods_running",
				Help: "Number of running pods in cluster",
			},
			[]string{"cluster_id"},
		),
		ClusterPodsPending: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cluster_pods_pending",
				Help: "Number of pending pods in cluster",
			},
			[]string{"cluster_id"},
		),
		ClusterPodsError: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cluster_pods_error",
				Help: "Number of pods in error state in cluster",
			},
			[]string{"cluster_id"},
		),

		// SSE metrics
		SSEActiveConnections: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_sse_active_connections",
				Help: "Number of active SSE connections",
			},
		),
		SSEEventsPublished: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kube_ops_view_sse_events_published_total",
				Help: "Total number of SSE events published",
			},
			[]string{"event_type"},
		),
		SSEConnectionErrors: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "kube_ops_view_sse_connection_errors_total",
				Help: "Total number of SSE connection errors",
			},
		),

		// Storage metrics
		StorageOperations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kube_ops_view_storage_operations_total",
				Help: "Total number of storage operations",
			},
			[]string{"operation", "status"},
		),
		StorageOperationTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "kube_ops_view_storage_operation_duration_seconds",
				Help:    "Time spent on storage operations",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
			},
			[]string{"operation"},
		),
		StorageConnectionPool: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_storage_connection_pool_size",
				Help: "Current size of storage connection pool",
			},
		),

		// Authentication metrics
		AuthRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "kube_ops_view_auth_requests_total",
				Help: "Total number of authentication requests",
			},
			[]string{"provider", "status"},
		),
		AuthTokensActive: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_auth_tokens_active",
				Help: "Number of active authentication tokens",
			},
		),
		AuthSessionsTotal: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_auth_sessions_total",
				Help: "Total number of active authentication sessions",
			},
		),

		// Application metrics
		ApplicationInfo: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_application_info",
				Help: "Application information",
			},
			[]string{"version", "commit", "build_date"},
		),
		ApplicationUptime: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_application_uptime_seconds",
				Help: "Application uptime in seconds",
			},
		),
		ApplicationStartTime: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_application_start_time_timestamp",
				Help: "Application start time as Unix timestamp",
			},
		),

		// Resource usage metrics
		GoRoutinesActive: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_goroutines_active",
				Help: "Number of active goroutines",
			},
		),
		MemoryUsage: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),
		CPUUsage: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "kube_ops_view_cpu_usage_percent",
				Help: "Current CPU usage percentage",
			},
		),
	}
}

// RecordHTTPRequest records metrics for an HTTP request
func (m *Metrics) RecordHTTPRequest(method, path, statusCode string, duration time.Duration) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, statusCode).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordClusterQuery records metrics for a cluster query operation
func (m *Metrics) RecordClusterQuery(clusterID, operation string, duration time.Duration, err error) {
	m.ClusterQueryDuration.WithLabelValues(clusterID, operation).Observe(duration.Seconds())

	if err != nil {
		errorType := "unknown"
		// Categorize error types for better monitoring
		switch {
		case isConnectionError(err):
			errorType = "connection"
		case isTimeoutError(err):
			errorType = "timeout"
		case isAuthError(err):
			errorType = "authentication"
		case isNotFoundError(err):
			errorType = "not_found"
		}
		m.ClusterQueryErrors.WithLabelValues(clusterID, operation, errorType).Inc()
	} else {
		m.ClusterLastQueryTime.WithLabelValues(clusterID).SetToCurrentTime()
	}
}

// UpdateClusterStats updates cluster statistics metrics
func (m *Metrics) UpdateClusterStats(clusterID string, nodes, totalPods, runningPods, pendingPods, errorPods int) {
	m.ClusterNodesTotal.WithLabelValues(clusterID).Set(float64(nodes))
	m.ClusterPodsTotal.WithLabelValues(clusterID).Set(float64(totalPods))
	m.ClusterPodsRunning.WithLabelValues(clusterID).Set(float64(runningPods))
	m.ClusterPodsPending.WithLabelValues(clusterID).Set(float64(pendingPods))
	m.ClusterPodsError.WithLabelValues(clusterID).Set(float64(errorPods))
}

// RecordSSEEvent records metrics for SSE events
func (m *Metrics) RecordSSEEvent(eventType string) {
	m.SSEEventsPublished.WithLabelValues(eventType).Inc()
}

// RecordStorageOperation records metrics for storage operations
func (m *Metrics) RecordStorageOperation(operation string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	m.StorageOperations.WithLabelValues(operation, status).Inc()
	m.StorageOperationTime.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordAuthRequest records metrics for authentication requests
func (m *Metrics) RecordAuthRequest(provider, status string) {
	m.AuthRequests.WithLabelValues(provider, status).Inc()
}

// SetApplicationInfo sets application information metrics
func (m *Metrics) SetApplicationInfo(version, commit, buildDate string) {
	m.ApplicationInfo.WithLabelValues(version, commit, buildDate).Set(1)
	m.ApplicationStartTime.SetToCurrentTime()
}

// Helper functions to categorize errors
func isConnectionError(err error) bool {
	// Check for common connection error patterns
	errStr := err.Error()
	return contains(errStr, "connection refused") ||
		contains(errStr, "no such host") ||
		contains(errStr, "network unreachable")
}

func isTimeoutError(err error) bool {
	errStr := err.Error()
	return contains(errStr, "timeout") ||
		contains(errStr, "deadline exceeded")
}

func isAuthError(err error) bool {
	errStr := err.Error()
	return contains(errStr, "unauthorized") ||
		contains(errStr, "forbidden") ||
		contains(errStr, "authentication")
}

func isNotFoundError(err error) bool {
	errStr := err.Error()
	return contains(errStr, "not found") ||
		contains(errStr, "404")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

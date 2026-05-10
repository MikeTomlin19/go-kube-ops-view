package metrics

import (
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	// Create a new registry for testing to avoid conflicts
	registry := prometheus.NewRegistry()

	// Create metrics with custom registry
	metrics := &Metrics{
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "test_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		ClusterQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "test_cluster_query_duration_seconds",
				Help:    "Time spent querying cluster data",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
			},
			[]string{"cluster_id", "operation"},
		),
	}

	// Register metrics
	registry.MustRegister(metrics.HTTPRequestsTotal)
	registry.MustRegister(metrics.ClusterQueryDuration)

	assert.NotNil(t, metrics.HTTPRequestsTotal)
	assert.NotNil(t, metrics.ClusterQueryDuration)
}

func TestRecordHTTPRequest(t *testing.T) {
	registry := prometheus.NewRegistry()

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	registry.MustRegister(counter)
	registry.MustRegister(histogram)

	metrics := &Metrics{
		HTTPRequestsTotal:   counter,
		HTTPRequestDuration: histogram,
	}

	// Record a request
	metrics.RecordHTTPRequest("GET", "/api/clusters", "200", 100*time.Millisecond)

	// Check counter
	counterValue := testutil.ToFloat64(counter.WithLabelValues("GET", "/api/clusters", "200"))
	assert.Equal(t, float64(1), counterValue)

	assert.Equal(t, uint64(1), histogramSampleCount(t, registry, "test_http_request_duration_seconds", map[string]string{
		"method": "GET",
		"path":   "/api/clusters",
	}))
}

func TestRecordClusterQuery(t *testing.T) {
	registry := prometheus.NewRegistry()

	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_cluster_query_duration_seconds",
			Help:    "Time spent querying cluster data",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		},
		[]string{"cluster_id", "operation"},
	)

	errorCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_cluster_query_errors_total",
			Help: "Total number of cluster query errors",
		},
		[]string{"cluster_id", "operation", "error_type"},
	)

	lastQueryTime := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cluster_last_query_timestamp",
			Help: "Timestamp of last successful cluster query",
		},
		[]string{"cluster_id"},
	)

	registry.MustRegister(histogram)
	registry.MustRegister(errorCounter)
	registry.MustRegister(lastQueryTime)

	metrics := &Metrics{
		ClusterQueryDuration: histogram,
		ClusterQueryErrors:   errorCounter,
		ClusterLastQueryTime: lastQueryTime,
	}

	t.Run("successful query", func(t *testing.T) {
		metrics.RecordClusterQuery("cluster1", "nodes", 200*time.Millisecond, nil)

		assert.Equal(t, uint64(1), histogramSampleCount(t, registry, "test_cluster_query_duration_seconds", map[string]string{
			"cluster_id": "cluster1",
			"operation":  "nodes",
		}))

		// Check that no error was recorded
		errorValue := testutil.ToFloat64(errorCounter.WithLabelValues("cluster1", "nodes", "connection"))
		assert.Equal(t, float64(0), errorValue)
	})

	t.Run("failed query", func(t *testing.T) {
		err := errors.New("connection refused")
		metrics.RecordClusterQuery("cluster1", "pods", 500*time.Millisecond, err)

		// Check that error was recorded
		errorValue := testutil.ToFloat64(errorCounter.WithLabelValues("cluster1", "pods", "connection"))
		assert.Equal(t, float64(1), errorValue)
	})
}

func histogramSampleCount(t *testing.T, registry *prometheus.Registry, name string, labels map[string]string) uint64 {
	t.Helper()

	families, err := registry.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != name {
			continue
		}

		for _, metric := range family.GetMetric() {
			if metricLabelsMatch(metric.GetLabel(), labels) {
				require.NotNil(t, metric.GetHistogram())
				return metric.GetHistogram().GetSampleCount()
			}
		}
	}

	t.Fatalf("histogram %q with labels %v not found", name, labels)
	return 0
}

func metricLabelsMatch(labelPairs []*dto.LabelPair, labels map[string]string) bool {
	actual := make(map[string]string, len(labelPairs))
	for _, label := range labelPairs {
		actual[label.GetName()] = label.GetValue()
	}

	for name, expected := range labels {
		if actual[name] != expected {
			return false
		}
	}

	return true
}

func TestUpdateClusterStats(t *testing.T) {
	registry := prometheus.NewRegistry()

	nodesGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cluster_nodes_total",
			Help: "Total number of nodes in cluster",
		},
		[]string{"cluster_id"},
	)

	podsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cluster_pods_total",
			Help: "Total number of pods in cluster",
		},
		[]string{"cluster_id"},
	)
	runningPodsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cluster_pods_running",
			Help: "Number of running pods in cluster",
		},
		[]string{"cluster_id"},
	)
	pendingPodsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cluster_pods_pending",
			Help: "Number of pending pods in cluster",
		},
		[]string{"cluster_id"},
	)
	errorPodsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_cluster_pods_error",
			Help: "Number of pods in error state in cluster",
		},
		[]string{"cluster_id"},
	)

	registry.MustRegister(nodesGauge)
	registry.MustRegister(podsGauge)
	registry.MustRegister(runningPodsGauge)
	registry.MustRegister(pendingPodsGauge)
	registry.MustRegister(errorPodsGauge)

	metrics := &Metrics{
		ClusterNodesTotal:  nodesGauge,
		ClusterPodsTotal:   podsGauge,
		ClusterPodsRunning: runningPodsGauge,
		ClusterPodsPending: pendingPodsGauge,
		ClusterPodsError:   errorPodsGauge,
	}

	metrics.UpdateClusterStats("cluster1", 3, 50, 45, 3, 2)

	nodesValue := testutil.ToFloat64(nodesGauge.WithLabelValues("cluster1"))
	assert.Equal(t, float64(3), nodesValue)

	podsValue := testutil.ToFloat64(podsGauge.WithLabelValues("cluster1"))
	assert.Equal(t, float64(50), podsValue)
}

func TestErrorCategorization(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "connection error",
			err:      errors.New("connection refused"),
			expected: "connection",
		},
		{
			name:     "timeout error",
			err:      errors.New("context deadline exceeded"),
			expected: "timeout",
		},
		{
			name:     "auth error",
			err:      errors.New("unauthorized access"),
			expected: "authentication",
		},
		{
			name:     "not found error",
			err:      errors.New("resource not found"),
			expected: "not_found",
		},
		{
			name:     "unknown error",
			err:      errors.New("some other error"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var errorType string
			switch {
			case isConnectionError(tt.err):
				errorType = "connection"
			case isTimeoutError(tt.err):
				errorType = "timeout"
			case isAuthError(tt.err):
				errorType = "authentication"
			case isNotFoundError(tt.err):
				errorType = "not_found"
			default:
				errorType = "unknown"
			}

			assert.Equal(t, tt.expected, errorType)
		})
	}
}

func TestRecordStorageOperation(t *testing.T) {
	registry := prometheus.NewRegistry()

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_storage_operations_total",
			Help: "Total number of storage operations",
		},
		[]string{"operation", "status"},
	)

	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "test_storage_operation_duration_seconds",
			Help:    "Time spent on storage operations",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation"},
	)

	registry.MustRegister(counter)
	registry.MustRegister(histogram)

	metrics := &Metrics{
		StorageOperations:    counter,
		StorageOperationTime: histogram,
	}

	t.Run("successful operation", func(t *testing.T) {
		metrics.RecordStorageOperation("get", 10*time.Millisecond, nil)

		successValue := testutil.ToFloat64(counter.WithLabelValues("get", "success"))
		assert.Equal(t, float64(1), successValue)

		errorValue := testutil.ToFloat64(counter.WithLabelValues("get", "error"))
		assert.Equal(t, float64(0), errorValue)
	})

	t.Run("failed operation", func(t *testing.T) {
		metrics.RecordStorageOperation("set", 5*time.Millisecond, errors.New("storage error"))

		errorValue := testutil.ToFloat64(counter.WithLabelValues("set", "error"))
		assert.Equal(t, float64(1), errorValue)
	})
}

func TestRecordAuthRequest(t *testing.T) {
	registry := prometheus.NewRegistry()

	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "test_auth_requests_total",
			Help: "Total number of authentication requests",
		},
		[]string{"provider", "status"},
	)

	registry.MustRegister(counter)

	metrics := &Metrics{
		AuthRequests: counter,
	}

	metrics.RecordAuthRequest("oauth2", "success")
	metrics.RecordAuthRequest("oauth2", "failure")

	successValue := testutil.ToFloat64(counter.WithLabelValues("oauth2", "success"))
	assert.Equal(t, float64(1), successValue)

	failureValue := testutil.ToFloat64(counter.WithLabelValues("oauth2", "failure"))
	assert.Equal(t, float64(1), failureValue)
}

func TestSetApplicationInfo(t *testing.T) {
	registry := prometheus.NewRegistry()

	infoGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "test_application_info",
			Help: "Application information",
		},
		[]string{"version", "commit", "build_date"},
	)

	startTimeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "test_application_start_time_timestamp",
			Help: "Application start time as Unix timestamp",
		},
	)

	registry.MustRegister(infoGauge)
	registry.MustRegister(startTimeGauge)

	metrics := &Metrics{
		ApplicationInfo:      infoGauge,
		ApplicationStartTime: startTimeGauge,
	}

	metrics.SetApplicationInfo("v1.0.0", "abc123", "2024-01-01")

	infoValue := testutil.ToFloat64(infoGauge.WithLabelValues("v1.0.0", "abc123", "2024-01-01"))
	assert.Equal(t, float64(1), infoValue)

	// Start time should be set to current time (approximately)
	startTimeValue := testutil.ToFloat64(startTimeGauge)
	require.Greater(t, startTimeValue, float64(0))
}

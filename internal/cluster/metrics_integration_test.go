package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// MockMetricsRecorder implements MetricsRecorder for testing
type MockMetricsRecorder struct {
	QueryCalls []QueryCall
	StatsCalls []StatsCall
}

type QueryCall struct {
	ClusterID string
	Operation string
	Duration  time.Duration
	Error     error
}

type StatsCall struct {
	ClusterID   string
	Nodes       int
	TotalPods   int
	RunningPods int
	PendingPods int
	ErrorPods   int
}

func (m *MockMetricsRecorder) RecordClusterQuery(clusterID, operation string, duration time.Duration, err error) {
	m.QueryCalls = append(m.QueryCalls, QueryCall{
		ClusterID: clusterID,
		Operation: operation,
		Duration:  duration,
		Error:     err,
	})
}

func (m *MockMetricsRecorder) UpdateClusterStats(clusterID string, nodes, totalPods, runningPods, pendingPods, errorPods int) {
	m.StatsCalls = append(m.StatsCalls, StatsCall{
		ClusterID:   clusterID,
		Nodes:       nodes,
		TotalPods:   totalPods,
		RunningPods: runningPods,
		PendingPods: pendingPods,
		ErrorPods:   errorPods,
	})
}

func TestQueryEngine_MetricsIntegration(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()
	metricsRecorder := &MockMetricsRecorder{}

	qe := NewQueryEngine(config, store, publisher, metricsRecorder)

	// Create test Kubernetes objects
	node1 := createTestNode("node1")
	pod1 := createTestPod("pod1", "default", "node1")
	pod1.Status.Phase = corev1.PodRunning // Set pod to running state

	pod2 := createTestPod("pod2", "default", "node1")
	pod2.Status.Phase = corev1.PodPending // Set pod to pending state

	nodeMetrics1 := createTestNodeMetrics("node1")
	podMetrics1 := createTestPodMetrics("pod1", "default")

	// Create fake clients with test data
	fakeClient := fake.NewSimpleClientset(node1, pod1, pod2)
	fakeMetricsClient := metricsfake.NewSimpleClientset(nodeMetrics1, podMetrics1)

	client := &ClusterClient{
		ID:            "test-cluster",
		APIServer:     "https://api.test-cluster.com",
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
	}

	// Perform a query
	ctx := context.Background()
	err := qe.queryCluster(ctx, client)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify metrics were recorded
	if len(metricsRecorder.QueryCalls) != 1 {
		t.Fatalf("Expected 1 query call, got %d", len(metricsRecorder.QueryCalls))
	}

	queryCall := metricsRecorder.QueryCalls[0]
	if queryCall.ClusterID != "test-cluster" {
		t.Errorf("Expected cluster ID 'test-cluster', got '%s'", queryCall.ClusterID)
	}
	if queryCall.Operation != "query" {
		t.Errorf("Expected operation 'query', got '%s'", queryCall.Operation)
	}
	if queryCall.Error != nil {
		t.Errorf("Expected no error, got %v", queryCall.Error)
	}
	if queryCall.Duration <= 0 {
		t.Errorf("Expected positive duration, got %v", queryCall.Duration)
	}

	// Verify stats were recorded
	if len(metricsRecorder.StatsCalls) != 1 {
		t.Fatalf("Expected 1 stats call, got %d", len(metricsRecorder.StatsCalls))
	}

	statsCall := metricsRecorder.StatsCalls[0]
	if statsCall.ClusterID != "test-cluster" {
		t.Errorf("Expected cluster ID 'test-cluster', got '%s'", statsCall.ClusterID)
	}
	if statsCall.Nodes != 1 {
		t.Errorf("Expected 1 node, got %d", statsCall.Nodes)
	}
	if statsCall.TotalPods != 2 {
		t.Errorf("Expected 2 total pods, got %d", statsCall.TotalPods)
	}
	if statsCall.RunningPods != 1 {
		t.Errorf("Expected 1 running pod, got %d", statsCall.RunningPods)
	}
	if statsCall.PendingPods != 1 {
		t.Errorf("Expected 1 pending pod, got %d", statsCall.PendingPods)
	}
	if statsCall.ErrorPods != 0 {
		t.Errorf("Expected 0 error pods, got %d", statsCall.ErrorPods)
	}
}

func TestQueryEngine_MetricsIntegration_ErrorHandling(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()
	metricsRecorder := &MockMetricsRecorder{}

	qe := NewQueryEngine(config, store, publisher, metricsRecorder)

	// Create a client that will fail
	fakeClient := fake.NewSimpleClientset()
	// Simulate API server error
	fakeClient.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated API server error")
	})

	client := &ClusterClient{
		ID:        "test-cluster",
		APIServer: "https://api.test-cluster.com",
		Client:    fakeClient,
	}

	// Perform a query that should fail
	ctx := context.Background()
	err := qe.queryCluster(ctx, client)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// The error should be recorded during the retry logic, not in queryCluster itself
	// So we need to test the retry logic instead
	qe.queryClusterWithRetry(client)

	// Verify error metrics were recorded
	if len(metricsRecorder.QueryCalls) == 0 {
		t.Fatal("Expected at least one query call for error recording")
	}

	// Check that at least one call recorded an error
	hasError := false
	for _, call := range metricsRecorder.QueryCalls {
		if call.Error != nil {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("Expected at least one query call to record an error")
	}
}

package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kube-ops-view/internal/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// MockStore implements the Store interface for testing
type MockStore struct {
	clusterData   map[string]*models.ClusterData
	clusterStatus map[string]*ClusterStatus
}

func NewMockStore() *MockStore {
	return &MockStore{
		clusterData:   make(map[string]*models.ClusterData),
		clusterStatus: make(map[string]*ClusterStatus),
	}
}

func (ms *MockStore) SetClusterData(clusterID string, data *models.ClusterData) error {
	ms.clusterData[clusterID] = data
	return nil
}

func (ms *MockStore) GetClusterData(clusterID string) (*models.ClusterData, error) {
	if data, exists := ms.clusterData[clusterID]; exists {
		return data, nil
	}
	return nil, nil
}

func (ms *MockStore) SetClusterStatus(clusterID string, status *ClusterStatus) error {
	ms.clusterStatus[clusterID] = status
	return nil
}

func (ms *MockStore) GetClusterStatus(clusterID string) (*ClusterStatus, error) {
	if status, exists := ms.clusterStatus[clusterID]; exists {
		return status, nil
	}
	return nil, nil
}

func (ms *MockStore) DeleteCluster(clusterID string) error {
	delete(ms.clusterData, clusterID)
	delete(ms.clusterStatus, clusterID)
	return nil
}

// MockEventPublisher implements the EventPublisher interface for testing
type MockEventPublisher struct {
	clusterUpdates []string
	statusUpdates  []string
}

func NewMockEventPublisher() *MockEventPublisher {
	return &MockEventPublisher{
		clusterUpdates: make([]string, 0),
		statusUpdates:  make([]string, 0),
	}
}

func (mep *MockEventPublisher) PublishClusterUpdate(clusterID string, data *models.ClusterData) error {
	mep.clusterUpdates = append(mep.clusterUpdates, clusterID)
	return nil
}

func (mep *MockEventPublisher) PublishClusterStatus(clusterID string, status *ClusterStatus) error {
	mep.statusUpdates = append(mep.statusUpdates, clusterID)
	return nil
}

// Helper function to create a test node
func createTestNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"kubernetes.io/hostname":         name,
				"node-role.kubernetes.io/worker": "",
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3800m"),
				corev1.ResourceMemory: resource.MustParse("7.5Gi"),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "10.0.0.1",
				},
			},
		},
	}
}

// Helper function to create a test pod
func createTestPod(name, namespace, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": "test-app",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "test-container",
					Ready:        true,
					RestartCount: 0,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.Now(),
						},
					},
				},
			},
			StartTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}
}

// Helper function to create test node metrics
func createTestNodeMetrics(nodeName string) *metricsv1beta1.NodeMetrics {
	return &metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1500m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}
}

// Helper function to create test pod metrics
func createTestPodMetrics(podName, namespace string) *metricsv1beta1.PodMetrics {
	return &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Containers: []metricsv1beta1.ContainerMetrics{
			{
				Name: "test-container",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
	}
}

func TestNewQueryEngine(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	if qe == nil {
		t.Fatal("Expected QueryEngine to be created, got nil")
	}

	if qe.interval != config.QueryInterval {
		t.Errorf("Expected interval %v, got %v", config.QueryInterval, qe.interval)
	}

	if qe.timeout != config.Timeout {
		t.Errorf("Expected timeout %v, got %v", config.Timeout, qe.timeout)
	}
}

func TestQueryEngine_AddRemoveCluster(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create a mock cluster client
	clusterConfig := &ClusterConfig{
		ID:        "test-cluster",
		APIServer: "https://test-cluster.example.com",
	}

	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	fakeMetricsClient := metricsfake.NewSimpleClientset()

	client := &ClusterClient{
		ID:            clusterConfig.ID,
		APIServer:     clusterConfig.APIServer,
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
		config:        clusterConfig,
	}

	// Test adding cluster
	qe.AddCluster(client)

	if len(qe.clients) != 1 {
		t.Errorf("Expected 1 cluster, got %d", len(qe.clients))
	}

	if _, exists := qe.clients["test-cluster"]; !exists {
		t.Error("Expected cluster 'test-cluster' to be added")
	}

	// Test removing cluster
	qe.RemoveCluster("test-cluster")

	if len(qe.clients) != 0 {
		t.Errorf("Expected 0 clusters, got %d", len(qe.clients))
	}
}

func TestQueryEngine_ConvertToClusterData(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create test data
	node1 := createTestNode("node1")
	pod1 := createTestPod("pod1", "default", "node1")
	pod2 := createTestPod("pod2", "kube-system", "") // Unassigned pod

	nodes := &corev1.NodeList{
		Items: []corev1.Node{*node1},
	}

	pods := &corev1.PodList{
		Items: []corev1.Pod{*pod1, *pod2},
	}

	nodeMetrics := []NodeMetrics{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
			Usage: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1500m"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		},
	}

	podMetrics := []PodMetrics{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "default",
			},
			Containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "test-container",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
		},
	}

	// Create mock cluster client
	fakeClient := fake.NewSimpleClientset()
	fakeMetricsClient := metricsfake.NewSimpleClientset()

	client := &ClusterClient{
		ID:            "test-cluster",
		APIServer:     "https://test-cluster.example.com",
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
	}

	// Test conversion
	clusterData, err := qe.convertToClusterData(client, nodes, pods, nodeMetrics, podMetrics)
	if err != nil {
		t.Fatalf("Failed to convert cluster data: %v", err)
	}

	// Verify cluster data
	if clusterData.ID != "test-cluster" {
		t.Errorf("Expected cluster ID 'test-cluster', got '%s'", clusterData.ID)
	}

	if len(clusterData.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(clusterData.Nodes))
	}

	if len(clusterData.UnassignedPods) != 1 {
		t.Errorf("Expected 1 unassigned pod, got %d", len(clusterData.UnassignedPods))
	}

	// Verify node data
	node, exists := clusterData.Nodes["node1"]
	if !exists {
		t.Fatal("Expected node 'node1' to exist")
	}

	if node.Name != "node1" {
		t.Errorf("Expected node name 'node1', got '%s'", node.Name)
	}

	if !node.Status.Ready {
		t.Error("Expected node to be ready")
	}

	if len(node.Pods) != 1 {
		t.Errorf("Expected 1 pod on node, got %d", len(node.Pods))
	}

	// Verify pod data
	podKey := "default/pod1"
	pod, exists := node.Pods[podKey]
	if !exists {
		t.Fatalf("Expected pod '%s' to exist on node", podKey)
	}

	if pod.Name != "pod1" {
		t.Errorf("Expected pod name 'pod1', got '%s'", pod.Name)
	}

	if pod.Namespace != "default" {
		t.Errorf("Expected pod namespace 'default', got '%s'", pod.Namespace)
	}

	if !pod.Ready {
		t.Error("Expected pod to be ready")
	}

	// Verify container data
	if len(pod.Containers) != 1 {
		t.Errorf("Expected 1 container, got %d", len(pod.Containers))
	}

	container := pod.Containers[0]
	if container.Name != "test-container" {
		t.Errorf("Expected container name 'test-container', got '%s'", container.Name)
	}

	if container.Image != "nginx:latest" {
		t.Errorf("Expected container image 'nginx:latest', got '%s'", container.Image)
	}

	// Verify resource requests
	cpuRequest, exists := container.Resources.Requests["cpu"]
	if !exists {
		t.Error("Expected CPU request to exist")
	}
	if cpuRequest != "100m" {
		t.Errorf("Expected CPU request '100m', got '%s'", cpuRequest)
	}

	// Verify metrics
	if node.Usage == nil {
		t.Error("Expected node usage metrics to be set")
	} else {
		if node.Usage.CPU != "1500m" {
			t.Errorf("Expected node CPU usage '1500m', got '%s'", node.Usage.CPU)
		}
		if node.Usage.Memory != "2Gi" {
			t.Errorf("Expected node memory usage '2Gi', got '%s'", node.Usage.Memory)
		}
	}

	if container.Usage == nil {
		t.Error("Expected container usage metrics to be set")
	} else {
		if container.Usage.CPU != "50m" {
			t.Errorf("Expected container CPU usage '50m', got '%s'", container.Usage.CPU)
		}
		if container.Usage.Memory != "64Mi" {
			t.Errorf("Expected container memory usage '64Mi', got '%s'", container.Usage.Memory)
		}
	}
}

func TestQueryEngine_QueryCluster(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create test Kubernetes objects
	node1 := createTestNode("node1")
	pod1 := createTestPod("pod1", "default", "node1")
	nodeMetrics1 := createTestNodeMetrics("node1")
	podMetrics1 := createTestPodMetrics("pod1", "default")

	// Create fake clients with test data
	fakeClient := fake.NewSimpleClientset(node1, pod1)
	fakeMetricsClient := metricsfake.NewSimpleClientset(nodeMetrics1, podMetrics1)

	client := &ClusterClient{
		ID:            "test-cluster",
		APIServer:     "https://test-cluster.example.com",
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
	}

	// Test query
	ctx := context.Background()
	err := qe.queryCluster(ctx, client)
	if err != nil {
		t.Fatalf("Failed to query cluster: %v", err)
	}

	// Verify data was stored
	storedData, err := store.GetClusterData("test-cluster")
	if err != nil {
		t.Fatalf("Failed to get stored cluster data: %v", err)
	}

	if storedData == nil {
		t.Fatal("Expected cluster data to be stored")
	}

	if storedData.ID != "test-cluster" {
		t.Errorf("Expected cluster ID 'test-cluster', got '%s'", storedData.ID)
	}

	// Verify event was published
	if len(publisher.clusterUpdates) != 1 {
		t.Errorf("Expected 1 cluster update event, got %d", len(publisher.clusterUpdates))
	}

	if publisher.clusterUpdates[0] != "test-cluster" {
		t.Errorf("Expected cluster update for 'test-cluster', got '%s'", publisher.clusterUpdates[0])
	}
}

func TestQueryEngine_QueryClusterWithoutMetrics(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create test Kubernetes objects
	node1 := createTestNode("node1")
	pod1 := createTestPod("pod1", "default", "node1")

	// Create fake clients without metrics
	fakeClient := fake.NewSimpleClientset(node1, pod1)
	fakeMetricsClient := metricsfake.NewSimpleClientset() // Empty metrics client

	client := &ClusterClient{
		ID:            "test-cluster",
		APIServer:     "https://test-cluster.example.com",
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
	}

	// Test query
	ctx := context.Background()
	err := qe.queryCluster(ctx, client)
	if err != nil {
		t.Fatalf("Failed to query cluster: %v", err)
	}

	// Verify data was stored even without metrics
	storedData, err := store.GetClusterData("test-cluster")
	if err != nil {
		t.Fatalf("Failed to get stored cluster data: %v", err)
	}

	if storedData == nil {
		t.Fatal("Expected cluster data to be stored")
	}

	// Verify node exists without metrics
	node, exists := storedData.Nodes["node1"]
	if !exists {
		t.Fatal("Expected node 'node1' to exist")
	}

	// Node usage should be calculated from pod requests since no metrics available
	if node.Usage == nil {
		t.Error("Expected node usage to be calculated from pod requests")
	}
}

func TestQueryEngine_ResourceCalculations(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create a pod with specific resource requests
	pod := &models.Pod{
		Name:      "test-pod",
		Namespace: "default",
		Containers: []models.Container{
			{
				Name:  "container1",
				Image: "nginx:latest",
				Resources: models.ContainerResources{
					Requests: models.ResourceList{
						"cpu":    "100m",
						"memory": "128Mi",
					},
					Limits: models.ResourceList{
						"cpu":    "200m",
						"memory": "256Mi",
					},
				},
			},
			{
				Name:  "container2",
				Image: "redis:latest",
				Resources: models.ContainerResources{
					Requests: models.ResourceList{
						"cpu":    "50m",
						"memory": "64Mi",
					},
				},
			},
		},
	}

	// Test CPU requests calculation
	totalCPU, err := pod.GetTotalCPURequests()
	if err != nil {
		t.Fatalf("Failed to calculate total CPU requests: %v", err)
	}

	expectedCPU := int64(150) // 100m + 50m
	if totalCPU != expectedCPU {
		t.Errorf("Expected total CPU requests %d, got %d", expectedCPU, totalCPU)
	}

	// Test memory requests calculation
	totalMemory, err := pod.GetTotalMemoryRequests()
	if err != nil {
		t.Fatalf("Failed to calculate total memory requests: %v", err)
	}

	expectedMemory := int64(201326592) // 128Mi + 64Mi in bytes
	if totalMemory != expectedMemory {
		t.Errorf("Expected total memory requests %d, got %d", expectedMemory, totalMemory)
	}

	// Test node resource calculation
	node := &models.Node{
		Name: "test-node",
		Pods: map[string]*models.Pod{
			"default/test-pod": pod,
		},
	}

	qe.calculateNodeResourceUsage(node)

	if node.Usage == nil {
		t.Fatal("Expected node usage to be calculated")
	}

	// Should use formatted values
	if node.Usage.CPU != "150m" {
		t.Errorf("Expected node CPU usage '150m', got '%s'", node.Usage.CPU)
	}
}

func TestQueryEngine_ErrorHandling(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create a client that will fail (empty fake client with no objects)
	fakeClient := fake.NewSimpleClientset()
	// Simulate API server error by using a client that returns errors
	fakeClient.PrependReactor("list", "nodes", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("simulated API server error")
	})

	client := &ClusterClient{
		ID:        "failing-cluster",
		APIServer: "https://failing-cluster.example.com",
		Client:    fakeClient,
	}

	// Test query with error
	ctx := context.Background()
	err := qe.queryCluster(ctx, client)
	if err == nil {
		t.Error("Expected query to fail, but it succeeded")
	}

	// Verify error handling doesn't crash the system
	if err.Error() == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestQueryEngine_StartStop(t *testing.T) {
	config := &QueryEngineConfig{
		QueryInterval: 100 * time.Millisecond, // Short interval for testing
		Timeout:       1 * time.Second,
	}
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Add a test cluster
	node1 := createTestNode("node1")
	fakeClient := fake.NewSimpleClientset(node1)
	fakeMetricsClient := metricsfake.NewSimpleClientset()

	client := &ClusterClient{
		ID:            "test-cluster",
		APIServer:     "https://test-cluster.example.com",
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
	}

	qe.AddCluster(client)

	// Start the query engine
	qe.Start()

	// Wait for at least one query cycle
	time.Sleep(200 * time.Millisecond)

	// Stop the query engine
	qe.Stop()

	// Verify that data was collected
	storedData, err := store.GetClusterData("test-cluster")
	if err != nil {
		t.Fatalf("Failed to get stored cluster data: %v", err)
	}

	if storedData == nil {
		t.Error("Expected cluster data to be collected during polling")
	}

	// Verify that events were published
	if len(publisher.clusterUpdates) == 0 {
		t.Error("Expected at least one cluster update event")
	}
}

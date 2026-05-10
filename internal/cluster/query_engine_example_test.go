package cluster

import (
	"context"
	"fmt"
	"log"
	"time"

	"kube-ops-view/internal/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// ExampleQueryEngine demonstrates how to use the QueryEngine
func ExampleQueryEngine() {
	// Create a mock store and event publisher
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	// Configure the query engine
	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}

	// Create the query engine
	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create a mock cluster with some test data
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-1",
			Labels: map[string]string{
				"kubernetes.io/hostname":         "worker-node-1",
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
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deployment-abc123",
			Namespace: "default",
			Labels: map[string]string{
				"app": "nginx",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-node-1",
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.21",
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
		},
	}

	// Create metrics
	nodeMetrics := &metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-1",
		},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1500m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	podMetrics := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deployment-abc123",
			Namespace: "default",
		},
		Containers: []metricsv1beta1.ContainerMetrics{
			{
				Name: "nginx",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
	}

	// Create fake Kubernetes clients with test data
	fakeClient := fake.NewSimpleClientset(node, pod)
	fakeMetricsClient := metricsfake.NewSimpleClientset(nodeMetrics, podMetrics)

	// Create cluster client
	client := &ClusterClient{
		ID:            "example-cluster",
		APIServer:     "https://example-cluster.k8s.local:6443",
		Client:        fakeClient,
		MetricsClient: fakeMetricsClient,
	}

	// Add cluster to query engine
	qe.AddCluster(client)

	// Perform a single query (normally this would be done automatically by Start())
	ctx := context.Background()
	err := qe.queryCluster(ctx, client)
	if err != nil {
		log.Fatalf("Failed to query cluster: %v", err)
	}

	// Retrieve the collected data
	clusterData, err := qe.GetClusterData("example-cluster")
	if err != nil {
		log.Fatalf("Failed to get cluster data: %v", err)
	}

	// Display the results
	fmt.Printf("Cluster ID: %s\n", clusterData.ID)
	fmt.Printf("API Server: %s\n", clusterData.APIServerURL)
	fmt.Printf("Nodes: %d\n", len(clusterData.Nodes))
	fmt.Printf("Unassigned Pods: %d\n", len(clusterData.UnassignedPods))

	// Show node details
	for nodeName, node := range clusterData.Nodes {
		fmt.Printf("\nNode: %s\n", nodeName)
		fmt.Printf("  Ready: %t\n", node.Status.Ready)
		fmt.Printf("  Pods: %d\n", len(node.Pods))
		if node.Usage != nil {
			fmt.Printf("  CPU Usage: %s\n", node.Usage.CPU)
			fmt.Printf("  Memory Usage: %s\n", node.Usage.Memory)
		}

		// Show pod details
		for podKey, pod := range node.Pods {
			fmt.Printf("    Pod: %s\n", podKey)
			fmt.Printf("      Phase: %s\n", pod.Phase)
			fmt.Printf("      Ready: %t\n", pod.Ready)
			fmt.Printf("      Containers: %d\n", len(pod.Containers))

			// Show resource requests
			if cpuReq, err := pod.GetTotalCPURequests(); err == nil {
				fmt.Printf("      CPU Requests: %s\n", models.FormatCPU(cpuReq))
			}
			if memReq, err := pod.GetTotalMemoryRequests(); err == nil {
				fmt.Printf("      Memory Requests: %s\n", models.FormatMemory(memReq))
			}
		}
	}

	// Output:
	// Cluster ID: example-cluster
	// API Server: https://example-cluster.k8s.local:6443
	// Nodes: 1
	// Unassigned Pods: 0
	//
	// Node: worker-node-1
	//   Ready: true
	//   Pods: 1
	//   CPU Usage: 100m
	//   Memory Usage: 128.0Mi
	//     Pod: default/nginx-deployment-abc123
	//       Phase: Running
	//       Ready: true
	//       Containers: 1
	//       CPU Requests: 100m
	//       Memory Requests: 128.0Mi
}

// ExampleQueryEngine_withRetry demonstrates error handling and retry logic
func ExampleQueryEngine_withRetry() {
	store := NewMockStore()
	publisher := NewMockEventPublisher()

	config := &QueryEngineConfig{
		QueryInterval: 5 * time.Second,
		Timeout:       30 * time.Second,
	}

	qe := NewQueryEngine(config, store, publisher, &NoOpMetricsRecorder{})

	// Create a client that will initially fail
	fakeClient := fake.NewSimpleClientset()

	client := &ClusterClient{
		ID:        "unreliable-cluster",
		APIServer: "https://unreliable-cluster.k8s.local:6443",
		Client:    fakeClient,
	}

	qe.AddCluster(client)

	// This will trigger the retry logic since there are no nodes in the fake client
	// In a real scenario, this might be due to network issues, authentication problems, etc.
	qe.queryClusterWithRetry(client)

	// Check cluster status
	status, err := qe.GetClusterStatus("unreliable-cluster")
	if err != nil {
		log.Printf("Failed to get cluster status: %v", err)
		return
	}

	if status != nil {
		fmt.Printf("Cluster Status: %s\n", status.ID)
		fmt.Printf("Available: %t\n", status.Available)
		if status.Error != "" {
			fmt.Printf("Error: %s\n", status.Error)
		}
	}

	// Output will vary based on the specific error, but demonstrates error handling
}

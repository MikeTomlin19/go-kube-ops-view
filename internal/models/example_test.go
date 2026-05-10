package models_test

import (
	"fmt"
	"time"

	"kube-ops-view/internal/models"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExampleClusterData demonstrates how to create and use ClusterData
func ExampleClusterData() {
	// Create a new cluster data instance
	clusterData := models.CreateClusterData("production-cluster", "https://k8s-api.example.com")

	// Create a sample pod
	pod := &models.Pod{
		Name:      "nginx-deployment-abc123",
		Namespace: "default",
		Phase:     "Running",
		Ready:     true,
		Labels: map[string]string{
			"app": "nginx",
		},
		Containers: []models.Container{
			{
				Name:  "nginx",
				Image: "nginx:1.21",
				Ready: true,
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
		},
	}

	// Add the pod to a node
	clusterData.AddPodToNode(pod, "worker-node-1")

	// Get cluster statistics
	fmt.Printf("Cluster: %s\n", clusterData.ID)
	fmt.Printf("Nodes: %d\n", clusterData.GetNodeCount())
	fmt.Printf("Pods: %d\n", clusterData.GetPodCount())

	// Calculate resource usage
	cpuRequests, _ := pod.GetTotalCPURequests()
	memoryRequests, _ := pod.GetTotalMemoryRequests()

	fmt.Printf("Pod CPU requests: %s\n", models.FormatCPU(cpuRequests))
	fmt.Printf("Pod memory requests: %s\n", models.FormatMemory(memoryRequests))

	// Output:
	// Cluster: production-cluster
	// Nodes: 1
	// Pods: 1
	// Pod CPU requests: 100m
	// Pod memory requests: 128.0Mi
}

// ExampleConvertNode demonstrates converting Kubernetes API objects to internal models
func ExampleConvertNode() {
	// Create a Kubernetes Node object (as would come from the API)
	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-1",
			Labels: map[string]string{
				"kubernetes.io/hostname":           "worker-node-1",
				"node.kubernetes.io/instance-type": "m5.large",
				"topology.kubernetes.io/zone":      "us-west-2a",
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1900m"),
				corev1.ResourceMemory: resource.MustParse("7Gi"),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	// Convert to internal model
	node := models.ConvertNode(k8sNode)

	fmt.Printf("Node: %s\n", node.Name)
	fmt.Printf("Ready: %t\n", node.Status.Ready)
	fmt.Printf("CPU Capacity: %s\n", node.Capacity["cpu"])
	fmt.Printf("Memory Capacity: %s\n", node.Capacity["memory"])
	fmt.Printf("Instance Type: %s\n", node.Labels["node.kubernetes.io/instance-type"])

	// Output:
	// Node: worker-node-1
	// Ready: true
	// CPU Capacity: 2
	// Memory Capacity: 8Gi
	// Instance Type: m5.large
}

// ExampleParseCPU demonstrates resource parsing and formatting
func ExampleParseCPU() {
	// Parse CPU values
	cpuParsed, _ := models.ParseCPU("500m")
	fmt.Printf("CPU: %s = %.1f cores (%d millicores)\n",
		cpuParsed.Raw, cpuParsed.Value, cpuParsed.MilliUnit)

	// Parse memory values
	memParsed, _ := models.ParseMemory("2Gi")
	fmt.Printf("Memory: %s = %.0f bytes\n",
		memParsed.Raw, memParsed.Value)

	// Format values back to human-readable strings
	fmt.Printf("Formatted CPU: %s\n", models.FormatCPU(1500))
	fmt.Printf("Formatted Memory: %s\n", models.FormatMemory(2147483648))

	// Output:
	// CPU: 500m = 0.5 cores (500 millicores)
	// Memory: 2Gi = 2147483648 bytes
	// Formatted CPU: 1.5
	// Formatted Memory: 2.0Gi
}

// ExampleClusterData_Validate demonstrates data validation
func ExampleClusterData_Validate() {
	// Create a valid cluster data
	clusterData := &models.ClusterData{
		ID:             "test-cluster",
		APIServerURL:   "https://api.test.com",
		Nodes:          make(map[string]*models.Node),
		UnassignedPods: make(map[string]*models.Pod),
		LastUpdate:     time.Now(),
	}

	// Validate - should pass
	if err := clusterData.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	} else {
		fmt.Println("Cluster data is valid")
	}

	// Create invalid cluster data (missing ID)
	invalidCluster := &models.ClusterData{
		APIServerURL: "https://api.test.com",
	}

	// Validate - should fail
	if err := invalidCluster.Validate(); err != nil {
		fmt.Printf("Validation error: %v\n", err)
	}

	// Output:
	// Cluster data is valid
	// Validation error: cluster ID cannot be empty
}

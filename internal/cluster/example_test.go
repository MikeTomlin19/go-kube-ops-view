package cluster

import (
	"context"
	"fmt"
	"log"
)

// ExampleClusterClient demonstrates how to use the cluster client integration
func ExampleClusterClient() {
	// Create a cluster configuration
	config := &ClusterConfig{
		ID:        "my-cluster",
		APIServer: "https://kubernetes.example.com",
		Token:     "my-token",
	}

	// Create a new cluster client
	client, err := NewClusterClient(config)
	if err != nil {
		log.Fatalf("Failed to create cluster client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test the connection
	if err := client.TestConnection(ctx); err != nil {
		log.Fatalf("Failed to connect to cluster: %v", err)
	}

	// Get cluster status
	status, err := client.GetClusterStatus(ctx)
	if err != nil {
		log.Fatalf("Failed to get cluster status: %v", err)
	}

	fmt.Printf("Cluster %s is available: %v\n", status.ID, status.Available)

	// Get nodes
	nodes, err := client.GetNodes(ctx)
	if err != nil {
		log.Fatalf("Failed to get nodes: %v", err)
	}

	fmt.Printf("Found %d nodes\n", len(nodes.Items))

	// Get pods
	pods, err := client.GetPods(ctx)
	if err != nil {
		log.Fatalf("Failed to get pods: %v", err)
	}

	fmt.Printf("Found %d pods\n", len(pods.Items))

	// Check if metrics are available
	if client.IsMetricsAvailable(ctx) {
		// Get node metrics
		nodeMetrics, err := client.GetNodeMetrics(ctx)
		if err != nil {
			log.Printf("Failed to get node metrics: %v", err)
		} else {
			fmt.Printf("Found metrics for %d nodes\n", len(nodeMetrics))
		}

		// Get pod metrics
		podMetrics, err := client.GetPodMetrics(ctx)
		if err != nil {
			log.Printf("Failed to get pod metrics: %v", err)
		} else {
			fmt.Printf("Found metrics for %d pods\n", len(podMetrics))
		}
	} else {
		fmt.Println("Metrics server not available")
	}
}

// ExampleStaticDiscoverer demonstrates how to discover clusters from static URLs
func ExampleStaticDiscoverer() {
	discoverer := NewStaticDiscoverer([]string{
		"https://cluster1.example.com",
		"https://cluster2.example.com",
	})

	ctx := context.Background()
	configs, err := discoverer.DiscoverClusters(ctx)
	if err != nil {
		log.Fatalf("Failed to discover clusters: %v", err)
	}

	fmt.Printf("Discovered %d clusters\n", len(configs))
	for _, config := range configs {
		fmt.Printf("- Cluster %s: %s\n", config.ID, config.APIServer)
	}
}

// ExampleKubeconfigDiscoverer demonstrates how to discover clusters from kubeconfig
func ExampleKubeconfigDiscoverer() {
	discoverer := NewKubeconfigDiscoverer("~/.kube/config", []string{})

	ctx := context.Background()
	configs, err := discoverer.DiscoverClusters(ctx)
	if err != nil {
		log.Fatalf("Failed to discover clusters: %v", err)
	}

	fmt.Printf("Discovered %d clusters from kubeconfig\n", len(configs))
	for _, config := range configs {
		fmt.Printf("- Context %s: %s\n", config.ID, config.APIServer)
	}
}

// ExampleMultiDiscoverer demonstrates how to combine multiple discovery methods
func ExampleMultiDiscoverer() {
	staticDiscoverer := NewStaticDiscoverer([]string{
		"https://prod-cluster.example.com",
	})

	kubeconfigDiscoverer := NewKubeconfigDiscoverer("~/.kube/config", []string{})

	multiDiscoverer := NewMultiDiscoverer(staticDiscoverer, kubeconfigDiscoverer)

	ctx := context.Background()
	configs, err := multiDiscoverer.DiscoverClusters(ctx)
	if err != nil {
		log.Fatalf("Failed to discover clusters: %v", err)
	}

	fmt.Printf("Discovered %d clusters total\n", len(configs))
	for _, config := range configs {
		fmt.Printf("- Cluster %s: %s\n", config.ID, config.APIServer)
	}
}

// ExampleDiscovererFactory demonstrates how to use the factory to create discoverers
func ExampleDiscovererFactory() {
	factory := NewDiscovererFactory()

	discoverer := factory.CreateDiscoverer(
		[]string{"https://static-cluster.example.com"}, // static URLs
		"~/.kube/config", // kubeconfig path
		[]string{"prod-context", "staging-context"}, // specific contexts
		"https://registry.example.com/clusters",     // registry URL
	)

	ctx := context.Background()
	configs, err := discoverer.DiscoverClusters(ctx)
	if err != nil {
		log.Fatalf("Failed to discover clusters: %v", err)
	}

	fmt.Printf("Factory discovered %d clusters\n", len(configs))
}

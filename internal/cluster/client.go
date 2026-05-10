package cluster

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

// NewClusterClient creates a new ClusterClient from a ClusterConfig
func NewClusterClient(config *ClusterConfig) (*ClusterClient, error) {
	restConfig, err := buildRestConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build rest config for cluster %s: %w", config.ID, err)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client for cluster %s: %w", config.ID, err)
	}

	// Create metrics client
	metricsClient, err := versioned.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics client for cluster %s: %w", config.ID, err)
	}

	return &ClusterClient{
		ID:            config.ID,
		APIServer:     config.APIServer,
		Client:        client,
		MetricsClient: metricsClient,
		config:        config,
	}, nil
}

// buildRestConfig creates a rest.Config from ClusterConfig
func buildRestConfig(config *ClusterConfig) (*rest.Config, error) {
	var restConfig *rest.Config
	var err error

	// If kubeconfig path is provided, use it
	if config.KubeconfigPath != "" {
		restConfig, err = buildConfigFromKubeconfig(config.KubeconfigPath, config.Context)
		if err != nil {
			return nil, err
		}
	} else {
		// Build config from individual parameters
		restConfig = &rest.Config{
			Host: config.APIServer,
		}

		// Set authentication
		if config.Token != "" {
			restConfig.BearerToken = config.Token
		}

		// Set TLS configuration
		if config.CertFile != "" && config.KeyFile != "" {
			restConfig.CertFile = config.CertFile
			restConfig.KeyFile = config.KeyFile
		}

		if config.CAFile != "" {
			restConfig.CAFile = config.CAFile
		}

		if config.Insecure {
			restConfig.Insecure = true
		}
	}

	// Set default timeout
	restConfig.Timeout = 30 * time.Second

	return restConfig, nil
}

// buildConfigFromKubeconfig creates a rest.Config from kubeconfig file
func buildConfigFromKubeconfig(kubeconfigPath, context string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = kubeconfigPath

	configOverrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		configOverrides.CurrentContext = context
	}

	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	)

	return config.ClientConfig()
}

// TestConnection tests the connection to the cluster
func (c *ClusterClient) TestConnection(ctx context.Context) error {
	_, err := c.Client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("failed to connect to cluster %s: %w", c.ID, err)
	}
	return nil
}

// GetNodes retrieves all nodes from the cluster
func (c *ClusterClient) GetNodes(ctx context.Context) (*corev1.NodeList, error) {
	nodes, err := c.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes from cluster %s: %w", c.ID, err)
	}
	return nodes, nil
}

// GetPods retrieves all pods from the cluster
func (c *ClusterClient) GetPods(ctx context.Context) (*corev1.PodList, error) {
	pods, err := c.Client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pods from cluster %s: %w", c.ID, err)
	}
	return pods, nil
}

// GetPodsForNode retrieves pods scheduled on a specific node
func (c *ClusterClient) GetPodsForNode(ctx context.Context, nodeName string) (*corev1.PodList, error) {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := c.Client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get pods for node %s in cluster %s: %w", nodeName, c.ID, err)
	}
	return pods, nil
}

// GetClusterVersion retrieves the Kubernetes version of the cluster
func (c *ClusterClient) GetClusterVersion(ctx context.Context) (string, error) {
	version, err := c.Client.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get cluster version for %s: %w", c.ID, err)
	}
	return version.GitVersion, nil
}

// GetClusterStatus retrieves the current status of the cluster
func (c *ClusterClient) GetClusterStatus(ctx context.Context) (*ClusterStatus, error) {
	status := &ClusterStatus{
		ID:        c.ID,
		Available: false,
		LastSeen:  time.Now(),
	}

	// Test connection
	if err := c.TestConnection(ctx); err != nil {
		status.Error = err.Error()
		return status, nil
	}

	status.Available = true

	// Get version
	if version, err := c.GetClusterVersion(ctx); err == nil {
		status.Version = version
	}

	// Get node count
	if nodes, err := c.GetNodes(ctx); err == nil {
		status.NodeCount = len(nodes.Items)
	}

	return status, nil
}

// GetVersion retrieves the Kubernetes version of the cluster (alias for GetClusterVersion)
func (c *ClusterClient) GetVersion(ctx context.Context) (string, error) {
	return c.GetClusterVersion(ctx)
}

// Close cleans up any resources used by the client
func (c *ClusterClient) Close() error {
	// Currently no cleanup needed for client-go clients
	// This method is here for future extensibility
	return nil
}

// IsMetricsAvailable checks if the metrics API is available for this cluster
func (c *ClusterClient) IsMetricsAvailable(ctx context.Context) bool {
	if c.MetricsClient == nil {
		return false
	}

	// Try to list node metrics to check if the API is available
	_, err := c.MetricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

// GetNodeMetrics retrieves node metrics from the cluster
func (c *ClusterClient) GetNodeMetrics(ctx context.Context) ([]NodeMetrics, error) {
	if c.MetricsClient == nil {
		return nil, fmt.Errorf("metrics client not available")
	}

	nodeMetricsList, err := c.MetricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node metrics: %w", err)
	}

	// Convert to our type alias
	metrics := make([]NodeMetrics, len(nodeMetricsList.Items))
	for i, item := range nodeMetricsList.Items {
		metrics[i] = item
	}

	return metrics, nil
}

// GetPodMetrics retrieves pod metrics from the cluster
func (c *ClusterClient) GetPodMetrics(ctx context.Context) ([]PodMetrics, error) {
	if c.MetricsClient == nil {
		return nil, fmt.Errorf("metrics client not available")
	}

	podMetricsList, err := c.MetricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	// Convert to our type alias
	metrics := make([]PodMetrics, len(podMetricsList.Items))
	for i, item := range podMetricsList.Items {
		metrics[i] = item
	}

	return metrics, nil
}

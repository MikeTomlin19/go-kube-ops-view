package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/tools/clientcmd"
)

// StaticDiscoverer discovers clusters from a static list of URLs
type StaticDiscoverer struct {
	clusterURLs []string
}

// NewStaticDiscoverer creates a new StaticDiscoverer
func NewStaticDiscoverer(clusterURLs []string) *StaticDiscoverer {
	return &StaticDiscoverer{
		clusterURLs: clusterURLs,
	}
}

// DiscoverClusters implements ClusterDiscoverer for static cluster URLs
func (d *StaticDiscoverer) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	var configs []ClusterConfig

	for i, clusterURL := range d.clusterURLs {
		// Parse URL to extract host for ID generation
		parsedURL, err := url.Parse(clusterURL)
		if err != nil {
			return nil, fmt.Errorf("invalid cluster URL %s: %w", clusterURL, err)
		}

		// Generate cluster ID from URL
		clusterID := fmt.Sprintf("cluster-%d", i+1)
		if parsedURL.Host != "" {
			// Use hostname as ID if available
			clusterID = strings.ReplaceAll(parsedURL.Host, ":", "-")
		}

		config := ClusterConfig{
			ID:        clusterID,
			APIServer: clusterURL,
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// KubeconfigDiscoverer discovers clusters from kubeconfig files
type KubeconfigDiscoverer struct {
	kubeconfigPath string
	contexts       []string // If empty, use all contexts
}

// NewKubeconfigDiscoverer creates a new KubeconfigDiscoverer
func NewKubeconfigDiscoverer(kubeconfigPath string, contexts []string) *KubeconfigDiscoverer {
	return &KubeconfigDiscoverer{
		kubeconfigPath: kubeconfigPath,
		contexts:       contexts,
	}
}

// DiscoverClusters implements ClusterDiscoverer for kubeconfig files
func (d *KubeconfigDiscoverer) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	// Load kubeconfig
	config, err := clientcmd.LoadFromFile(d.kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", d.kubeconfigPath, err)
	}

	var configs []ClusterConfig
	contextsToUse := d.contexts

	// If no specific contexts provided, use all available contexts
	if len(contextsToUse) == 0 {
		for contextName := range config.Contexts {
			contextsToUse = append(contextsToUse, contextName)
		}
	}

	for _, contextName := range contextsToUse {
		contextConfig, exists := config.Contexts[contextName]
		if !exists {
			return nil, fmt.Errorf("context %s not found in kubeconfig", contextName)
		}

		cluster, exists := config.Clusters[contextConfig.Cluster]
		if !exists {
			return nil, fmt.Errorf("cluster %s not found in kubeconfig", contextConfig.Cluster)
		}

		clusterConfig := ClusterConfig{
			ID:             contextName,
			APIServer:      cluster.Server,
			KubeconfigPath: d.kubeconfigPath,
			Context:        contextName,
		}

		configs = append(configs, clusterConfig)
	}

	return configs, nil
}

// RegistryDiscoverer discovers clusters from a cluster registry endpoint
type RegistryDiscoverer struct {
	registryURL string
	httpClient  *http.Client
}

// NewRegistryDiscoverer creates a new RegistryDiscoverer
func NewRegistryDiscoverer(registryURL string) *RegistryDiscoverer {
	return &RegistryDiscoverer{
		registryURL: registryURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegistryCluster represents a cluster entry from the registry
type RegistryCluster struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	APIServer string `json:"api_server"`
	Token     string `json:"token,omitempty"`
	CAData    string `json:"ca_data,omitempty"`
}

// RegistryResponse represents the response from the cluster registry
type RegistryResponse struct {
	Clusters []RegistryCluster `json:"clusters"`
}

// DiscoverClusters implements ClusterDiscoverer for cluster registry
func (d *RegistryDiscoverer) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", d.registryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to registry: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clusters from registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry response: %w", err)
	}

	var registryResp RegistryResponse
	if err := json.Unmarshal(body, &registryResp); err != nil {
		return nil, fmt.Errorf("failed to parse registry response: %w", err)
	}

	var configs []ClusterConfig
	for _, cluster := range registryResp.Clusters {
		config := ClusterConfig{
			ID:        cluster.ID,
			APIServer: cluster.APIServer,
			Token:     cluster.Token,
		}

		// Handle CA data if provided
		if cluster.CAData != "" {
			// In a real implementation, you might want to write this to a temp file
			// For now, we'll store it in the config (this would need additional handling)
			config.CAFile = cluster.CAData
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// MultiDiscoverer combines multiple discovery methods
type MultiDiscoverer struct {
	discoverers []ClusterDiscoverer
}

// NewMultiDiscoverer creates a new MultiDiscoverer
func NewMultiDiscoverer(discoverers ...ClusterDiscoverer) *MultiDiscoverer {
	return &MultiDiscoverer{
		discoverers: discoverers,
	}
}

// DiscoverClusters implements ClusterDiscoverer by combining results from multiple discoverers
func (d *MultiDiscoverer) DiscoverClusters(ctx context.Context) ([]ClusterConfig, error) {
	var allConfigs []ClusterConfig
	seenIDs := make(map[string]bool)

	for _, discoverer := range d.discoverers {
		configs, err := discoverer.DiscoverClusters(ctx)
		if err != nil {
			// Log error but continue with other discoverers
			continue
		}

		for _, config := range configs {
			// Avoid duplicate cluster IDs
			if !seenIDs[config.ID] {
				allConfigs = append(allConfigs, config)
				seenIDs[config.ID] = true
			}
		}
	}

	return allConfigs, nil
}

// DiscovererFactory creates discoverers based on configuration
type DiscovererFactory struct{}

// NewDiscovererFactory creates a new DiscovererFactory
func NewDiscovererFactory() *DiscovererFactory {
	return &DiscovererFactory{}
}

// CreateDiscoverer creates appropriate discoverers based on the provided configuration
func (f *DiscovererFactory) CreateDiscoverer(
	clusterURLs []string,
	kubeconfigPath string,
	contexts []string,
	registryURL string,
) ClusterDiscoverer {
	var discoverers []ClusterDiscoverer

	// Add static discoverer if URLs are provided
	if len(clusterURLs) > 0 {
		discoverers = append(discoverers, NewStaticDiscoverer(clusterURLs))
	}

	// Add kubeconfig discoverer if path is provided
	if kubeconfigPath != "" {
		// Expand tilde in path
		if strings.HasPrefix(kubeconfigPath, "~/") {
			kubeconfigPath = filepath.Join(homeDir(), kubeconfigPath[2:])
		}
		discoverers = append(discoverers, NewKubeconfigDiscoverer(kubeconfigPath, contexts))
	}

	// Add registry discoverer if URL is provided
	if registryURL != "" {
		discoverers = append(discoverers, NewRegistryDiscoverer(registryURL))
	}

	// If only one discoverer, return it directly
	if len(discoverers) == 1 {
		return discoverers[0]
	}

	// If multiple discoverers, use MultiDiscoverer
	if len(discoverers) > 1 {
		return NewMultiDiscoverer(discoverers...)
	}

	// Default to empty static discoverer
	return NewStaticDiscoverer([]string{})
}

// homeDir returns the user's home directory
func homeDir() string {
	if h := clientcmd.NewDefaultPathOptions().GlobalFile; h != "" {
		return filepath.Dir(h)
	}
	return ""
}

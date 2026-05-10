package cluster

import (
	"context"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

// ClusterConfig represents the configuration for a single cluster
type ClusterConfig struct {
	ID             string `json:"id"`
	APIServer      string `json:"api_server"`
	KubeconfigPath string `json:"kubeconfig_path,omitempty"`
	Context        string `json:"context,omitempty"`
	Token          string `json:"token,omitempty"`
	CertFile       string `json:"cert_file,omitempty"`
	KeyFile        string `json:"key_file,omitempty"`
	CAFile         string `json:"ca_file,omitempty"`
	Insecure       bool   `json:"insecure,omitempty"`
}

// ClusterClient wraps Kubernetes client interfaces for a single cluster
type ClusterClient struct {
	ID            string
	APIServer     string
	Client        kubernetes.Interface
	MetricsClient versioned.Interface
	config        *ClusterConfig
}

// ClusterDiscoverer defines the interface for discovering clusters
type ClusterDiscoverer interface {
	DiscoverClusters(ctx context.Context) ([]ClusterConfig, error)
}

// ClusterStatus represents the status of a cluster connection
type ClusterStatus struct {
	ID        string    `json:"id"`
	Available bool      `json:"available"`
	LastSeen  time.Time `json:"last_seen"`
	Error     string    `json:"error,omitempty"`
	Version   string    `json:"version,omitempty"`
	NodeCount int       `json:"node_count,omitempty"`
}

// AuthMethod represents different authentication methods for clusters
type AuthMethod int

const (
	AuthMethodToken AuthMethod = iota
	AuthMethodCertificate
	AuthMethodKubeconfig
	AuthMethodServiceAccount
)

// DiscoveryMethod represents different cluster discovery methods
type DiscoveryMethod int

const (
	DiscoveryMethodStatic DiscoveryMethod = iota
	DiscoveryMethodKubeconfig
	DiscoveryMethodRegistry
)

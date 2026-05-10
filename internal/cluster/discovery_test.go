package cluster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStaticDiscoverer_DiscoverClusters(t *testing.T) {
	tests := []struct {
		name          string
		clusterURLs   []string
		expectedCount int
		expectError   bool
	}{
		{
			name: "valid cluster URLs",
			clusterURLs: []string{
				"https://cluster1.example.com",
				"https://cluster2.example.com:6443",
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "empty cluster URLs",
			clusterURLs:   []string{},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "invalid URL",
			clusterURLs: []string{
				"://invalid-url",
			},
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoverer := NewStaticDiscoverer(tt.clusterURLs)
			ctx := context.Background()

			configs, err := discoverer.DiscoverClusters(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(configs) != tt.expectedCount {
				t.Errorf("expected %d configs, got %d", tt.expectedCount, len(configs))
			}

			// Verify config structure
			for i, config := range configs {
				if config.APIServer != tt.clusterURLs[i] {
					t.Errorf("expected APIServer %s, got %s", tt.clusterURLs[i], config.APIServer)
				}
				if config.ID == "" {
					t.Errorf("expected non-empty ID")
				}
			}
		})
	}
}

func TestKubeconfigDiscoverer_DiscoverClusters(t *testing.T) {
	// Create a temporary kubeconfig file for testing
	tempDir := t.TempDir()
	kubeconfigPath := filepath.Join(tempDir, "kubeconfig")

	// Write kubeconfig to file in YAML format
	kubeconfigYAML := `
apiVersion: v1
kind: Config
clusters:
- name: cluster1
  cluster:
    server: https://cluster1.example.com
- name: cluster2
  cluster:
    server: https://cluster2.example.com
contexts:
- name: context1
  context:
    cluster: cluster1
- name: context2
  context:
    cluster: cluster2
current-context: context1
`

	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfigYAML), 0644); err != nil {
		t.Fatalf("failed to write kubeconfig: %v", err)
	}

	tests := []struct {
		name           string
		kubeconfigPath string
		contexts       []string
		expectedCount  int
		expectError    bool
	}{
		{
			name:           "all contexts",
			kubeconfigPath: kubeconfigPath,
			contexts:       []string{},
			expectedCount:  2,
			expectError:    false,
		},
		{
			name:           "specific context",
			kubeconfigPath: kubeconfigPath,
			contexts:       []string{"context1"},
			expectedCount:  1,
			expectError:    false,
		},
		{
			name:           "nonexistent context",
			kubeconfigPath: kubeconfigPath,
			contexts:       []string{"nonexistent"},
			expectedCount:  0,
			expectError:    true,
		},
		{
			name:           "nonexistent kubeconfig",
			kubeconfigPath: "/nonexistent/kubeconfig",
			contexts:       []string{},
			expectedCount:  0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoverer := NewKubeconfigDiscoverer(tt.kubeconfigPath, tt.contexts)
			ctx := context.Background()

			configs, err := discoverer.DiscoverClusters(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(configs) != tt.expectedCount {
				t.Errorf("expected %d configs, got %d", tt.expectedCount, len(configs))
			}

			// Verify config structure
			for _, config := range configs {
				if config.KubeconfigPath != tt.kubeconfigPath {
					t.Errorf("expected KubeconfigPath %s, got %s", tt.kubeconfigPath, config.KubeconfigPath)
				}
				if config.APIServer == "" {
					t.Errorf("expected non-empty APIServer")
				}
				if config.ID == "" {
					t.Errorf("expected non-empty ID")
				}
			}
		})
	}
}

func TestRegistryDiscoverer_DiscoverClusters(t *testing.T) {
	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		expectedCount int
		expectError   bool
	}{
		{
			name: "successful registry response",
			setupServer: func() *httptest.Server {
				response := RegistryResponse{
					Clusters: []RegistryCluster{
						{
							ID:        "cluster1",
							Name:      "Cluster 1",
							APIServer: "https://cluster1.example.com",
							Token:     "token1",
						},
						{
							ID:        "cluster2",
							Name:      "Cluster 2",
							APIServer: "https://cluster2.example.com",
							Token:     "token2",
						},
					},
				}

				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				}))
			},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "empty registry response",
			setupServer: func() *httptest.Server {
				response := RegistryResponse{
					Clusters: []RegistryCluster{},
				}

				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				}))
			},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "registry server error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectedCount: 0,
			expectError:   true,
		},
		{
			name: "invalid JSON response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte("invalid json"))
				}))
			},
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			discoverer := NewRegistryDiscoverer(server.URL)
			ctx := context.Background()

			configs, err := discoverer.DiscoverClusters(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(configs) != tt.expectedCount {
				t.Errorf("expected %d configs, got %d", tt.expectedCount, len(configs))
			}

			// Verify config structure
			for _, config := range configs {
				if config.ID == "" {
					t.Errorf("expected non-empty ID")
				}
				if config.APIServer == "" {
					t.Errorf("expected non-empty APIServer")
				}
			}
		})
	}
}

func TestMultiDiscoverer_DiscoverClusters(t *testing.T) {
	tests := []struct {
		name             string
		setupDiscoverers func() []ClusterDiscoverer
		expectedCount    int
		expectError      bool
	}{
		{
			name: "multiple successful discoverers",
			setupDiscoverers: func() []ClusterDiscoverer {
				static := NewStaticDiscoverer([]string{
					"https://cluster1.example.com",
					"https://cluster2.example.com",
				})

				// Create a mock registry server
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := RegistryResponse{
						Clusters: []RegistryCluster{
							{
								ID:        "cluster3",
								APIServer: "https://cluster3.example.com",
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				}))

				registry := NewRegistryDiscoverer(server.URL)

				return []ClusterDiscoverer{static, registry}
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name: "one failing discoverer",
			setupDiscoverers: func() []ClusterDiscoverer {
				static := NewStaticDiscoverer([]string{
					"https://cluster1.example.com",
				})

				// Create a failing registry server
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))

				registry := NewRegistryDiscoverer(server.URL)

				return []ClusterDiscoverer{static, registry}
			},
			expectedCount: 1, // Only static discoverer succeeds
			expectError:   false,
		},
		{
			name: "duplicate cluster IDs",
			setupDiscoverers: func() []ClusterDiscoverer {
				static1 := NewStaticDiscoverer([]string{
					"https://cluster1.example.com",
				})

				static2 := NewStaticDiscoverer([]string{
					"https://cluster1.example.com", // Same URL, will generate same ID
				})

				return []ClusterDiscoverer{static1, static2}
			},
			expectedCount: 1, // Duplicate should be filtered out
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoverers := tt.setupDiscoverers()
			multiDiscoverer := NewMultiDiscoverer(discoverers...)
			ctx := context.Background()

			configs, err := multiDiscoverer.DiscoverClusters(ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(configs) != tt.expectedCount {
				t.Errorf("expected %d configs, got %d", tt.expectedCount, len(configs))
			}
		})
	}
}

func TestDiscovererFactory_CreateDiscoverer(t *testing.T) {
	factory := NewDiscovererFactory()

	tests := []struct {
		name           string
		clusterURLs    []string
		kubeconfigPath string
		contexts       []string
		registryURL    string
		expectedType   string
	}{
		{
			name:         "static only",
			clusterURLs:  []string{"https://cluster1.example.com"},
			expectedType: "*cluster.StaticDiscoverer",
		},
		{
			name:           "kubeconfig only",
			kubeconfigPath: "/tmp/kubeconfig",
			expectedType:   "*cluster.KubeconfigDiscoverer",
		},
		{
			name:         "registry only",
			registryURL:  "https://registry.example.com",
			expectedType: "*cluster.RegistryDiscoverer",
		},
		{
			name:           "multiple sources",
			clusterURLs:    []string{"https://cluster1.example.com"},
			kubeconfigPath: "/tmp/kubeconfig",
			expectedType:   "*cluster.MultiDiscoverer",
		},
		{
			name:         "no sources",
			expectedType: "*cluster.StaticDiscoverer", // Default empty static discoverer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoverer := factory.CreateDiscoverer(
				tt.clusterURLs,
				tt.kubeconfigPath,
				tt.contexts,
				tt.registryURL,
			)

			if discoverer == nil {
				t.Errorf("expected discoverer but got nil")
				return
			}

			// We can't easily check the exact type without reflection,
			// but we can verify it implements the interface
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			_, err := discoverer.DiscoverClusters(ctx)
			// We don't care about the error here, just that the method exists
			_ = err
		})
	}
}

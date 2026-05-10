package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestNewClusterClient(t *testing.T) {
	tests := []struct {
		name        string
		config      *ClusterConfig
		expectError bool
	}{
		{
			name: "valid config with token",
			config: &ClusterConfig{
				ID:        "test-cluster",
				APIServer: "https://kubernetes.example.com",
				Token:     "test-token",
			},
			expectError: false,
		},
		{
			name: "valid config with kubeconfig",
			config: &ClusterConfig{
				ID:             "test-cluster",
				APIServer:      "https://kubernetes.example.com",
				KubeconfigPath: "/tmp/nonexistent",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClusterClient(tt.config)

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

			if client == nil {
				t.Errorf("expected client but got nil")
				return
			}

			if client.ID != tt.config.ID {
				t.Errorf("expected ID %s, got %s", tt.config.ID, client.ID)
			}
		})
	}
}

func TestClusterClient_TestConnection(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func() *ClusterClient
		expectError bool
	}{
		{
			name: "successful connection",
			setupClient: func() *ClusterClient {
				fakeClient := fake.NewSimpleClientset()
				return &ClusterClient{
					ID:     "test-cluster",
					Client: fakeClient,
				}
			},
			expectError: false,
		},
		{
			name: "connection failure",
			setupClient: func() *ClusterClient {
				fakeClient := fake.NewSimpleClientset()
				fakeClient.PrependReactor("list", "namespaces", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("connection failed"))
				})
				return &ClusterClient{
					ID:     "test-cluster",
					Client: fakeClient,
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			ctx := context.Background()

			err := client.TestConnection(ctx)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestClusterClient_GetNodes(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() *ClusterClient
		expectedNodes int
		expectError   bool
	}{
		{
			name: "successful node retrieval",
			setupClient: func() *ClusterClient {
				node1 := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
				}
				node2 := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-2",
					},
				}
				fakeClient := fake.NewSimpleClientset(node1, node2)
				return &ClusterClient{
					ID:     "test-cluster",
					Client: fakeClient,
				}
			},
			expectedNodes: 2,
			expectError:   false,
		},
		{
			name: "no nodes",
			setupClient: func() *ClusterClient {
				fakeClient := fake.NewSimpleClientset()
				return &ClusterClient{
					ID:     "test-cluster",
					Client: fakeClient,
				}
			},
			expectedNodes: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			ctx := context.Background()

			nodes, err := client.GetNodes(ctx)

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

			if len(nodes.Items) != tt.expectedNodes {
				t.Errorf("expected %d nodes, got %d", tt.expectedNodes, len(nodes.Items))
			}
		})
	}
}

func TestClusterClient_GetClusterStatus(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func() *ClusterClient
		expectAvailable bool
	}{
		{
			name: "healthy cluster",
			setupClient: func() *ClusterClient {
				node := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
				}
				fakeClient := fake.NewSimpleClientset(node)
				fakeDiscovery := &fakediscovery.FakeDiscovery{
					Fake: &fakeClient.Fake,
				}
				fakeDiscovery.FakedServerVersion = &version.Info{
					GitVersion: "v1.28.0",
				}
				fakeClientWithDiscovery := &fakeClientWithCustomDiscovery{
					Clientset: fakeClient,
					discovery: fakeDiscovery,
				}
				return &ClusterClient{
					ID:     "test-cluster",
					Client: fakeClientWithDiscovery,
				}
			},
			expectAvailable: true,
		},
		{
			name: "unhealthy cluster",
			setupClient: func() *ClusterClient {
				fakeClient := fake.NewSimpleClientset()
				fakeClient.PrependReactor("list", "namespaces", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("connection failed"))
				})
				return &ClusterClient{
					ID:     "test-cluster",
					Client: fakeClient,
				}
			},
			expectAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			ctx := context.Background()

			status, err := client.GetClusterStatus(ctx)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if status == nil {
				t.Errorf("expected status but got nil")
				return
			}

			if status.Available != tt.expectAvailable {
				t.Errorf("expected available %v, got %v", tt.expectAvailable, status.Available)
			}

			if status.ID != client.ID {
				t.Errorf("expected ID %s, got %s", client.ID, status.ID)
			}

			if time.Since(status.LastSeen) > time.Minute {
				t.Errorf("LastSeen timestamp is too old: %v", status.LastSeen)
			}
		})
	}
}

// Helper type to wrap fake client with custom discovery
type fakeClientWithCustomDiscovery struct {
	*fake.Clientset
	discovery discovery.DiscoveryInterface
}

func (f *fakeClientWithCustomDiscovery) Discovery() discovery.DiscoveryInterface {
	return f.discovery
}

package cluster

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestClusterClient_GetNodeMetrics(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func() *ClusterClient
		expectedMetrics int
		expectError     bool
	}{
		{
			name: "successful node metrics retrieval",
			setupClient: func() *ClusterClient {
				nodeMetrics1 := &v1beta1.NodeMetrics{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				}

				fakeMetricsClient := metricsfake.NewSimpleClientset(nodeMetrics1)
				return &ClusterClient{
					ID:            "test-cluster",
					MetricsClient: fakeMetricsClient,
				}
			},
			expectedMetrics: 0,
			expectError:     false,
		},
		{
			name: "no metrics client",
			setupClient: func() *ClusterClient {
				return &ClusterClient{
					ID:            "test-cluster",
					MetricsClient: nil,
				}
			},
			expectedMetrics: 0,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			ctx := context.Background()

			metrics, err := client.GetNodeMetrics(ctx)

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

			if len(metrics) != tt.expectedMetrics {
				t.Errorf("expected %d metrics, got %d", tt.expectedMetrics, len(metrics))
			}
		})
	}
}

func TestClusterClient_IsMetricsAvailable(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func() *ClusterClient
		expectAvailable bool
	}{
		{
			name: "metrics available",
			setupClient: func() *ClusterClient {
				nodeMetrics := &v1beta1.NodeMetrics{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
				}
				fakeMetricsClient := metricsfake.NewSimpleClientset(nodeMetrics)
				return &ClusterClient{
					ID:            "test-cluster",
					MetricsClient: fakeMetricsClient,
				}
			},
			expectAvailable: true,
		},
		{
			name: "no metrics client",
			setupClient: func() *ClusterClient {
				return &ClusterClient{
					ID:            "test-cluster",
					MetricsClient: nil,
				}
			},
			expectAvailable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupClient()
			ctx := context.Background()

			available := client.IsMetricsAvailable(ctx)

			if available != tt.expectAvailable {
				t.Errorf("expected available %v, got %v", tt.expectAvailable, available)
			}
		})
	}
}

package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCPU(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *ParsedResource
		expectError bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:  "millicores",
			input: "100m",
			expected: &ParsedResource{
				Raw:       "100m",
				Value:     0.1,
				Unit:      "cores",
				MilliUnit: 100,
			},
		},
		{
			name:  "decimal cores",
			input: "0.5",
			expected: &ParsedResource{
				Raw:       "0.5",
				Value:     0.5,
				Unit:      "cores",
				MilliUnit: 500,
			},
		},
		{
			name:  "whole cores",
			input: "2",
			expected: &ParsedResource{
				Raw:       "2",
				Value:     2.0,
				Unit:      "cores",
				MilliUnit: 2000,
			},
		},
		{
			name:  "kubernetes quantity format",
			input: "1500m",
			expected: &ParsedResource{
				Raw:       "1500m",
				Value:     1.5,
				Unit:      "cores",
				MilliUnit: 1500,
			},
		},
		{
			name:        "invalid format",
			input:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCPU(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *ParsedResource
		expectError bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:  "bytes",
			input: "1000000000",
			expected: &ParsedResource{
				Raw:       "1000000000",
				Value:     1000000000,
				Unit:      "bytes",
				MilliUnit: 1000000000,
			},
		},
		{
			name:  "kubernetes Mi format",
			input: "128Mi",
			expected: &ParsedResource{
				Raw:       "128Mi",
				Value:     134217728, // 128 * 1024 * 1024
				Unit:      "bytes",
				MilliUnit: 134217728,
			},
		},
		{
			name:  "kubernetes Gi format",
			input: "1Gi",
			expected: &ParsedResource{
				Raw:       "1Gi",
				Value:     1073741824, // 1024^3
				Unit:      "bytes",
				MilliUnit: 1073741824,
			},
		},
		{
			name:        "invalid format",
			input:       "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseMemory(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		name       string
		milliCores int64
		expected   string
	}{
		{
			name:       "less than 1 core",
			milliCores: 500,
			expected:   "500m",
		},
		{
			name:       "exactly 1 core",
			milliCores: 1000,
			expected:   "1.0",
		},
		{
			name:       "more than 1 core",
			milliCores: 1500,
			expected:   "1.5",
		},
		{
			name:       "zero cores",
			milliCores: 0,
			expected:   "0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCPU(tt.milliCores)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512",
		},
		{
			name:     "kilobytes",
			bytes:    2048, // 2 KB
			expected: "2.0Ki",
		},
		{
			name:     "megabytes",
			bytes:    134217728, // 128 MB
			expected: "128.0Mi",
		},
		{
			name:     "gigabytes",
			bytes:    1073741824, // 1 GB
			expected: "1.0Gi",
		},
		{
			name:     "terabytes",
			bytes:    1099511627776, // 1 TB
			expected: "1.0Ti",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMemory(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClusterDataValidation(t *testing.T) {
	tests := []struct {
		name        string
		clusterData *ClusterData
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid cluster data",
			clusterData: &ClusterData{
				ID:             "test-cluster",
				APIServerURL:   "https://api.test.com",
				Nodes:          make(map[string]*Node),
				UnassignedPods: make(map[string]*Pod),
			},
			expectError: false,
		},
		{
			name: "empty cluster ID",
			clusterData: &ClusterData{
				APIServerURL: "https://api.test.com",
			},
			expectError: true,
			errorMsg:    "cluster ID cannot be empty",
		},
		{
			name: "empty API server URL",
			clusterData: &ClusterData{
				ID: "test-cluster",
			},
			expectError: true,
			errorMsg:    "API server URL cannot be empty",
		},
		{
			name: "nil node",
			clusterData: &ClusterData{
				ID:           "test-cluster",
				APIServerURL: "https://api.test.com",
				Nodes: map[string]*Node{
					"node1": nil,
				},
			},
			expectError: true,
			errorMsg:    "node node1 cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.clusterData.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNodeValidation(t *testing.T) {
	tests := []struct {
		name        string
		node        *Node
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid node",
			node: &Node{
				Name: "test-node",
				Pods: make(map[string]*Pod),
			},
			expectError: false,
		},
		{
			name: "empty node name",
			node: &Node{
				Pods: make(map[string]*Pod),
			},
			expectError: true,
			errorMsg:    "node name cannot be empty",
		},
		{
			name: "nil pod",
			node: &Node{
				Name: "test-node",
				Pods: map[string]*Pod{
					"pod1": nil,
				},
			},
			expectError: true,
			errorMsg:    "pod pod1 cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.node.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPodValidation(t *testing.T) {
	tests := []struct {
		name        string
		pod         *Pod
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid pod",
			pod: &Pod{
				Name:      "test-pod",
				Namespace: "default",
				Containers: []Container{
					{
						Name:  "container1",
						Image: "nginx:latest",
					},
				},
			},
			expectError: false,
		},
		{
			name: "empty pod name",
			pod: &Pod{
				Namespace: "default",
			},
			expectError: true,
			errorMsg:    "pod name cannot be empty",
		},
		{
			name: "empty namespace",
			pod: &Pod{
				Name: "test-pod",
			},
			expectError: true,
			errorMsg:    "pod namespace cannot be empty",
		},
		{
			name: "invalid container",
			pod: &Pod{
				Name:      "test-pod",
				Namespace: "default",
				Containers: []Container{
					{
						Name: "container1",
						// Missing image
					},
				},
			},
			expectError: true,
			errorMsg:    "invalid container 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pod.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainerValidation(t *testing.T) {
	tests := []struct {
		name        string
		container   *Container
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid container",
			container: &Container{
				Name:  "test-container",
				Image: "nginx:latest",
			},
			expectError: false,
		},
		{
			name: "empty container name",
			container: &Container{
				Image: "nginx:latest",
			},
			expectError: true,
			errorMsg:    "container name cannot be empty",
		},
		{
			name: "empty image",
			container: &Container{
				Name: "test-container",
			},
			expectError: true,
			errorMsg:    "container image cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.container.Validate()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPodIsSystemPod(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		expected  bool
	}{
		{
			name:      "kube-system namespace",
			namespace: "kube-system",
			expected:  true,
		},
		{
			name:      "kube-public namespace",
			namespace: "kube-public",
			expected:  true,
		},
		{
			name:      "default namespace",
			namespace: "default",
			expected:  true,
		},
		{
			name:      "custom namespace",
			namespace: "my-app",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &Pod{Namespace: tt.namespace}
			result := pod.IsSystemPod()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPodResourceCalculations(t *testing.T) {
	pod := &Pod{
		Name:      "test-pod",
		Namespace: "default",
		Containers: []Container{
			{
				Name:  "container1",
				Image: "nginx:latest",
				Resources: ContainerResources{
					Requests: ResourceList{
						"cpu":    "100m",
						"memory": "128Mi",
					},
					Limits: ResourceList{
						"cpu":    "200m",
						"memory": "256Mi",
					},
				},
			},
			{
				Name:  "container2",
				Image: "redis:latest",
				Resources: ContainerResources{
					Requests: ResourceList{
						"cpu":    "50m",
						"memory": "64Mi",
					},
					Limits: ResourceList{
						"cpu":    "100m",
						"memory": "128Mi",
					},
				},
			},
		},
	}

	t.Run("CPU requests", func(t *testing.T) {
		total, err := pod.GetTotalCPURequests()
		require.NoError(t, err)
		assert.Equal(t, int64(150), total) // 100m + 50m
	})

	t.Run("Memory requests", func(t *testing.T) {
		total, err := pod.GetTotalMemoryRequests()
		require.NoError(t, err)
		expected := int64(134217728 + 67108864) // 128Mi + 64Mi in bytes
		assert.Equal(t, expected, total)
	})

	t.Run("CPU limits", func(t *testing.T) {
		total, err := pod.GetTotalCPULimits()
		require.NoError(t, err)
		assert.Equal(t, int64(300), total) // 200m + 100m
	})

	t.Run("Memory limits", func(t *testing.T) {
		total, err := pod.GetTotalMemoryLimits()
		require.NoError(t, err)
		expected := int64(268435456 + 134217728) // 256Mi + 128Mi in bytes
		assert.Equal(t, expected, total)
	})
}

func TestJSONMarshaling(t *testing.T) {
	now := time.Now()

	clusterData := &ClusterData{
		ID:           "test-cluster",
		APIServerURL: "https://api.test.com",
		LastUpdate:   now,
		Nodes: map[string]*Node{
			"node1": {
				Name: "node1",
				Labels: map[string]string{
					"kubernetes.io/hostname": "node1",
				},
				Status: NodeStatus{
					Ready: true,
				},
				Pods: map[string]*Pod{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
						Phase:     "Running",
						Ready:     true,
						StartTime: &now,
						Containers: []Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
								Ready: true,
								Resources: ContainerResources{
									Requests: ResourceList{
										"cpu":    "100m",
										"memory": "128Mi",
									},
								},
							},
						},
					},
				},
			},
		},
		UnassignedPods: make(map[string]*Pod),
	}

	t.Run("marshal to JSON", func(t *testing.T) {
		data, err := json.Marshal(clusterData)
		require.NoError(t, err)
		assert.NotEmpty(t, data)

		// Verify it contains expected fields
		var result map[string]interface{}
		err = json.Unmarshal(data, &result)
		require.NoError(t, err)

		assert.Equal(t, "test-cluster", result["id"])
		assert.Equal(t, "https://api.test.com", result["api_server_url"])
		assert.NotNil(t, result["nodes"])
		assert.NotNil(t, result["unassigned_pods"])
	})

	t.Run("unmarshal from JSON", func(t *testing.T) {
		// First marshal to get JSON
		originalData, err := json.Marshal(clusterData)
		require.NoError(t, err)

		// Then unmarshal back
		var unmarshaled ClusterData
		err = json.Unmarshal(originalData, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, clusterData.ID, unmarshaled.ID)
		assert.Equal(t, clusterData.APIServerURL, unmarshaled.APIServerURL)
		assert.Len(t, unmarshaled.Nodes, 1)

		node := unmarshaled.Nodes["node1"]
		require.NotNil(t, node)
		assert.Equal(t, "node1", node.Name)
		assert.True(t, node.Status.Ready)

		pod := node.Pods["pod1"]
		require.NotNil(t, pod)
		assert.Equal(t, "pod1", pod.Name)
		assert.Equal(t, "default", pod.Namespace)
		assert.Equal(t, "Running", pod.Phase)
		assert.True(t, pod.Ready)
		assert.Len(t, pod.Containers, 1)

		container := pod.Containers[0]
		assert.Equal(t, "nginx", container.Name)
		assert.Equal(t, "nginx:latest", container.Image)
		assert.True(t, container.Ready)
		assert.Equal(t, "100m", container.Resources.Requests["cpu"])
		assert.Equal(t, "128Mi", container.Resources.Requests["memory"])
	})
}

func TestCloning(t *testing.T) {
	now := time.Now()

	original := &ClusterData{
		ID:           "test-cluster",
		APIServerURL: "https://api.test.com",
		LastUpdate:   now,
		Nodes: map[string]*Node{
			"node1": {
				Name: "node1",
				Labels: map[string]string{
					"test": "value",
				},
				Pods: map[string]*Pod{
					"pod1": {
						Name:      "pod1",
						Namespace: "default",
						Labels: map[string]string{
							"app": "test",
						},
						StartTime: &now,
						Containers: []Container{
							{
								Name:  "nginx",
								Image: "nginx:latest",
								Resources: ContainerResources{
									Requests: ResourceList{
										"cpu": "100m",
									},
								},
							},
						},
					},
				},
			},
		},
		UnassignedPods: make(map[string]*Pod),
	}

	t.Run("clone cluster data", func(t *testing.T) {
		clone := original.Clone()

		// Verify it's a deep copy
		assert.Equal(t, original.ID, clone.ID)
		assert.Equal(t, original.APIServerURL, clone.APIServerURL)
		assert.Equal(t, original.LastUpdate, clone.LastUpdate)

		// Modify original and ensure clone is not affected
		original.ID = "modified"
		assert.Equal(t, "test-cluster", clone.ID)

		// Modify node labels in original
		original.Nodes["node1"].Labels["test"] = "modified"
		assert.Equal(t, "value", clone.Nodes["node1"].Labels["test"])

		// Modify pod in original
		original.Nodes["node1"].Pods["pod1"].Name = "modified"
		assert.Equal(t, "pod1", clone.Nodes["node1"].Pods["pod1"].Name)
	})
}

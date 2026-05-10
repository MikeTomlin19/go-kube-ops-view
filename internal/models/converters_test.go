package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestConvertNode(t *testing.T) {
	now := metav1.Now()

	k8sNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
			Labels: map[string]string{
				"kubernetes.io/hostname":         "test-node",
				"node-role.kubernetes.io/worker": "",
			},
			Annotations: map[string]string{
				"test-annotation": "test-value",
			},
		},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3800m"),
				corev1.ResourceMemory: resource.MustParse("7Gi"),
			},
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "10.0.0.1",
				},
				{
					Type:    corev1.NodeHostName,
					Address: "test-node",
				},
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
					LastHeartbeatTime:  now,
					LastTransitionTime: now,
					Reason:             "KubeletReady",
					Message:            "kubelet is posting ready status",
				},
			},
		},
	}

	t.Run("convert valid node", func(t *testing.T) {
		node := ConvertNode(k8sNode)

		require.NotNil(t, node)
		assert.Equal(t, "test-node", node.Name)
		assert.Equal(t, "test-node", node.Labels["kubernetes.io/hostname"])
		assert.Equal(t, "", node.Labels["node-role.kubernetes.io/worker"])
		assert.Equal(t, "test-value", node.Annotations["test-annotation"])
		assert.True(t, node.Status.Ready)

		// Check capacity
		assert.Equal(t, "4", node.Capacity["cpu"])
		assert.Equal(t, "8Gi", node.Capacity["memory"])

		// Check allocatable
		assert.Equal(t, "3800m", node.Allocatable["cpu"])
		assert.Equal(t, "7Gi", node.Allocatable["memory"])

		// Check addresses
		require.Len(t, node.Status.Addresses, 2)
		assert.Equal(t, "InternalIP", node.Status.Addresses[0].Type)
		assert.Equal(t, "10.0.0.1", node.Status.Addresses[0].Address)

		// Check conditions
		require.Len(t, node.Status.Conditions, 1)
		assert.Equal(t, "Ready", node.Status.Conditions[0].Type)
		assert.Equal(t, "True", node.Status.Conditions[0].Status)
	})

	t.Run("convert nil node", func(t *testing.T) {
		node := ConvertNode(nil)
		assert.Nil(t, node)
	})
}

func TestConvertPod(t *testing.T) {
	now := metav1.Now()
	startTime := metav1.NewTime(time.Now().Add(-5 * time.Minute))

	k8sPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "nginx",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "Deployment",
					Name: "nginx-deployment",
				},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
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
			Phase:     corev1.PodRunning,
			StartTime: &startTime,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "nginx",
					Ready:        true,
					RestartCount: 0,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: now,
						},
					},
				},
			},
		},
	}

	t.Run("convert valid pod", func(t *testing.T) {
		pod := ConvertPod(k8sPod)

		require.NotNil(t, pod)
		assert.Equal(t, "test-pod", pod.Name)
		assert.Equal(t, "default", pod.Namespace)
		assert.Equal(t, "nginx", pod.Labels["app"])
		assert.Equal(t, "Running", pod.Phase)
		assert.Equal(t, "test-node", pod.NodeName)
		assert.True(t, pod.Ready)
		assert.Equal(t, "Deployment", pod.OwnerKind)
		assert.Equal(t, "nginx-deployment", pod.OwnerName)

		// Check start time
		require.NotNil(t, pod.StartTime)
		assert.Equal(t, startTime.Time, *pod.StartTime)

		// Check containers
		require.Len(t, pod.Containers, 1)
		container := pod.Containers[0]
		assert.Equal(t, "nginx", container.Name)
		assert.Equal(t, "nginx:latest", container.Image)
		assert.True(t, container.Ready)
		assert.Equal(t, int32(0), container.RestartCount)

		// Check resources
		assert.Equal(t, "100m", container.Resources.Requests["cpu"])
		assert.Equal(t, "128Mi", container.Resources.Requests["memory"])
		assert.Equal(t, "200m", container.Resources.Limits["cpu"])
		assert.Equal(t, "256Mi", container.Resources.Limits["memory"])

		// Check state
		require.NotNil(t, container.State)
		running, exists := container.State["running"]
		assert.True(t, exists)
		runningMap := running.(map[string]interface{})
		assert.Equal(t, now.Time, runningMap["startedAt"])
	})

	t.Run("convert nil pod", func(t *testing.T) {
		pod := ConvertPod(nil)
		assert.Nil(t, pod)
	})
}

func TestApplyNodeMetrics(t *testing.T) {
	node := &Node{
		Name: "test-node",
	}

	metrics := &metricsv1beta1.NodeMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Usage: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	t.Run("apply valid metrics", func(t *testing.T) {
		ApplyNodeMetrics(node, metrics)

		require.NotNil(t, node.Usage)
		assert.Equal(t, "500m", node.Usage.CPU)
		assert.Equal(t, "2Gi", node.Usage.Memory)
	})

	t.Run("apply nil metrics", func(t *testing.T) {
		node := &Node{Name: "test-node"}
		ApplyNodeMetrics(node, nil)
		assert.Nil(t, node.Usage)
	})

	t.Run("apply to nil node", func(t *testing.T) {
		// Should not panic
		ApplyNodeMetrics(nil, metrics)
	})
}

func TestApplyPodMetrics(t *testing.T) {
	pod := &Pod{
		Name:      "test-pod",
		Namespace: "default",
		Containers: []Container{
			{
				Name:  "nginx",
				Image: "nginx:latest",
			},
			{
				Name:  "sidecar",
				Image: "sidecar:latest",
			},
		},
	}

	metrics := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Containers: []metricsv1beta1.ContainerMetrics{
			{
				Name: "nginx",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			{
				Name: "sidecar",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
	}

	t.Run("apply valid metrics", func(t *testing.T) {
		ApplyPodMetrics(pod, metrics)

		require.NotNil(t, pod.Usage)
		assert.Equal(t, "150m", pod.Usage.CPU)       // 100m + 50m
		assert.Equal(t, "192.0Mi", pod.Usage.Memory) // 128Mi + 64Mi

		// Check container metrics
		require.NotNil(t, pod.Containers[0].Usage)
		assert.Equal(t, "100m", pod.Containers[0].Usage.CPU)
		assert.Equal(t, "128Mi", pod.Containers[0].Usage.Memory)

		require.NotNil(t, pod.Containers[1].Usage)
		assert.Equal(t, "50m", pod.Containers[1].Usage.CPU)
		assert.Equal(t, "64Mi", pod.Containers[1].Usage.Memory)
	})

	t.Run("apply nil metrics", func(t *testing.T) {
		pod := &Pod{Name: "test-pod"}
		ApplyPodMetrics(pod, nil)
		assert.Nil(t, pod.Usage)
	})

	t.Run("apply to nil pod", func(t *testing.T) {
		// Should not panic
		ApplyPodMetrics(nil, metrics)
	})
}

func TestCreateClusterData(t *testing.T) {
	clusterData := CreateClusterData("test-cluster", "https://api.test.com")

	assert.Equal(t, "test-cluster", clusterData.ID)
	assert.Equal(t, "https://api.test.com", clusterData.APIServerURL)
	assert.NotNil(t, clusterData.Nodes)
	assert.NotNil(t, clusterData.UnassignedPods)
	assert.False(t, clusterData.LastUpdate.IsZero())
}

func TestClusterDataOperations(t *testing.T) {
	clusterData := CreateClusterData("test-cluster", "https://api.test.com")

	pod := &Pod{
		Name:      "test-pod",
		Namespace: "default",
	}

	t.Run("add pod to node", func(t *testing.T) {
		clusterData.AddPodToNode(pod, "test-node")

		assert.Len(t, clusterData.Nodes, 1)
		assert.NotNil(t, clusterData.Nodes["test-node"])
		assert.Len(t, clusterData.Nodes["test-node"].Pods, 1)
		assert.Equal(t, "test-node", pod.NodeName)

		podKey := "default/test-pod"
		assert.NotNil(t, clusterData.Nodes["test-node"].Pods[podKey])
	})

	t.Run("add pod to unassigned", func(t *testing.T) {
		unassignedPod := &Pod{
			Name:      "unassigned-pod",
			Namespace: "default",
		}

		clusterData.AddPodToNode(unassignedPod, "")

		assert.Len(t, clusterData.UnassignedPods, 1)
		podKey := "default/unassigned-pod"
		assert.NotNil(t, clusterData.UnassignedPods[podKey])
	})

	t.Run("remove pod", func(t *testing.T) {
		clusterData.RemovePod("default", "test-pod")

		assert.Len(t, clusterData.Nodes["test-node"].Pods, 0)

		clusterData.RemovePod("default", "unassigned-pod")
		assert.Len(t, clusterData.UnassignedPods, 0)
	})

	t.Run("get counts", func(t *testing.T) {
		// Create fresh cluster data for this test
		freshClusterData := CreateClusterData("test-cluster", "https://api.test.com")

		// Add some test data
		freshClusterData.AddPodToNode(&Pod{Name: "pod1", Namespace: "default"}, "node1")
		freshClusterData.AddPodToNode(&Pod{Name: "pod2", Namespace: "default"}, "node2")
		freshClusterData.AddPodToNode(&Pod{Name: "pod3", Namespace: "default"}, "")

		assert.Equal(t, 2, freshClusterData.GetNodeCount())
		assert.Equal(t, 3, freshClusterData.GetPodCount())
	})

	t.Run("update last seen", func(t *testing.T) {
		oldTime := clusterData.LastUpdate
		time.Sleep(1 * time.Millisecond) // Ensure time difference

		clusterData.UpdateLastSeen()

		assert.True(t, clusterData.LastUpdate.After(oldTime))
	})
}

func TestIsNodeReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []corev1.NodeCondition
		expected   bool
	}{
		{
			name: "ready node",
			conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
			expected: true,
		},
		{
			name: "not ready node",
			conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
			expected: false,
		},
		{
			name:       "no conditions",
			conditions: []corev1.NodeCondition{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: tt.conditions,
				},
			}

			result := isNodeReady(node)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name       string
		phase      corev1.PodPhase
		conditions []corev1.PodCondition
		expected   bool
	}{
		{
			name:  "ready running pod",
			phase: corev1.PodRunning,
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			expected: true,
		},
		{
			name:  "not ready running pod",
			phase: corev1.PodRunning,
			conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionFalse,
				},
			},
			expected: false,
		},
		{
			name:       "pending pod",
			phase:      corev1.PodPending,
			conditions: []corev1.PodCondition{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      tt.phase,
					Conditions: tt.conditions,
				},
			}

			result := isPodReady(pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

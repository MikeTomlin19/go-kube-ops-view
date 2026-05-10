package models

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// ConvertNode converts a Kubernetes Node object to our internal Node model
func ConvertNode(k8sNode *corev1.Node) *Node {
	if k8sNode == nil {
		return nil
	}

	node := &Node{
		Name:        k8sNode.Name,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Pods:        make(map[string]*Pod),
		Capacity:    make(ResourceList),
		Allocatable: make(ResourceList),
		Status: NodeStatus{
			Ready: isNodeReady(k8sNode),
		},
	}

	// Copy labels
	for k, v := range k8sNode.Labels {
		node.Labels[k] = v
	}

	// Copy annotations
	for k, v := range k8sNode.Annotations {
		node.Annotations[k] = v
	}

	// Convert capacity
	for resource, quantity := range k8sNode.Status.Capacity {
		node.Capacity[string(resource)] = quantity.String()
	}

	// Convert allocatable
	for resource, quantity := range k8sNode.Status.Allocatable {
		node.Allocatable[string(resource)] = quantity.String()
	}

	// Convert addresses
	for _, addr := range k8sNode.Status.Addresses {
		node.Status.Addresses = append(node.Status.Addresses, NodeAddress{
			Type:    string(addr.Type),
			Address: addr.Address,
		})
	}

	// Convert conditions
	for _, condition := range k8sNode.Status.Conditions {
		node.Status.Conditions = append(node.Status.Conditions, NodeCondition{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			LastHeartbeatTime:  condition.LastHeartbeatTime.Time,
			LastTransitionTime: condition.LastTransitionTime.Time,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	return node
}

// ConvertPod converts a Kubernetes Pod object to our internal Pod model
func ConvertPod(k8sPod *corev1.Pod) *Pod {
	if k8sPod == nil {
		return nil
	}

	pod := &Pod{
		Name:      k8sPod.Name,
		Namespace: k8sPod.Namespace,
		Labels:    make(map[string]string),
		Phase:     string(k8sPod.Status.Phase),
		NodeName:  k8sPod.Spec.NodeName,
		Ready:     isPodReady(k8sPod),
	}

	// Copy labels
	for k, v := range k8sPod.Labels {
		pod.Labels[k] = v
	}

	// Set start time
	if k8sPod.Status.StartTime != nil {
		pod.StartTime = &k8sPod.Status.StartTime.Time
	}

	// Set deletion timestamp if pod is being deleted
	if k8sPod.DeletionTimestamp != nil {
		pod.Deleted = &k8sPod.DeletionTimestamp.Time
	}

	// Extract owner information
	if len(k8sPod.OwnerReferences) > 0 {
		owner := k8sPod.OwnerReferences[0] // Use first owner
		pod.OwnerKind = owner.Kind
		pod.OwnerName = owner.Name
	}

	// Convert containers
	for _, k8sContainer := range k8sPod.Spec.Containers {
		container := ConvertContainer(&k8sContainer, k8sPod)
		pod.Containers = append(pod.Containers, container)
	}

	return pod
}

// ConvertContainer converts a Kubernetes Container spec to our internal Container model
func ConvertContainer(k8sContainer *corev1.Container, k8sPod *corev1.Pod) Container {
	container := Container{
		Name:  k8sContainer.Name,
		Image: k8sContainer.Image,
		Resources: ContainerResources{
			Requests: make(ResourceList),
			Limits:   make(ResourceList),
		},
	}

	// Convert resource requests
	for resource, quantity := range k8sContainer.Resources.Requests {
		container.Resources.Requests[string(resource)] = quantity.String()
	}

	// Convert resource limits
	for resource, quantity := range k8sContainer.Resources.Limits {
		container.Resources.Limits[string(resource)] = quantity.String()
	}

	// Find container status from pod status
	if k8sPod != nil {
		for _, status := range k8sPod.Status.ContainerStatuses {
			if status.Name == k8sContainer.Name {
				container.Ready = status.Ready
				container.RestartCount = status.RestartCount

				// Convert container state
				container.State = make(map[string]interface{})
				if status.State.Running != nil {
					container.State["running"] = map[string]interface{}{
						"startedAt": status.State.Running.StartedAt.Time,
					}
				} else if status.State.Waiting != nil {
					container.State["waiting"] = map[string]interface{}{
						"reason":  status.State.Waiting.Reason,
						"message": status.State.Waiting.Message,
					}
				} else if status.State.Terminated != nil {
					container.State["terminated"] = map[string]interface{}{
						"exitCode":   status.State.Terminated.ExitCode,
						"reason":     status.State.Terminated.Reason,
						"message":    status.State.Terminated.Message,
						"startedAt":  status.State.Terminated.StartedAt.Time,
						"finishedAt": status.State.Terminated.FinishedAt.Time,
					}
				}
				break
			}
		}
	}

	return container
}

// ApplyNodeMetrics applies metrics data to a Node
func ApplyNodeMetrics(node *Node, metrics *metricsv1beta1.NodeMetrics) {
	if node == nil || metrics == nil {
		return
	}

	if node.Usage == nil {
		node.Usage = &ResourceUsage{}
	}

	// Apply CPU usage
	if cpu, exists := metrics.Usage[corev1.ResourceCPU]; exists {
		node.Usage.CPU = cpu.String()
	}

	// Apply memory usage
	if memory, exists := metrics.Usage[corev1.ResourceMemory]; exists {
		node.Usage.Memory = memory.String()
	}
}

// ApplyPodMetrics applies metrics data to a Pod and its containers
func ApplyPodMetrics(pod *Pod, metrics *metricsv1beta1.PodMetrics) {
	if pod == nil || metrics == nil {
		return
	}

	if pod.Usage == nil {
		pod.Usage = &ResourceUsage{}
	}

	// Calculate total pod usage from container metrics
	var totalCPU, totalMemory int64

	for _, containerMetrics := range metrics.Containers {
		// Find matching container in pod
		for i := range pod.Containers {
			if pod.Containers[i].Name == containerMetrics.Name {
				if pod.Containers[i].Usage == nil {
					pod.Containers[i].Usage = &ResourceUsage{}
				}

				// Apply container CPU usage
				if cpu, exists := containerMetrics.Usage[corev1.ResourceCPU]; exists {
					pod.Containers[i].Usage.CPU = cpu.String()
					if parsed, err := ParseCPU(cpu.String()); err == nil && parsed != nil {
						totalCPU += parsed.MilliUnit
					}
				}

				// Apply container memory usage
				if memory, exists := containerMetrics.Usage[corev1.ResourceMemory]; exists {
					pod.Containers[i].Usage.Memory = memory.String()
					if parsed, err := ParseMemory(memory.String()); err == nil && parsed != nil {
						totalMemory += parsed.MilliUnit
					}
				}
				break
			}
		}
	}

	// Set total pod usage
	if totalCPU > 0 {
		pod.Usage.CPU = FormatCPU(totalCPU)
	}
	if totalMemory > 0 {
		pod.Usage.Memory = FormatMemory(totalMemory)
	}
}

// isNodeReady checks if a node is in Ready condition
func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// isPodReady checks if a pod is ready (all containers are ready)
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// CreateClusterData creates a new ClusterData instance
func CreateClusterData(clusterID, apiServerURL string) *ClusterData {
	return &ClusterData{
		ID:             clusterID,
		APIServerURL:   apiServerURL,
		Nodes:          make(map[string]*Node),
		UnassignedPods: make(map[string]*Pod),
		LastUpdate:     time.Now(),
	}
}

// AddPodToNode adds a pod to a node, creating the node if it doesn't exist
func (cd *ClusterData) AddPodToNode(pod *Pod, nodeName string) {
	if pod == nil {
		return
	}

	if nodeName == "" {
		// Add to unassigned pods
		cd.UnassignedPods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = pod
		return
	}

	// Ensure node exists
	if cd.Nodes[nodeName] == nil {
		cd.Nodes[nodeName] = &Node{
			Name: nodeName,
			Pods: make(map[string]*Pod),
		}
	}

	// Add pod to node
	cd.Nodes[nodeName].Pods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = pod
	pod.NodeName = nodeName
}

// RemovePod removes a pod from the cluster data
func (cd *ClusterData) RemovePod(namespace, name string) {
	podKey := fmt.Sprintf("%s/%s", namespace, name)

	// Remove from unassigned pods
	delete(cd.UnassignedPods, podKey)

	// Remove from nodes
	for _, node := range cd.Nodes {
		delete(node.Pods, podKey)
	}
}

// GetPodCount returns the total number of pods in the cluster
func (cd *ClusterData) GetPodCount() int {
	count := len(cd.UnassignedPods)
	for _, node := range cd.Nodes {
		count += len(node.Pods)
	}
	return count
}

// GetNodeCount returns the number of nodes in the cluster
func (cd *ClusterData) GetNodeCount() int {
	return len(cd.Nodes)
}

// UpdateLastSeen updates the last update timestamp
func (cd *ClusterData) UpdateLastSeen() {
	cd.LastUpdate = time.Now()
}

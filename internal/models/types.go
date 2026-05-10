package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// ClusterData represents the complete data structure for a Kubernetes cluster
type ClusterData struct {
	ID             string           `json:"id"`
	APIServerURL   string           `json:"api_server_url"`
	Nodes          map[string]*Node `json:"nodes"`
	UnassignedPods map[string]*Pod  `json:"unassigned_pods"`
	LastUpdate     time.Time        `json:"last_update"`
}

// Node represents a Kubernetes node with its pods and resource information
type Node struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Status      NodeStatus        `json:"status"`
	Pods        map[string]*Pod   `json:"pods"`
	Usage       *ResourceUsage    `json:"usage,omitempty"`
	Capacity    ResourceList      `json:"capacity,omitempty"`
	Allocatable ResourceList      `json:"allocatable,omitempty"`
}

// NodeStatus represents the status information of a node
type NodeStatus struct {
	Addresses   []NodeAddress     `json:"addresses,omitempty"`
	Capacity    map[string]string `json:"capacity,omitempty"`
	Allocatable map[string]string `json:"allocatable,omitempty"`
	Conditions  []NodeCondition   `json:"conditions,omitempty"`
	Ready       bool              `json:"ready"`
}

// NodeAddress represents a node address
type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

// NodeCondition represents a node condition
type NodeCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastHeartbeatTime  time.Time `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// Pod represents a Kubernetes pod with its containers and resource information
type Pod struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Labels     map[string]string `json:"labels"`
	Phase      string            `json:"phase"`
	StartTime  *time.Time        `json:"startTime,omitempty"`
	Containers []Container       `json:"containers"`
	Deleted    *time.Time        `json:"deleted,omitempty"`
	NodeName   string            `json:"nodeName,omitempty"`
	OwnerKind  string            `json:"ownerKind,omitempty"`
	OwnerName  string            `json:"ownerName,omitempty"`
	Ready      bool              `json:"ready"`
	Usage      *ResourceUsage    `json:"usage,omitempty"`
}

// Container represents a container within a pod
type Container struct {
	Name         string                 `json:"name"`
	Image        string                 `json:"image"`
	Resources    ContainerResources     `json:"resources"`
	Ready        bool                   `json:"ready,omitempty"`
	State        map[string]interface{} `json:"state,omitempty"`
	RestartCount int32                  `json:"restartCount,omitempty"`
	Usage        *ResourceUsage         `json:"usage,omitempty"`
}

// ContainerResources represents resource requests, limits, and usage for a container
type ContainerResources struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

// ResourceUsage represents current resource usage
type ResourceUsage struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// ResourceList represents a map of resource names to quantities
type ResourceList map[string]string

// ParsedResource represents a parsed resource quantity with both string and numeric values
type ParsedResource struct {
	Raw       string  `json:"raw"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	MilliUnit int64   `json:"milli_unit,omitempty"` // For CPU in millicores, Memory in bytes
}

// ParseCPU parses a CPU resource string (e.g., "100m", "0.1", "1") into a standardized format
func ParseCPU(cpu string) (*ParsedResource, error) {
	if cpu == "" {
		return nil, nil
	}

	// Handle Kubernetes resource.Quantity format
	if quantity, err := resource.ParseQuantity(cpu); err == nil {
		milliValue := quantity.MilliValue()
		return &ParsedResource{
			Raw:       cpu,
			Value:     float64(milliValue) / 1000.0,
			Unit:      "cores",
			MilliUnit: milliValue,
		}, nil
	}

	// Handle direct numeric values
	cpu = strings.TrimSpace(cpu)

	// Handle millicores (e.g., "100m")
	if strings.HasSuffix(cpu, "m") {
		milliStr := strings.TrimSuffix(cpu, "m")
		milli, err := strconv.ParseInt(milliStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid CPU format: %s", cpu)
		}
		return &ParsedResource{
			Raw:       cpu,
			Value:     float64(milli) / 1000.0,
			Unit:      "cores",
			MilliUnit: milli,
		}, nil
	}

	// Handle decimal cores (e.g., "0.1", "1.5")
	value, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU format: %s", cpu)
	}

	return &ParsedResource{
		Raw:       cpu,
		Value:     value,
		Unit:      "cores",
		MilliUnit: int64(value * 1000),
	}, nil
}

// ParseMemory parses a memory resource string (e.g., "128Mi", "1Gi", "1000000000") into a standardized format
func ParseMemory(memory string) (*ParsedResource, error) {
	if memory == "" {
		return nil, nil
	}

	// Handle Kubernetes resource.Quantity format
	if quantity, err := resource.ParseQuantity(memory); err == nil {
		bytes := quantity.Value()
		return &ParsedResource{
			Raw:       memory,
			Value:     float64(bytes),
			Unit:      "bytes",
			MilliUnit: bytes,
		}, nil
	}

	// Handle direct numeric values (assume bytes)
	memory = strings.TrimSpace(memory)
	value, err := strconv.ParseInt(memory, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid memory format: %s", memory)
	}

	return &ParsedResource{
		Raw:       memory,
		Value:     float64(value),
		Unit:      "bytes",
		MilliUnit: value,
	}, nil
}

// FormatCPU formats CPU value to a human-readable string
func FormatCPU(milliCores int64) string {
	if milliCores < 1000 {
		return fmt.Sprintf("%dm", milliCores)
	}
	return fmt.Sprintf("%.1f", float64(milliCores)/1000.0)
}

// FormatMemory formats memory value to a human-readable string
func FormatMemory(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.1fTi", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.1fGi", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1fMi", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1fKi", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

// Validate performs validation on ClusterData
func (cd *ClusterData) Validate() error {
	if cd.ID == "" {
		return fmt.Errorf("cluster ID cannot be empty")
	}
	if cd.APIServerURL == "" {
		return fmt.Errorf("API server URL cannot be empty")
	}

	// Validate nodes
	for nodeName, node := range cd.Nodes {
		if node == nil {
			return fmt.Errorf("node %s cannot be nil", nodeName)
		}
		if err := node.Validate(); err != nil {
			return fmt.Errorf("invalid node %s: %w", nodeName, err)
		}
	}

	// Validate unassigned pods
	for podName, pod := range cd.UnassignedPods {
		if pod == nil {
			return fmt.Errorf("unassigned pod %s cannot be nil", podName)
		}
		if err := pod.Validate(); err != nil {
			return fmt.Errorf("invalid unassigned pod %s: %w", podName, err)
		}
	}

	return nil
}

// Validate performs validation on Node
func (n *Node) Validate() error {
	if n.Name == "" {
		return fmt.Errorf("node name cannot be empty")
	}

	// Validate pods
	for podName, pod := range n.Pods {
		if pod == nil {
			return fmt.Errorf("pod %s cannot be nil", podName)
		}
		if err := pod.Validate(); err != nil {
			return fmt.Errorf("invalid pod %s: %w", podName, err)
		}
	}

	return nil
}

// Validate performs validation on Pod
func (p *Pod) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("pod name cannot be empty")
	}
	if p.Namespace == "" {
		return fmt.Errorf("pod namespace cannot be empty")
	}

	// Validate containers
	for i, container := range p.Containers {
		if err := container.Validate(); err != nil {
			return fmt.Errorf("invalid container %d: %w", i, err)
		}
	}

	return nil
}

// Validate performs validation on Container
func (c *Container) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("container name cannot be empty")
	}
	if c.Image == "" {
		return fmt.Errorf("container image cannot be empty")
	}

	return nil
}

// IsSystemPod returns true if the pod belongs to a system namespace
func (p *Pod) IsSystemPod() bool {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
	}

	for _, ns := range systemNamespaces {
		if p.Namespace == ns {
			return true
		}
	}

	return false
}

// GetTotalCPURequests calculates total CPU requests for all containers in the pod
func (p *Pod) GetTotalCPURequests() (int64, error) {
	var total int64
	for _, container := range p.Containers {
		if cpuReq, exists := container.Resources.Requests["cpu"]; exists {
			parsed, err := ParseCPU(cpuReq)
			if err != nil {
				return 0, err
			}
			if parsed != nil {
				total += parsed.MilliUnit
			}
		}
	}
	return total, nil
}

// GetTotalMemoryRequests calculates total memory requests for all containers in the pod
func (p *Pod) GetTotalMemoryRequests() (int64, error) {
	var total int64
	for _, container := range p.Containers {
		if memReq, exists := container.Resources.Requests["memory"]; exists {
			parsed, err := ParseMemory(memReq)
			if err != nil {
				return 0, err
			}
			if parsed != nil {
				total += parsed.MilliUnit
			}
		}
	}
	return total, nil
}

// GetTotalCPULimits calculates total CPU limits for all containers in the pod
func (p *Pod) GetTotalCPULimits() (int64, error) {
	var total int64
	for _, container := range p.Containers {
		if cpuLimit, exists := container.Resources.Limits["cpu"]; exists {
			parsed, err := ParseCPU(cpuLimit)
			if err != nil {
				return 0, err
			}
			if parsed != nil {
				total += parsed.MilliUnit
			}
		}
	}
	return total, nil
}

// GetTotalMemoryLimits calculates total memory limits for all containers in the pod
func (p *Pod) GetTotalMemoryLimits() (int64, error) {
	var total int64
	for _, container := range p.Containers {
		if memLimit, exists := container.Resources.Limits["memory"]; exists {
			parsed, err := ParseMemory(memLimit)
			if err != nil {
				return 0, err
			}
			if parsed != nil {
				total += parsed.MilliUnit
			}
		}
	}
	return total, nil
}

// Clone creates a deep copy of ClusterData
func (cd *ClusterData) Clone() *ClusterData {
	clone := &ClusterData{
		ID:             cd.ID,
		APIServerURL:   cd.APIServerURL,
		LastUpdate:     cd.LastUpdate,
		Nodes:          make(map[string]*Node),
		UnassignedPods: make(map[string]*Pod),
	}

	for name, node := range cd.Nodes {
		clone.Nodes[name] = node.Clone()
	}

	for name, pod := range cd.UnassignedPods {
		clone.UnassignedPods[name] = pod.Clone()
	}

	return clone
}

// Clone creates a deep copy of Node
func (n *Node) Clone() *Node {
	clone := &Node{
		Name:        n.Name,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Status:      n.Status,
		Pods:        make(map[string]*Pod),
		Capacity:    make(ResourceList),
		Allocatable: make(ResourceList),
	}

	for k, v := range n.Labels {
		clone.Labels[k] = v
	}

	for k, v := range n.Annotations {
		clone.Annotations[k] = v
	}

	for k, v := range n.Capacity {
		clone.Capacity[k] = v
	}

	for k, v := range n.Allocatable {
		clone.Allocatable[k] = v
	}

	for name, pod := range n.Pods {
		clone.Pods[name] = pod.Clone()
	}

	if n.Usage != nil {
		clone.Usage = &ResourceUsage{
			CPU:    n.Usage.CPU,
			Memory: n.Usage.Memory,
		}
	}

	return clone
}

// Clone creates a deep copy of Pod
func (p *Pod) Clone() *Pod {
	clone := &Pod{
		Name:      p.Name,
		Namespace: p.Namespace,
		Labels:    make(map[string]string),
		Phase:     p.Phase,
		NodeName:  p.NodeName,
		OwnerKind: p.OwnerKind,
		OwnerName: p.OwnerName,
		Ready:     p.Ready,
	}

	if p.StartTime != nil {
		startTime := *p.StartTime
		clone.StartTime = &startTime
	}

	if p.Deleted != nil {
		deleted := *p.Deleted
		clone.Deleted = &deleted
	}

	for k, v := range p.Labels {
		clone.Labels[k] = v
	}

	for _, container := range p.Containers {
		clone.Containers = append(clone.Containers, container.Clone())
	}

	if p.Usage != nil {
		clone.Usage = &ResourceUsage{
			CPU:    p.Usage.CPU,
			Memory: p.Usage.Memory,
		}
	}

	return clone
}

// Clone creates a deep copy of Container
func (c *Container) Clone() Container {
	clone := Container{
		Name:         c.Name,
		Image:        c.Image,
		Ready:        c.Ready,
		RestartCount: c.RestartCount,
		Resources: ContainerResources{
			Requests: make(ResourceList),
			Limits:   make(ResourceList),
		},
	}

	for k, v := range c.Resources.Requests {
		clone.Resources.Requests[k] = v
	}

	for k, v := range c.Resources.Limits {
		clone.Resources.Limits[k] = v
	}

	if c.State != nil {
		clone.State = make(map[string]interface{})
		for k, v := range c.State {
			clone.State[k] = v
		}
	}

	if c.Usage != nil {
		clone.Usage = &ResourceUsage{
			CPU:    c.Usage.CPU,
			Memory: c.Usage.Memory,
		}
	}

	return clone
}

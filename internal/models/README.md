# Models Package

This package contains the core data models and structures for the Kubernetes Operational View application. It provides Go structs that represent Kubernetes resources (nodes, pods, containers) along with utilities for resource parsing, validation, and conversion from Kubernetes API objects.

## Overview

The models package is designed to:

1. **Represent cluster data** in a format optimized for the web UI
2. **Parse and format** Kubernetes resource quantities (CPU, memory)
3. **Convert** between Kubernetes API objects and internal models
4. **Validate** data integrity and consistency
5. **Provide utilities** for resource calculations and cluster operations

## Core Data Structures

### ClusterData

The main container for all cluster information:

```go
type ClusterData struct {
    ID             string            `json:"id"`
    APIServerURL   string            `json:"api_server_url"`
    Nodes          map[string]*Node  `json:"nodes"`
    UnassignedPods map[string]*Pod   `json:"unassigned_pods"`
    LastUpdate     time.Time         `json:"last_update"`
}
```

### Node

Represents a Kubernetes node with its pods and resource information:

```go
type Node struct {
    Name        string            `json:"name"`
    Labels      map[string]string `json:"labels"`
    Status      NodeStatus        `json:"status"`
    Pods        map[string]*Pod   `json:"pods"`
    Usage       *ResourceUsage    `json:"usage,omitempty"`
    Capacity    ResourceList      `json:"capacity,omitempty"`
    Allocatable ResourceList      `json:"allocatable,omitempty"`
}
```

### Pod

Represents a Kubernetes pod with its containers and metadata:

```go
type Pod struct {
    Name       string            `json:"name"`
    Namespace  string            `json:"namespace"`
    Labels     map[string]string `json:"labels"`
    Phase      string            `json:"phase"`
    Containers []Container       `json:"containers"`
    Ready      bool              `json:"ready"`
    Usage      *ResourceUsage    `json:"usage,omitempty"`
}
```

### Container

Represents a container within a pod:

```go
type Container struct {
    Name         string                 `json:"name"`
    Image        string                 `json:"image"`
    Resources    ContainerResources     `json:"resources"`
    Ready        bool                   `json:"ready,omitempty"`
    Usage        *ResourceUsage         `json:"usage,omitempty"`
}
```

## Resource Parsing and Formatting

The package provides utilities for parsing and formatting Kubernetes resource quantities:

### CPU Resources

```go
// Parse CPU values (supports "100m", "0.1", "1.5")
parsed, err := models.ParseCPU("500m")
if err == nil {
    fmt.Printf("CPU: %.1f cores (%d millicores)\n", parsed.Value, parsed.MilliUnit)
}

// Format CPU values
formatted := models.FormatCPU(1500) // "1.5"
```

### Memory Resources

```go
// Parse memory values (supports "128Mi", "1Gi", "1000000000")
parsed, err := models.ParseMemory("2Gi")
if err == nil {
    fmt.Printf("Memory: %.0f bytes\n", parsed.Value)
}

// Format memory values
formatted := models.FormatMemory(2147483648) // "2.0Gi"
```

## Kubernetes API Conversion

The package provides converters for transforming Kubernetes API objects into internal models:

### Converting Nodes

```go
import corev1 "k8s.io/api/core/v1"

// Convert Kubernetes Node to internal model
k8sNode := &corev1.Node{...}
node := models.ConvertNode(k8sNode)
```

### Converting Pods

```go
// Convert Kubernetes Pod to internal model
k8sPod := &corev1.Pod{...}
pod := models.ConvertPod(k8sPod)
```

### Applying Metrics

```go
import metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

// Apply node metrics
nodeMetrics := &metricsv1beta1.NodeMetrics{...}
models.ApplyNodeMetrics(node, nodeMetrics)

// Apply pod metrics
podMetrics := &metricsv1beta1.PodMetrics{...}
models.ApplyPodMetrics(pod, podMetrics)
```

## Cluster Operations

### Creating and Managing Cluster Data

```go
// Create new cluster data
clusterData := models.CreateClusterData("my-cluster", "https://api.example.com")

// Add pods to nodes
pod := &models.Pod{Name: "nginx", Namespace: "default"}
clusterData.AddPodToNode(pod, "worker-node-1")

// Add unassigned pods
clusterData.AddPodToNode(unassignedPod, "") // Empty node name = unassigned

// Remove pods
clusterData.RemovePod("default", "nginx")

// Get statistics
nodeCount := clusterData.GetNodeCount()
podCount := clusterData.GetPodCount()
```

### Resource Calculations

```go
// Calculate total resource requests for a pod
cpuRequests, err := pod.GetTotalCPURequests()
memoryRequests, err := pod.GetTotalMemoryRequests()

// Calculate total resource limits for a pod
cpuLimits, err := pod.GetTotalCPULimits()
memoryLimits, err := pod.GetTotalMemoryLimits()
```

## Validation

All major data structures support validation:

```go
// Validate cluster data
if err := clusterData.Validate(); err != nil {
    log.Printf("Invalid cluster data: %v", err)
}

// Validate individual components
if err := node.Validate(); err != nil {
    log.Printf("Invalid node: %v", err)
}

if err := pod.Validate(); err != nil {
    log.Printf("Invalid pod: %v", err)
}
```

## JSON Serialization

All models support JSON marshaling/unmarshaling with proper field tags:

```go
// Marshal to JSON
data, err := json.Marshal(clusterData)

// Unmarshal from JSON
var clusterData models.ClusterData
err := json.Unmarshal(data, &clusterData)
```

## Deep Copying

Models support deep copying for safe concurrent access:

```go
// Clone cluster data
clonedCluster := clusterData.Clone()

// Clone individual components
clonedNode := node.Clone()
clonedPod := pod.Clone()
```

## Utility Functions

### System Pod Detection

```go
// Check if a pod belongs to a system namespace
isSystem := pod.IsSystemPod() // true for kube-system, kube-public, etc.
```

### Time Management

```go
// Update last seen timestamp
clusterData.UpdateLastSeen()
```

## Testing

The package includes comprehensive unit tests covering:

- Resource parsing and formatting
- Kubernetes API object conversion
- Data validation
- JSON serialization/deserialization
- Deep copying
- Cluster operations

Run tests with:

```bash
go test ./internal/models -v
```

## Dependencies

- `k8s.io/api/core/v1` - Kubernetes core API types
- `k8s.io/apimachinery/pkg/api/resource` - Resource quantity parsing
- `k8s.io/metrics/pkg/apis/metrics/v1beta1` - Metrics API types
- `github.com/stretchr/testify` - Testing utilities (test dependencies only)

## Design Principles

1. **JSON Compatibility**: All structures are designed to be JSON-serializable for web API compatibility
2. **Kubernetes Compatibility**: Resource parsing follows Kubernetes quantity formats exactly
3. **Performance**: Efficient data structures with minimal allocations
4. **Safety**: Comprehensive validation and nil-safe operations
5. **Testability**: Extensive test coverage with clear examples
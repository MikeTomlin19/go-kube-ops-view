# Cluster Package

The cluster package provides Kubernetes client integration for the kube-ops-view application. It includes functionality for connecting to multiple Kubernetes clusters, discovering clusters through various methods, and retrieving cluster data including nodes, pods, and metrics.

## Features

- **Multi-cluster support**: Connect to and manage multiple Kubernetes clusters simultaneously
- **Flexible authentication**: Support for token-based auth, certificate auth, and kubeconfig files
- **Cluster discovery**: Automatic cluster discovery through static URLs, kubeconfig files, and registry endpoints
- **Metrics integration**: Built-in support for Kubernetes metrics server
- **Comprehensive testing**: Full test coverage using fake Kubernetes clientsets

## Core Components

### ClusterClient

The `ClusterClient` wraps Kubernetes client-go interfaces and provides methods for interacting with a single cluster:

```go
// Create a cluster client
config := &ClusterConfig{
    ID:        "my-cluster",
    APIServer: "https://kubernetes.example.com",
    Token:     "my-token",
}

client, err := NewClusterClient(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Test connection
err = client.TestConnection(ctx)

// Get cluster resources
nodes, err := client.GetNodes(ctx)
pods, err := client.GetPods(ctx)
status, err := client.GetClusterStatus(ctx)

// Get metrics (if metrics server is available)
nodeMetrics, err := client.GetNodeMetrics(ctx)
podMetrics, err := client.GetPodMetrics(ctx)
```

### Cluster Discovery

The package supports multiple cluster discovery methods:

#### Static Discovery

Discover clusters from a static list of API server URLs:

```go
discoverer := NewStaticDiscoverer([]string{
    "https://cluster1.example.com",
    "https://cluster2.example.com",
})

configs, err := discoverer.DiscoverClusters(ctx)
```

#### Kubeconfig Discovery

Discover clusters from kubeconfig files:

```go
// Use all contexts from kubeconfig
discoverer := NewKubeconfigDiscoverer("~/.kube/config", []string{})

// Use specific contexts only
discoverer := NewKubeconfigDiscoverer("~/.kube/config", []string{"prod", "staging"})

configs, err := discoverer.DiscoverClusters(ctx)
```

#### Registry Discovery

Discover clusters from a cluster registry endpoint:

```go
discoverer := NewRegistryDiscoverer("https://registry.example.com/clusters")
configs, err := discoverer.DiscoverClusters(ctx)
```

#### Multi-Discovery

Combine multiple discovery methods:

```go
static := NewStaticDiscoverer([]string{"https://prod.example.com"})
kubeconfig := NewKubeconfigDiscoverer("~/.kube/config", []string{})

multi := NewMultiDiscoverer(static, kubeconfig)
configs, err := multi.DiscoverClusters(ctx)
```

#### Discovery Factory

Use the factory for convenient discoverer creation:

```go
factory := NewDiscovererFactory()
discoverer := factory.CreateDiscoverer(
    []string{"https://static.example.com"}, // static URLs
    "~/.kube/config",                       // kubeconfig path
    []string{"prod", "staging"},            // contexts
    "https://registry.example.com",         // registry URL
)
```

## Authentication Methods

The package supports multiple authentication methods:

### Token Authentication

```go
config := &ClusterConfig{
    ID:        "cluster-1",
    APIServer: "https://kubernetes.example.com",
    Token:     "bearer-token-here",
}
```

### Certificate Authentication

```go
config := &ClusterConfig{
    ID:        "cluster-1",
    APIServer: "https://kubernetes.example.com",
    CertFile:  "/path/to/client.crt",
    KeyFile:   "/path/to/client.key",
    CAFile:    "/path/to/ca.crt",
}
```

### Kubeconfig Authentication

```go
config := &ClusterConfig{
    ID:             "cluster-1",
    APIServer:      "https://kubernetes.example.com",
    KubeconfigPath: "~/.kube/config",
    Context:        "my-context",
}
```

### Insecure Connection

```go
config := &ClusterConfig{
    ID:        "cluster-1",
    APIServer: "https://kubernetes.example.com",
    Token:     "bearer-token-here",
    Insecure:  true, // Skip TLS verification
}
```

## Metrics Support

The package includes built-in support for Kubernetes metrics server:

```go
client, err := NewClusterClient(config)

// Check if metrics are available
if client.IsMetricsAvailable(ctx) {
    // Get node resource usage
    nodeMetrics, err := client.GetNodeMetrics(ctx)

    // Get pod resource usage
    podMetrics, err := client.GetPodMetrics(ctx)

    // Get metrics for pods on a specific node
    nodeMetrics, err := client.GetPodMetricsForNode(ctx, "node-1")
}
```

## Error Handling

The package provides structured error handling:

```go
client, err := NewClusterClient(config)
if err != nil {
    // Handle client creation error
    log.Printf("Failed to create client: %v", err)
}

// Test connection with proper error handling
if err := client.TestConnection(ctx); err != nil {
    log.Printf("Cluster %s is unreachable: %v", client.ID, err)
}

// Get cluster status (doesn't return error, sets status instead)
status, _ := client.GetClusterStatus(ctx)
if !status.Available {
    log.Printf("Cluster %s is unavailable: %s", status.ID, status.Error)
}
```

## Testing

The package includes comprehensive tests using fake Kubernetes clientsets:

```bash
# Run all cluster package tests
go test ./internal/cluster/... -v

# Run specific test
go test ./internal/cluster/ -run TestClusterClient_GetNodes -v

# Run tests with coverage
go test ./internal/cluster/... -cover
```

## Integration with kube-ops-view

This cluster package is designed to integrate with the larger kube-ops-view application:

1. **Configuration**: Integrates with the config package for cluster configuration
2. **Data Storage**: Works with the store package for caching cluster data
3. **Event System**: Publishes cluster updates through the event system
4. **Web Server**: Provides data to the web server for the dashboard

## Requirements Satisfied

This implementation satisfies the following requirements from the specification:

- **Requirement 4.2**: Multi-cluster support with discovery services
- **Requirement 4.3**: Authentication handling and cluster connectivity

The package provides a robust foundation for the Kubernetes client integration layer of the kube-ops-view Go conversion.
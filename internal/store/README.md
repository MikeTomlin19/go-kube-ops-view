# Store Package

The store package provides a data storage abstraction layer for the Kubernetes Operational View application. It supports both in-memory and Redis-backed storage with pub/sub functionality for multi-replica deployments.

## Features

- **Multiple Storage Backends**: Memory and Redis support
- **Thread-Safe Operations**: All operations are safe for concurrent use
- **Event System**: Pub/sub events for real-time updates
- **Screen Token Management**: Authentication tokens for display screens
- **Cluster Data Management**: Store and retrieve Kubernetes cluster data
- **Status Tracking**: Monitor cluster availability and health

## Interfaces

### Store Interface

The main `Store` interface provides all storage operations:

```go
type Store interface {
    // Cluster operations
    GetClusterIDs() []string
    GetClusterData(clusterID string) (*models.ClusterData, error)
    SetClusterData(clusterID string, data *models.ClusterData) error
    GetClusterStatus(clusterID string) (*ClusterStatus, error)
    SetClusterStatus(clusterID string, status *ClusterStatus) error
    DeleteCluster(clusterID string) error

    // Event operations
    PublishEvent(eventType string, data interface{}) error
    Subscribe() (<-chan Event, error)
    Unsubscribe(ch <-chan Event) error

    // Screen token operations
    CreateScreenToken() (string, error)
    RedeemScreenToken(token, remoteAddr string) error
    ValidateScreenToken(token string) bool
    DeleteScreenToken(token string) error

    // Health and cleanup
    Close() error
    Ping() error
}
```

## Implementations

### Memory Store

The `MemoryStore` provides in-memory storage suitable for single-instance deployments:

```go
// Create a memory store with default settings
store := NewMemoryStore()

// Create with custom token TTL
store := NewMemoryStoreWithTTL(24 * time.Hour)
```

**Features:**
- Thread-safe operations using RWMutex
- Automatic token expiration cleanup
- Non-blocking event publishing
- Deep copying to prevent external modifications

### Redis Store

The `RedisStore` provides Redis-backed storage with pub/sub for multi-replica deployments:

```go
// Single Redis instance
config := RedisConfig{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
    TokenTTL: 24 * time.Hour,
}
store, err := NewRedisStore(config)

// Redis Cluster
addrs := []string{"localhost:7000", "localhost:7001", "localhost:7002"}
store, err := NewRedisClusterStore(addrs, "", 24*time.Hour)
```

**Features:**
- Automatic reconnection handling
- Pub/sub for cross-replica events
- Atomic operations using Redis pipelines
- Automatic token expiration using Redis TTL

## Configuration

Use the factory function for configuration-based store creation:

```go
config := store.Config{
    Type:     "redis", // "memory", "redis", or "redis-cluster"
    TokenTTL: 24 * time.Hour,
    Redis: store.RedisConfig{
        Addr:     "localhost:6379",
        Password: "",
        DB:       0,
    },
}

store, err := store.NewStore(config)
```

## Usage Examples

### Basic Cluster Operations

```go
// Store cluster data
clusterData := &models.ClusterData{
    ID:           "prod-cluster",
    APIServerURL: "https://k8s.prod.example.com",
    Nodes:        make(map[string]*models.Node),
    UnassignedPods: make(map[string]*models.Pod),
    LastUpdate:   time.Now(),
}

err := store.SetClusterData("prod-cluster", clusterData)
if err != nil {
    log.Fatal(err)
}

// Retrieve cluster data
data, err := store.GetClusterData("prod-cluster")
if err != nil {
    log.Fatal(err)
}

// Get all cluster IDs
ids := store.GetClusterIDs()
```

### Event System

```go
// Subscribe to events
eventCh, err := store.Subscribe()
if err != nil {
    log.Fatal(err)
}

// Listen for events
go func() {
    for event := range eventCh {
        switch event.Type {
        case store.EventTypeClusterUpdate:
            // Handle cluster update
        case store.EventTypeClusterStatus:
            // Handle status change
        }
    }
}()

// Publish events
err = store.PublishEvent(store.EventTypeClusterUpdate, clusterData)
if err != nil {
    log.Fatal(err)
}

// Cleanup
store.Unsubscribe(eventCh)
```

### Screen Token Management

```go
// Create a token for screen authentication
token, err := store.CreateScreenToken()
if err != nil {
    log.Fatal(err)
}

// Validate token
if store.ValidateScreenToken(token) {
    // Token is valid
}

// Redeem token (marks as used)
err = store.RedeemScreenToken(token, "192.168.1.100")
if err != nil {
    switch err {
    case store.ErrTokenNotFound:
        // Token doesn't exist
    case store.ErrTokenExpired:
        // Token has expired
    case store.ErrTokenAlreadyUsed:
        // Token was already redeemed
    }
}
```

### Cluster Status Tracking

```go
// Set cluster status
status := &store.ClusterStatus{
    ID:           "prod-cluster",
    Available:    true,
    LastSeen:     time.Now(),
    APIServerURL: "https://k8s.prod.example.com",
}

err := store.SetClusterStatus("prod-cluster", status)
if err != nil {
    log.Fatal(err)
}

// Check cluster status
status, err := store.GetClusterStatus("prod-cluster")
if err != nil {
    log.Fatal(err)
}

if !status.Available {
    log.Printf("Cluster %s is unavailable: %s", status.ID, status.ErrorMessage)
}
```

## Error Handling

The store package defines several error constants:

```go
const (
    ErrClusterNotFound    = StoreError("cluster not found")
    ErrTokenNotFound      = StoreError("token not found")
    ErrTokenExpired       = StoreError("token expired")
    ErrTokenAlreadyUsed   = StoreError("token already used")
    ErrInvalidToken       = StoreError("invalid token")
    ErrConnectionFailed   = StoreError("connection failed")
    ErrOperationFailed    = StoreError("operation failed")
)
```

Always check for these specific errors when handling store operations.

## Testing

The package includes comprehensive tests for both implementations:

```bash
# Run all tests
go test ./internal/store

# Run only memory store tests
go test ./internal/store -run TestMemoryStore

# Run Redis tests (requires Redis server)
go test ./internal/store -run TestRedisStore

# Run examples
go test ./internal/store -run Example
```

## Thread Safety

Both store implementations are thread-safe:

- **MemoryStore**: Uses `sync.RWMutex` for all operations
- **RedisStore**: Redis operations are inherently atomic

## Performance Considerations

### Memory Store
- Fast operations (in-memory)
- Limited by available RAM
- No persistence across restarts
- Single-instance only

### Redis Store
- Network latency for operations
- Persistent storage
- Supports clustering and replication
- Multi-instance coordination via pub/sub

## Best Practices

1. **Always close stores**: Use `defer store.Close()` to clean up resources
2. **Handle errors**: Check for specific error types when appropriate
3. **Use appropriate store type**: Memory for single-instance, Redis for multi-instance
4. **Monitor connections**: Use `Ping()` for health checks
5. **Manage subscriptions**: Always unsubscribe from event channels when done
6. **Token management**: Set appropriate TTL values for screen tokens

## Dependencies

- `github.com/redis/go-redis/v9` - Redis client library
- `kube-ops-view/internal/models` - Data models for cluster information
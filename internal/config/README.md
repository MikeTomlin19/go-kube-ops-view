# Configuration Management

This package provides a comprehensive configuration management system for the Kube Ops View application. It supports loading configuration from multiple sources with a clear precedence order.

## Features

- **Multiple Sources**: Load configuration from default values, YAML files, environment variables, and CLI flags
- **Precedence Order**: CLI flags > Environment variables > Configuration files > Default values
- **Validation**: Built-in validation with detailed error messages
- **Type Safety**: Strongly typed configuration with proper Go types
- **Testing Support**: Special configuration loading for tests

## Configuration Sources

### 1. Default Values

The system starts with sensible defaults:

```go
config := config.New()
// Server runs on 0.0.0.0:8080 with mock clusters enabled
```

### 2. Configuration Files

Load from YAML files:

```yaml
# config.yaml
server:
  port: 8080
  host: "0.0.0.0"
  debug: false

clusters:
  urls:
    - "https://k8s-cluster1.example.com"
    - "https://k8s-cluster2.example.com"
  query_interval: "5s"

auth:
  enabled: true
  authorize_url: "https://auth.example.com/authorize"
  token_url: "https://auth.example.com/token"
```

Default file locations (searched in order):
- `./config.yaml`
- `./config.yml`
- `./kube-ops-view.yaml`
- `./kube-ops-view.yml`
- `/etc/kube-ops-view/config.yaml`
- `/etc/kube-ops-view/config.yml`
- `~/.kube-ops-view.yaml`
- `~/.kube-ops-view.yml`
- `~/.config/kube-ops-view/config.yaml`
- `~/.config/kube-ops-view/config.yml`

### 3. Environment Variables

All configuration options can be set via environment variables:

```bash
export SERVER_PORT=9090
export DEBUG=true
export CLUSTERS="https://k8s1.example.com,https://k8s2.example.com"
export OAUTH2_ENABLED=true
export OAUTH2_CLIENT_ID="your-client-id"
export REDIS_URL="redis://localhost:6379"
```

### 4. CLI Flags

Use with Cobra commands:

```go
cfg := config.New()
cmd := &cobra.Command{
    Use: "kube-ops-view",
    Run: func(cmd *cobra.Command, args []string) {
        // Configuration is automatically populated from flags
    },
}
cfg.AddFlags(cmd)
```

## Usage Examples

### Basic Usage

```go
// Load with default options (includes validation)
cfg, err := config.LoadDefault()
if err != nil {
    log.Fatal(err)
}

// Use configuration
server := gin.New()
server.Run(cfg.GetServerAddress())
```

### Load from Specific File

```go
cfg, err := config.LoadWithFile("/path/to/config.yaml")
if err != nil {
    log.Fatal(err)
}
```

### Advanced Loading

```go
cfg, err := config.Load(config.LoadOptions{
    ConfigFile:      "/custom/config.yaml",
    SkipEnv:         false,  // Load from environment
    SkipDefaultFile: false,  // Try default file locations
    Validate:        true,   // Validate configuration
})
```

### Testing Configuration

```go
cfg, err := config.LoadForTesting()
// Returns configuration suitable for tests:
// - Random port (0)
// - Mock clusters enabled
// - Authentication disabled
// - Memory storage
```

## Configuration Structure

### Server Configuration

```go
type ServerConfig struct {
    Port        int    // Server port (1-65535)
    Host        string // Server host
    RoutePrefix string // Route prefix for reverse proxies
    Debug       bool   // Enable debug mode
    SecretKey   string // Secret key for sessions
}
```

### Cluster Configuration

```go
type ClusterConfig struct {
    URLs            []string      // Static cluster URLs
    RegistryURL     string        // Cluster registry URL
    KubeconfigPath  string        // Path to kubeconfig file
    Contexts        []string      // Kubeconfig contexts to use
    QueryInterval   time.Duration // Cluster polling interval
    ConnectTimeout  time.Duration // Connection timeout
    ReadTimeout     time.Duration // Read timeout
    Mock            bool          // Use mock data
}
```

### Authentication Configuration

```go
type AuthConfig struct {
    Enabled         bool   // Enable OAuth2 authentication
    AuthorizeURL    string // OAuth2 authorization URL
    TokenURL        string // OAuth2 token URL
    ClientID        string // OAuth2 client ID
    ClientSecret    string // OAuth2 client secret
    Scope           string // OAuth2 scope
    CredentialsDir  string // Directory for credentials
    ScreenTokens    bool   // Enable screen tokens
}
```

### Storage Configuration

```go
type StorageConfig struct {
    Type     string // "memory" or "redis"
    RedisURL string // Redis connection URL
    RedisDB  int    // Redis database number
}
```

### UI Configuration

```go
type UIConfig struct {
    URLTemplate    string // URL template for external links
    NodeLinkURL    string // Node link URL template
    PodLinkURL     string // Pod link URL template
    Theme          string // UI theme
    ShowCapacity   bool   // Show node capacity
    ShowRequests   bool   // Show resource requests
    ShowLimits     bool   // Show resource limits
    ShowUsage      bool   // Show resource usage
}
```

## Environment Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `SERVER_PORT` | int | 8080 | Server port |
| `HOST` | string | "0.0.0.0" | Server host |
| `ROUTE_PREFIX` | string | "/" | Route prefix |
| `DEBUG` | bool | false | Debug mode |
| `SECRET_KEY` | string | "development" | Session secret |
| `CLUSTERS` | []string | | Comma-separated cluster URLs |
| `CLUSTER_REGISTRY_URL` | string | | Cluster registry URL |
| `KUBECONFIG_PATH` | string | | Kubeconfig file path |
| `KUBECONFIG_CONTEXTS` | []string | | Comma-separated contexts |
| `QUERY_INTERVAL` | duration | "5s" | Cluster query interval |
| `CONNECT_TIMEOUT` | duration | "10s" | Connection timeout |
| `READ_TIMEOUT` | duration | "10s" | Read timeout |
| `MOCK` | bool | true | Use mock clusters |
| `OAUTH2_ENABLED` | bool | false | Enable OAuth2 |
| `OAUTH2_AUTHORIZE_URL` | string | | OAuth2 authorize URL |
| `OAUTH2_TOKEN_URL` | string | | OAuth2 token URL |
| `OAUTH2_CLIENT_ID` | string | | OAuth2 client ID |
| `OAUTH2_CLIENT_SECRET` | string | | OAuth2 client secret |
| `OAUTH2_SCOPE` | string | "openid" | OAuth2 scope |
| `CREDENTIALS_DIR` | string | | Credentials directory |
| `SCREEN_TOKENS` | bool | false | Enable screen tokens |
| `STORAGE_TYPE` | string | "memory" | Storage type |
| `REDIS_URL` | string | | Redis connection URL |
| `REDIS_DB` | int | 0 | Redis database |
| `URL_TEMPLATE` | string | | External URL template |
| `NODE_LINK_URL_TEMPLATE` | string | | Node link template |
| `POD_LINK_URL_TEMPLATE` | string | | Pod link template |
| `THEME` | string | "default" | UI theme |
| `SHOW_CAPACITY` | bool | true | Show capacity |
| `SHOW_REQUESTS` | bool | true | Show requests |
| `SHOW_LIMITS` | bool | true | Show limits |
| `SHOW_USAGE` | bool | true | Show usage |

## Validation

The configuration system includes comprehensive validation:

- **Server**: Port range (1-65535), non-empty secret key, valid route prefix
- **Clusters**: At least one cluster source, minimum intervals (1s)
- **Auth**: Required fields when authentication is enabled
- **Storage**: Valid storage type, Redis URL when using Redis
- **UI**: Valid theme selection

## Helper Methods

```go
// Check configuration state
cfg.IsAuthEnabled()     // Returns true if OAuth2 is enabled
cfg.IsRedisEnabled()    // Returns true if Redis storage is configured
cfg.GetServerAddress()  // Returns "host:port" string
cfg.GetClusterSources() // Returns list of configured cluster sources

// Print configuration (masks sensitive data)
cfg.PrintConfiguration()

// Validate configuration
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}
```
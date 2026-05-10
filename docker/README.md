# Docker Setup for Kube Ops View (Go)

This directory contains Docker configuration files and documentation for running the Go version of Kube Ops View in containers.

## Quick Start

### Local Development with Docker Compose

1. **Start the application with Redis:**
   ```bash
   docker-compose up -d
   ```

2. **Access the application:**
   - Main application: http://localhost:8080
   - Redis Commander (optional): http://localhost:8081

3. **Stop the application:**
   ```bash
   docker-compose down
   ```

### Building the Docker Image

1. **Build using the build script (recommended):**
   ```bash
   ./scripts/build-docker.sh
   ```

2. **Build manually:**
   ```bash
   docker build -f Dockerfile -t go-kube-ops-view:latest .
   ```

3. **Build for multiple platforms:**
   ```bash
   ./scripts/build-docker.sh --platform linux/amd64,linux/arm64
   ```

## Configuration

### Environment Variables

The Go application supports configuration through environment variables:

#### Server Configuration
- `SERVER_PORT`: Port to listen on (default: 8080)
- `DEBUG`: Enable debug mode (default: false)
- `SECRET_KEY`: Secret key for session management (required in production)
- `ROUTE_PREFIX`: Route prefix for the application (default: /)

#### Cluster Configuration
- `CLUSTERS`: Comma-separated list of cluster API server URLs
- `CLUSTER_REGISTRY_URL`: URL of cluster registry endpoint
- `KUBECONFIG_PATH`: Path to kubeconfig file
- `KUBECONFIG_CONTEXTS`: Comma-separated list of kubeconfig contexts
- `QUERY_INTERVAL`: Interval for querying cluster data (default: 5s)
- `MOCK`: Use mock data instead of real clusters (default: false)

#### Storage Configuration
- `REDIS_URL`: Redis URL for pub/sub and job locking
- `REDIS_PASSWORD`: Redis password
- `REDIS_DB`: Redis database number (default: 0)

#### Authentication Configuration
- `OAUTH_AUTHORIZE_URL`: OAuth2 authorization URL
- `OAUTH_TOKEN_URL`: OAuth2 token URL
- `OAUTH_CLIENT_ID`: OAuth2 client ID
- `OAUTH_CLIENT_SECRET`: OAuth2 client secret
- `OAUTH_SCOPE`: OAuth2 scope

#### Logging Configuration
- `LOG_LEVEL`: Log level (debug, info, warn, error) (default: info)
- `LOG_FORMAT`: Log format (json, text) (default: json)
- `LOG_OUTPUT`: Log output (stdout, stderr, or file path) (default: stdout)

### Docker Compose Configurations

#### Development (docker-compose.yml + docker-compose.override.yml)
- Includes Redis for development
- Enables debug mode and verbose logging
- Mounts kubeconfig for local cluster access
- Includes Redis Commander for Redis management

#### Production (docker-compose.prod.yml)
- Optimized for production deployment
- Includes resource limits and health checks
- Uses JSON logging format
- Requires explicit configuration of secrets

### Example Configurations

#### Local Development with Mock Data
```yaml
services:
  kube-ops-view:
    environment:
      - MOCK=true
      - DEBUG=true
      - LOG_LEVEL=debug
```

#### Production with Real Clusters
```yaml
services:
  kube-ops-view:
    environment:
      - CLUSTERS=https://cluster1.example.com,https://cluster2.example.com
      - SECRET_KEY=${SECRET_KEY}
      - REDIS_URL=redis://redis:6379
      - LOG_LEVEL=info
      - LOG_FORMAT=json
```

#### With OAuth Authentication
```yaml
services:
  kube-ops-view:
    environment:
      - OAUTH_AUTHORIZE_URL=https://oauth.example.com/auth
      - OAUTH_TOKEN_URL=https://oauth.example.com/token
      - OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID}
      - OAUTH_CLIENT_SECRET=${OAUTH_CLIENT_SECRET}
      - OAUTH_SCOPE=openid profile
```

## Security

### Container Security Features

The Docker image implements several security best practices:

1. **Non-root user**: Runs as user ID 1001 (appuser)
2. **Read-only root filesystem**: Prevents runtime modifications
3. **Minimal base image**: Uses Alpine Linux for smaller attack surface
4. **No unnecessary packages**: Only includes required runtime dependencies
5. **Security context**: Drops all capabilities and uses seccomp profile

### Kubernetes Security

The Kubernetes deployment includes additional security measures:

1. **Security contexts**: Pod and container security contexts
2. **Read-only root filesystem**: Enforced at container level
3. **Non-root user**: Enforced with runAsNonRoot
4. **Capability dropping**: Drops all Linux capabilities
5. **Seccomp profile**: Uses RuntimeDefault seccomp profile

### Secrets Management

For production deployments, use Kubernetes secrets for sensitive data:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kube-ops-view-secret
type: Opaque
data:
  secret-key: <base64-encoded-secret>
  oauth-client-secret: <base64-encoded-oauth-secret>
  redis-password: <base64-encoded-redis-password>
```

## Deployment

### Kubernetes Deployment

1. **Apply RBAC configuration:**
   ```bash
   kubectl apply -f deploy/rbac.yaml
   ```

2. **Deploy Redis (if using):**
   ```bash
   kubectl apply -f deploy/redis-deployment.yaml
   kubectl apply -f deploy/redis-service.yaml
   ```

3. **Deploy the application:**
   ```bash
   kubectl apply -f deploy/deployment.yaml
   kubectl apply -f deploy/service.yaml
   ```

4. **Using Kustomize:**
   ```bash
   kubectl apply -k deploy/
   ```

### Helm Deployment (Future)

A Helm chart will be provided in future versions for easier deployment and configuration management.

## Monitoring and Observability

### Health Checks

The application provides health check endpoints:

- `/health`: Basic health check
- `/metrics`: Prometheus metrics (if enabled)

### Logging

The application supports structured logging:

- **JSON format**: For production log aggregation
- **Text format**: For development and debugging
- **Configurable levels**: debug, info, warn, error

### Metrics

Prometheus metrics are available for monitoring:

- HTTP request metrics
- Cluster query performance
- Resource usage metrics
- Connection pool metrics

## Troubleshooting

### Common Issues

1. **Container fails to start:**
   - Check logs: `docker logs <container-name>`
   - Verify configuration: `docker exec <container-name> env`
   - Check health endpoint: `curl http://localhost:8080/health`

2. **Cannot connect to clusters:**
   - Verify kubeconfig mounting
   - Check cluster URLs and authentication
   - Review RBAC permissions

3. **Redis connection issues:**
   - Verify Redis URL and credentials
   - Check network connectivity
   - Review Redis logs

4. **Authentication problems:**
   - Verify OAuth configuration
   - Check client credentials
   - Review authentication logs

### Debug Mode

Enable debug mode for detailed logging:

```bash
docker run -e DEBUG=true -e LOG_LEVEL=debug go-kube-ops-view:latest
```

### Testing

Run the test suite to verify container functionality:

```bash
./scripts/test-docker.sh
```

## Performance Tuning

### Resource Limits

Recommended resource limits for different deployment sizes:

#### Small Deployment (1-3 clusters, <100 nodes)
```yaml
resources:
  limits:
    cpu: 200m
    memory: 128Mi
  requests:
    cpu: 50m
    memory: 64Mi
```

#### Medium Deployment (4-10 clusters, 100-500 nodes)
```yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

#### Large Deployment (10+ clusters, 500+ nodes)
```yaml
resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

### Redis Configuration

For high-throughput deployments, tune Redis configuration:

```conf
# Increase memory limit
maxmemory 512mb
maxmemory-policy allkeys-lru

# Optimize for pub/sub
client-output-buffer-limit pubsub 32mb 8mb 60
```

## Migration from Python Version

### Configuration Mapping

| Python Flag | Go Flag | Environment Variable |
|-------------|---------|---------------------|
| `--port` | `--port` | `SERVER_PORT` |
| `--redis-url` | `--redis-url` | `REDIS_URL` |
| `--clusters` | `--clusters` | `CLUSTERS` |
| `--kubeconfig-path` | `--kubeconfig-path` | `KUBECONFIG_PATH` |
| `--oauth-*` | `--oauth-*` | `OAUTH_*` |

### Docker Image Migration

Replace the Python image with the Go image:

```yaml
# Before (Python)
image: go-kube-ops-view:23.5.0

# After (Go)
image: go-kube-ops-view:latest
```

### Volume Mounts

The Go version uses the same volume mount patterns:

```yaml
volumeMounts:
- name: kubeconfig
  mountPath: /home/appuser/.kube
  readOnly: true
```

## Contributing

### Building Development Images

For development, build images with debug symbols:

```bash
./scripts/build-docker.sh --version dev --platform linux/amd64
```

### Testing Changes

Always run the test suite after making changes:

```bash
./scripts/test-docker.sh --image kube-ops-view:dev
```

### Multi-platform Builds

For releases, build for multiple platforms:

```bash
./scripts/build-docker.sh --platform linux/amd64,linux/arm64 --push
```

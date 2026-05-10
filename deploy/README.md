# Kube Ops View Deployment Guide

This directory contains deployment configurations for the Go version of Kube Ops View. Multiple deployment methods are provided to support different environments and preferences.

## Deployment Methods

### 1. Kustomize Deployment (Recommended)

Kustomize provides a declarative way to customize Kubernetes configurations for different environments.

#### Quick Start with Production Configuration

```bash
# Deploy to production namespace
kubectl apply -k deploy/production/

# Check deployment status
kubectl get pods -n kube-ops-view
```

#### Environment-Specific Deployments

```bash
# Development environment
kubectl apply -k deploy/overlays/development/

# Staging environment
kubectl apply -k deploy/overlays/staging/

# Production environment (same as above)
kubectl apply -k deploy/production/
```

### 2. Helm Chart Deployment

Helm provides templating and package management for Kubernetes applications.

#### Install with Helm

```bash
# Add the chart repository (if published)
helm repo add kube-ops-view https://charts.kube-ops-view.io
helm repo update

# Or install from local chart
cd deploy/helm/kube-ops-view

# Install with default values
helm install kube-ops-view . -n kube-ops-view --create-namespace

# Install with production values
helm install kube-ops-view . -n kube-ops-view --create-namespace -f values-production.yaml

# Install with development values
helm install kube-ops-view . -n kube-ops-view --create-namespace -f values-development.yaml
```

#### Upgrade Deployment

```bash
# Upgrade with new values
helm upgrade kube-ops-view . -n kube-ops-view -f values-production.yaml

# Rollback if needed
helm rollback kube-ops-view 1 -n kube-ops-view
```

### 3. Direct Kubectl Deployment

For simple deployments or testing, you can apply manifests directly.

```bash
# Apply all production manifests
kubectl apply -f deploy/production/

# Or apply individual components
kubectl apply -f deploy/production/namespace.yaml
kubectl apply -f deploy/production/rbac.yaml
kubectl apply -f deploy/production/configmap.yaml
kubectl apply -f deploy/production/secret.yaml
kubectl apply -f deploy/production/deployment.yaml
kubectl apply -f deploy/production/service.yaml
```

## Configuration

### Environment Variables

The application can be configured using environment variables or command-line flags:

| Environment Variable | CLI Flag | Default | Description |
|---------------------|----------|---------|-------------|
| `SERVER_PORT` | `--port` | `8080` | Server port |
| `ROUTE_PREFIX` | `--route-prefix` | `/` | Route prefix |
| `DEBUG` | `--debug` | `false` | Enable debug mode |
| `LOG_LEVEL` | `--log-level` | `info` | Log level |
| `LOG_FORMAT` | `--log-format` | `json` | Log format |
| `CLUSTERS` | `--clusters` | | Comma-separated cluster URLs |
| `QUERY_INTERVAL` | `--query-interval` | `5s` | Cluster query interval |
| `REDIS_URL` | `--redis-url` | | Redis URL for pub/sub |
| `SECRET_KEY` | `--secret-key` | | Session secret key |
| `OAUTH_CLIENT_ID` | `--oauth-client-id` | | OAuth client ID |
| `OAUTH_CLIENT_SECRET` | `--oauth-client-secret` | | OAuth client secret |

### Secrets Management

#### Production Secrets

In production, secrets should be managed securely:

```bash
# Create secret key
kubectl create secret generic kube-ops-view-secret \
  --from-literal=secret-key="$(openssl rand -base64 32)" \
  -n kube-ops-view

# Create OAuth secret (if using OAuth)
kubectl create secret generic kube-ops-view-oauth \
  --from-literal=client-secret="your-oauth-client-secret" \
  -n kube-ops-view

# Create Redis password (if using Redis auth)
kubectl create secret generic kube-ops-view-redis \
  --from-literal=password="$(openssl rand -base64 32)" \
  -n kube-ops-view
```

#### External Secrets Operator

For advanced secret management, use External Secrets Operator:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-secret-store
  namespace: kube-ops-view
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "kube-ops-view"
```

### RBAC Configuration

The application requires specific RBAC permissions to access Kubernetes resources:

- **Cluster-level permissions:**
  - `nodes`: get, list, watch
  - `pods`: get, list, watch
  - `namespaces`: get, list, watch
  - `metrics.k8s.io/nodes`: get, list
  - `metrics.k8s.io/pods`: get, list

- **Namespace-level permissions:**
  - `secrets`: get (for OAuth and app secrets)

### Multi-Cluster Configuration

#### In-Cluster Access (Default)

By default, the application uses the in-cluster service account to access the current cluster:

```yaml
env:
- name: CLUSTERS
  value: "https://kubernetes.default.svc"
```

#### External Clusters via Kubeconfig

To access external clusters, provide a kubeconfig file:

```bash
# Create kubeconfig secret
kubectl create secret generic kube-ops-view-kubeconfig \
  --from-file=config=/path/to/kubeconfig \
  -n kube-ops-view
```

```yaml
env:
- name: KUBECONFIG_PATH
  value: "/etc/kubeconfig/config"
- name: KUBECONFIG_CONTEXTS
  value: "cluster1,cluster2,cluster3"
```

#### Cluster Registry

For dynamic cluster discovery, use a cluster registry:

```yaml
env:
- name: CLUSTER_REGISTRY_URL
  value: "https://cluster-registry.example.com/api/v1/clusters"
```

## Monitoring and Observability

### Prometheus Metrics

The application exposes Prometheus metrics on `/metrics`:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Health Checks

Health check endpoints are available:

- `/health` - General health check
- `/ready` - Readiness check
- `/live` - Liveness check

### Logging

Structured logging is configured via environment variables:

```yaml
env:
- name: LOG_LEVEL
  value: "info"
- name: LOG_FORMAT
  value: "json"
- name: LOG_OUTPUT
  value: "stdout"
```

## Security Considerations

### Pod Security

- Runs as non-root user (UID 1001)
- Read-only root filesystem
- Drops all capabilities
- Uses seccomp profile

### Network Security

Network policies are provided to restrict traffic:

```bash
# Apply network policies
kubectl apply -f deploy/production/networkpolicy.yaml
```

### Resource Limits

Appropriate resource limits are set:

```yaml
resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 200m
    memory: 256Mi
```

## Troubleshooting

### Common Issues

1. **RBAC Permissions**
   ```bash
   # Check service account permissions
   kubectl auth can-i list nodes --as=system:serviceaccount:kube-ops-view:kube-ops-view
   ```

2. **Redis Connection**
   ```bash
   # Check Redis connectivity
   kubectl exec -it deployment/kube-ops-view-redis -n kube-ops-view -- redis-cli ping
   ```

3. **Cluster Access**
   ```bash
   # Check cluster connectivity
   kubectl logs deployment/kube-ops-view -n kube-ops-view
   ```

### Debug Mode

Enable debug mode for troubleshooting:

```yaml
env:
- name: DEBUG
  value: "true"
- name: LOG_LEVEL
  value: "debug"
```

## Scaling

### Horizontal Scaling

The application supports horizontal scaling with Redis:

```bash
# Scale up replicas
kubectl scale deployment kube-ops-view --replicas=5 -n kube-ops-view
```

### Resource Scaling

Adjust resources based on cluster size:

- Small clusters (< 100 nodes): 200m CPU, 256Mi memory
- Medium clusters (100-500 nodes): 500m CPU, 512Mi memory
- Large clusters (> 500 nodes): 1000m CPU, 1Gi memory

## Backup and Recovery

### Redis Data Backup

If using persistent Redis:

```bash
# Create backup
kubectl exec deployment/kube-ops-view-redis -n kube-ops-view -- redis-cli BGSAVE

# Copy backup
kubectl cp kube-ops-view/kube-ops-view-redis-xxx:/data/dump.rdb ./backup-$(date +%Y%m%d).rdb
```

### Configuration Backup

```bash
# Backup all configurations
kubectl get all,configmap,secret,pvc -n kube-ops-view -o yaml > kube-ops-view-backup.yaml
```
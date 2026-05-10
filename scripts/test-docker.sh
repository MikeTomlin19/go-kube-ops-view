#!/bin/bash

# Test script for kube-ops-view Docker container
set -euo pipefail

# Configuration
IMAGE_NAME=${IMAGE_NAME:-"go-kube-ops-view:latest"}
CONTAINER_NAME="kube-ops-view-test"
TEST_PORT=${TEST_PORT:-"8080"}
TIMEOUT=${TIMEOUT:-"30"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

info() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

# Cleanup function
cleanup() {
    log "Cleaning up test containers..."
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker stop "${CONTAINER_NAME}-redis" >/dev/null 2>&1 || true
    docker rm "${CONTAINER_NAME}-redis" >/dev/null 2>&1 || true
    docker network rm "${CONTAINER_NAME}-network" >/dev/null 2>&1 || true
}

# Set up cleanup trap
trap cleanup EXIT

# Function to wait for container to be ready
wait_for_container() {
    local container_name=$1
    local port=$2
    local max_attempts=$((TIMEOUT / 2))
    local attempt=1

    log "Waiting for container $container_name to be ready on port $port..."

    while [[ $attempt -le $max_attempts ]]; do
        if curl -f -s "http://localhost:$port/health" >/dev/null 2>&1; then
            log "Container $container_name is ready!"
            return 0
        fi

        info "Attempt $attempt/$max_attempts - waiting for container..."
        sleep 2
        ((attempt++))
    done

    error "Container $container_name failed to become ready within $TIMEOUT seconds"
}

# Function to test HTTP endpoints
test_endpoints() {
    local port=$1
    local base_url="http://localhost:$port"

    log "Testing HTTP endpoints..."

    # Test health endpoint
    info "Testing health endpoint..."
    if curl -f -s "$base_url/health" | grep -q "ok\|healthy\|ready"; then
        log "✓ Health endpoint test passed"
    else
        error "✗ Health endpoint test failed"
    fi

    # Test main endpoint
    info "Testing main endpoint..."
    if curl -f -s "$base_url/" >/dev/null; then
        log "✓ Main endpoint test passed"
    else
        warn "✗ Main endpoint test failed (may be expected without clusters)"
    fi

    # Test events endpoint (SSE)
    info "Testing events endpoint..."
    if timeout 5 curl -s "$base_url/events" | head -n 1 | grep -q "data:\|event:"; then
        log "✓ Events endpoint test passed"
    else
        warn "✗ Events endpoint test failed (may be expected without clusters)"
    fi

    # Test static assets
    info "Testing static assets..."
    if curl -f -s "$base_url/static/favicon.ico" >/dev/null; then
        log "✓ Static assets test passed"
    else
        warn "✗ Static assets test failed"
    fi
}

# Function to test container security
test_security() {
    local container_name=$1

    log "Testing container security..."

    # Check if running as non-root
    local user_id
    user_id=$(docker exec "$container_name" id -u)
    if [[ "$user_id" != "0" ]]; then
        log "✓ Container running as non-root user (UID: $user_id)"
    else
        error "✗ Container running as root user"
    fi

    # Check if filesystem is read-only
    if docker exec "$container_name" touch /test-file 2>/dev/null; then
        warn "✗ Root filesystem is not read-only"
        docker exec "$container_name" rm -f /test-file 2>/dev/null || true
    else
        log "✓ Root filesystem is read-only"
    fi

    # Check process list
    info "Container processes:"
    docker exec "$container_name" ps aux || true
}

# Function to test resource usage
test_resources() {
    local container_name=$1

    log "Testing resource usage..."

    # Get container stats
    local stats
    stats=$(docker stats "$container_name" --no-stream --format "table {{.CPUPerc}}\t{{.MemUsage}}")
    info "Container resource usage:"
    echo "$stats"

    # Check memory usage (should be reasonable)
    local memory_usage
    memory_usage=$(docker stats "$container_name" --no-stream --format "{{.MemUsage}}" | cut -d'/' -f1 | sed 's/MiB//')
    if [[ $(echo "$memory_usage < 200" | bc -l) -eq 1 ]]; then
        log "✓ Memory usage is reasonable ($memory_usage MiB)"
    else
        warn "✗ Memory usage is high ($memory_usage MiB)"
    fi
}

# Function to test graceful shutdown
test_graceful_shutdown() {
    local container_name=$1

    log "Testing graceful shutdown..."

    # Send SIGTERM and check if container stops gracefully
    docker kill --signal=TERM "$container_name"

    # Wait for container to stop
    local timeout=10
    local count=0
    while docker ps | grep -q "$container_name" && [[ $count -lt $timeout ]]; do
        sleep 1
        ((count++))
    done

    if docker ps | grep -q "$container_name"; then
        warn "✗ Container did not stop gracefully within $timeout seconds"
        docker kill "$container_name"
    else
        log "✓ Container stopped gracefully"
    fi
}

# Function to test with Redis
test_with_redis() {
    log "Testing with Redis backend..."

    # Create network
    docker network create "${CONTAINER_NAME}-network" >/dev/null 2>&1 || true

    # Start Redis container
    info "Starting Redis container..."
    docker run -d \
        --name "${CONTAINER_NAME}-redis" \
        --network "${CONTAINER_NAME}-network" \
        redis:7-alpine \
        redis-server --appendonly yes

    # Wait for Redis to be ready
    sleep 5

    # Start application container with Redis
    info "Starting application container with Redis..."
    docker run -d \
        --name "$CONTAINER_NAME" \
        --network "${CONTAINER_NAME}-network" \
        -p "$TEST_PORT:8080" \
        -e REDIS_URL="redis://${CONTAINER_NAME}-redis:6379" \
        -e MOCK=true \
        -e DEBUG=true \
        "$IMAGE_NAME"

    # Wait for application to be ready
    wait_for_container "$CONTAINER_NAME" "$TEST_PORT"

    # Test endpoints
    test_endpoints "$TEST_PORT"

    # Test security
    test_security "$CONTAINER_NAME"

    # Test resources
    test_resources "$CONTAINER_NAME"

    log "Redis integration test completed successfully"
}

# Function to test standalone mode
test_standalone() {
    log "Testing standalone mode (without Redis)..."

    # Start container in standalone mode
    info "Starting container in standalone mode..."
    docker run -d \
        --name "$CONTAINER_NAME" \
        -p "$TEST_PORT:8080" \
        -e MOCK=true \
        -e DEBUG=true \
        "$IMAGE_NAME"

    # Wait for container to be ready
    wait_for_container "$CONTAINER_NAME" "$TEST_PORT"

    # Test endpoints
    test_endpoints "$TEST_PORT"

    # Test security
    test_security "$CONTAINER_NAME"

    # Test resources
    test_resources "$CONTAINER_NAME"

    # Test graceful shutdown
    test_graceful_shutdown "$CONTAINER_NAME"

    log "Standalone test completed successfully"
}

# Function to test Kubernetes deployment
test_kubernetes_deployment() {
    log "Testing Kubernetes deployment compatibility..."

    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        warn "kubectl not found, skipping Kubernetes deployment test"
        return 0
    fi

    # Check if we have a Kubernetes cluster
    if ! kubectl cluster-info >/dev/null 2>&1; then
        warn "No Kubernetes cluster available, skipping deployment test"
        return 0
    fi

    info "Testing deployment with existing Kubernetes manifests..."

    # Create temporary namespace
    local test_namespace="kube-ops-view-test-$(date +%s)"
    kubectl create namespace "$test_namespace"

    # Cleanup function for Kubernetes resources
    cleanup_k8s() {
        kubectl delete namespace "$test_namespace" --ignore-not-found=true
    }
    trap cleanup_k8s EXIT

    # Apply manifests with image override
    kubectl apply -n "$test_namespace" -f deploy/rbac.yaml
    kubectl apply -n "$test_namespace" -f deploy/service.yaml
    kubectl apply -n "$test_namespace" -f deploy/redis-deployment.yaml
    kubectl apply -n "$test_namespace" -f deploy/redis-service.yaml

    # Apply Go deployment with image override
    sed "s|image: .*|image: $IMAGE_NAME|" deploy/deployment.yaml | \
        kubectl apply -n "$test_namespace" -f -

    # Wait for deployment to be ready
    info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available --timeout=120s \
        deployment/kube-ops-view-go -n "$test_namespace"

    # Test service connectivity
    info "Testing service connectivity..."
    kubectl port-forward -n "$test_namespace" service/kube-ops-view 8081:80 &
    local port_forward_pid=$!

    sleep 5

    if curl -f -s "http://localhost:8081/health" >/dev/null; then
        log "✓ Kubernetes deployment test passed"
    else
        error "✗ Kubernetes deployment test failed"
    fi

    # Cleanup port-forward
    kill $port_forward_pid 2>/dev/null || true

    log "Kubernetes deployment test completed successfully"
}

# Main test function
main() {
    log "Starting Docker container tests for: $IMAGE_NAME"

    # Check if Docker is available
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed or not in PATH"
    fi

    # Check if image exists
    if ! docker image inspect "$IMAGE_NAME" >/dev/null 2>&1; then
        error "Docker image '$IMAGE_NAME' not found. Please build it first."
    fi

    # Run tests
    test_standalone
    cleanup

    test_with_redis
    cleanup

    test_kubernetes_deployment

    log "All Docker container tests completed successfully!"
}

# Help function
show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Test Docker container for kube-ops-view

OPTIONS:
    -h, --help          Show this help message
    -i, --image         Docker image name to test (default: go-kube-ops-view:latest)
    -p, --port          Test port to use (default: 8080)
    -t, --timeout       Timeout for container readiness (default: 30s)

EXAMPLES:
    # Test default image
    $0

    # Test specific image
    $0 --image myregistry/kube-ops-view:v1.0.0

    # Test with custom port and timeout
    $0 --port 9090 --timeout 60
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -i|--image)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -p|--port)
            TEST_PORT="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Run main function
main

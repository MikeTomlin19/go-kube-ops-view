#!/bin/bash

# Kube Ops View Deployment Validation Script
# This script validates that the deployment is working correctly

set -euo pipefail

# Default values
NAMESPACE="kube-ops-view"
TIMEOUT=300
VERBOSE=false
SKIP_CONNECTIVITY=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
Kube Ops View Deployment Validation Script

Usage: $0 [OPTIONS]

OPTIONS:
    -n, --namespace NAMESPACE   Kubernetes namespace (default: kube-ops-view)
    -t, --timeout TIMEOUT      Timeout in seconds (default: 300)
    -v, --verbose              Enable verbose output
    -s, --skip-connectivity    Skip connectivity tests
    -h, --help                 Show this help message

EXAMPLES:
    # Validate deployment in default namespace
    $0

    # Validate with custom namespace and timeout
    $0 -n my-kube-ops-view -t 600

    # Skip connectivity tests
    $0 --skip-connectivity

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -s|--skip-connectivity)
                SKIP_CONNECTIVITY=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi

    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi

    log_success "Prerequisites check passed"
}

# Validate namespace exists
validate_namespace() {
    log_info "Validating namespace: $NAMESPACE"

    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace does not exist: $NAMESPACE"
        return 1
    fi

    log_success "Namespace exists"
}

# Validate RBAC resources
validate_rbac() {
    log_info "Validating RBAC resources..."

    local errors=0

    # Check ServiceAccount
    if ! kubectl get serviceaccount kube-ops-view -n "$NAMESPACE" &> /dev/null; then
        log_error "ServiceAccount not found: kube-ops-view"
        ((errors++))
    else
        log_success "ServiceAccount exists"
    fi

    # Check ClusterRole
    if ! kubectl get clusterrole kube-ops-view &> /dev/null; then
        log_error "ClusterRole not found: kube-ops-view"
        ((errors++))
    else
        log_success "ClusterRole exists"
    fi

    # Check ClusterRoleBinding
    if ! kubectl get clusterrolebinding kube-ops-view &> /dev/null; then
        log_error "ClusterRoleBinding not found: kube-ops-view"
        ((errors++))
    else
        log_success "ClusterRoleBinding exists"
    fi

    # Test permissions
    log_info "Testing RBAC permissions..."
    local sa_name="system:serviceaccount:$NAMESPACE:kube-ops-view"

    local permissions=(
        "list nodes"
        "list pods"
        "list namespaces"
    )

    for permission in "${permissions[@]}"; do
        if kubectl auth can-i $permission --as="$sa_name" &> /dev/null; then
            log_success "Permission granted: $permission"
        else
            log_error "Permission denied: $permission"
            ((errors++))
        fi
    done

    return $errors
}

# Validate ConfigMaps and Secrets
validate_config() {
    log_info "Validating configuration resources..."

    local errors=0

    # Check ConfigMap
    if ! kubectl get configmap -n "$NAMESPACE" | grep -q kube-ops-view; then
        log_warning "No kube-ops-view ConfigMap found"
    else
        log_success "ConfigMap exists"
    fi

    # Check Secrets
    if ! kubectl get secret -n "$NAMESPACE" | grep -q kube-ops-view; then
        log_warning "No kube-ops-view Secrets found"
    else
        log_success "Secrets exist"
    fi

    return $errors
}

# Validate deployments
validate_deployments() {
    log_info "Validating deployments..."

    local errors=0

    # Check main deployment
    if ! kubectl get deployment kube-ops-view -n "$NAMESPACE" &> /dev/null; then
        log_error "Main deployment not found: kube-ops-view"
        ((errors++))
        return $errors
    fi

    log_success "Main deployment exists"

    # Check deployment status
    local ready_replicas=$(kubectl get deployment kube-ops-view -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    local desired_replicas=$(kubectl get deployment kube-ops-view -n "$NAMESPACE" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")

    if [[ "$ready_replicas" == "$desired_replicas" ]] && [[ "$ready_replicas" -gt 0 ]]; then
        log_success "Main deployment is ready ($ready_replicas/$desired_replicas replicas)"
    else
        log_error "Main deployment is not ready ($ready_replicas/$desired_replicas replicas)"
        ((errors++))
    fi

    # Check Redis deployment if exists
    if kubectl get deployment kube-ops-view-redis -n "$NAMESPACE" &> /dev/null; then
        log_info "Checking Redis deployment..."

        local redis_ready=$(kubectl get deployment kube-ops-view-redis -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local redis_desired=$(kubectl get deployment kube-ops-view-redis -n "$NAMESPACE" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "0")

        if [[ "$redis_ready" == "$redis_desired" ]] && [[ "$redis_ready" -gt 0 ]]; then
            log_success "Redis deployment is ready ($redis_ready/$redis_desired replicas)"
        else
            log_error "Redis deployment is not ready ($redis_ready/$redis_desired replicas)"
            ((errors++))
        fi
    else
        log_info "Redis deployment not found (may be using external Redis)"
    fi

    return $errors
}

# Validate pods
validate_pods() {
    log_info "Validating pods..."

    local errors=0

    # Get all kube-ops-view pods
    local pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=kube-ops-view --no-headers 2>/dev/null || echo "")

    if [[ -z "$pods" ]]; then
        log_error "No kube-ops-view pods found"
        return 1
    fi

    # Check each pod
    while IFS= read -r pod_line; do
        if [[ -z "$pod_line" ]]; then
            continue
        fi

        local pod_name=$(echo "$pod_line" | awk '{print $1}')
        local pod_status=$(echo "$pod_line" | awk '{print $3}')
        local pod_ready=$(echo "$pod_line" | awk '{print $2}')

        if [[ "$pod_status" == "Running" ]]; then
            log_success "Pod $pod_name is running ($pod_ready ready)"
        else
            log_error "Pod $pod_name is not running (status: $pod_status)"
            ((errors++))

            if [[ "$VERBOSE" == "true" ]]; then
                log_info "Pod events for $pod_name:"
                kubectl describe pod "$pod_name" -n "$NAMESPACE" | tail -10
            fi
        fi
    done <<< "$pods"

    return $errors
}

# Validate services
validate_services() {
    log_info "Validating services..."

    local errors=0

    # Check main service
    if ! kubectl get service kube-ops-view -n "$NAMESPACE" &> /dev/null; then
        log_error "Main service not found: kube-ops-view"
        ((errors++))
    else
        log_success "Main service exists"

        # Check service endpoints
        local endpoints=$(kubectl get endpoints kube-ops-view -n "$NAMESPACE" -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || echo "")
        if [[ -n "$endpoints" ]]; then
            local endpoint_count=$(echo "$endpoints" | wc -w)
            log_success "Service has $endpoint_count endpoint(s)"
        else
            log_error "Service has no endpoints"
            ((errors++))
        fi
    fi

    # Check Redis service if exists
    if kubectl get service kube-ops-view-redis -n "$NAMESPACE" &> /dev/null; then
        log_success "Redis service exists"
    else
        log_info "Redis service not found (may be using external Redis)"
    fi

    return $errors
}

# Test health endpoints
test_health_endpoints() {
    if [[ "$SKIP_CONNECTIVITY" == "true" ]]; then
        log_info "Skipping connectivity tests"
        return 0
    fi

    log_info "Testing health endpoints..."

    local errors=0

    # Get a pod to test against
    local pod_name=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=kube-ops-view,app.kubernetes.io/component=frontend --no-headers | head -1 | awk '{print $1}' 2>/dev/null || echo "")

    if [[ -z "$pod_name" ]]; then
        log_error "No frontend pods found for testing"
        return 1
    fi

    log_info "Testing health endpoints on pod: $pod_name"

    # Test health endpoint
    if kubectl exec "$pod_name" -n "$NAMESPACE" -- wget -q -O - http://localhost:8080/health &> /dev/null; then
        log_success "Health endpoint is responding"
    else
        log_error "Health endpoint is not responding"
        ((errors++))
    fi

    # Test main page
    if kubectl exec "$pod_name" -n "$NAMESPACE" -- wget -q -O - http://localhost:8080/ | grep -q "kube-ops-view" &> /dev/null; then
        log_success "Main page is responding"
    else
        log_error "Main page is not responding correctly"
        ((errors++))
    fi

    return $errors
}

# Test Redis connectivity
test_redis_connectivity() {
    if [[ "$SKIP_CONNECTIVITY" == "true" ]]; then
        return 0
    fi

    # Check if Redis deployment exists
    if ! kubectl get deployment kube-ops-view-redis -n "$NAMESPACE" &> /dev/null; then
        log_info "Redis deployment not found, skipping Redis connectivity test"
        return 0
    fi

    log_info "Testing Redis connectivity..."

    local redis_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=redis --no-headers | head -1 | awk '{print $1}' 2>/dev/null || echo "")

    if [[ -z "$redis_pod" ]]; then
        log_error "No Redis pods found for testing"
        return 1
    fi

    if kubectl exec "$redis_pod" -n "$NAMESPACE" -- redis-cli ping | grep -q "PONG" &> /dev/null; then
        log_success "Redis is responding to ping"
        return 0
    else
        log_error "Redis is not responding to ping"
        return 1
    fi
}

# Check resource usage
check_resource_usage() {
    log_info "Checking resource usage..."

    # Get pod resource usage if metrics-server is available
    if kubectl top pods -n "$NAMESPACE" &> /dev/null; then
        log_info "Pod resource usage:"
        kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/name=kube-ops-view
        echo
    else
        log_warning "Metrics server not available, cannot show resource usage"
    fi

    # Check for resource limit violations
    local pods_with_issues=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=kube-ops-view -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[*].restartCount}{"\n"}{end}' | awk '$2 > 0 {print $1}')

    if [[ -n "$pods_with_issues" ]]; then
        log_warning "Pods with restarts detected:"
        echo "$pods_with_issues"
    else
        log_success "No pods with restart issues"
    fi
}

# Generate summary report
generate_summary() {
    local total_errors=$1

    echo
    log_info "=== Validation Summary ==="

    if [[ $total_errors -eq 0 ]]; then
        log_success "All validation checks passed!"
        log_info "Kube Ops View deployment is healthy and ready"
    else
        log_error "Validation failed with $total_errors error(s)"
        log_info "Please review the errors above and fix the issues"
    fi

    # Show access information
    echo
    log_info "=== Access Information ==="

    local service_type=$(kubectl get service kube-ops-view -n "$NAMESPACE" -o jsonpath='{.spec.type}' 2>/dev/null || echo "")

    case $service_type in
        LoadBalancer)
            local external_ip=$(kubectl get service kube-ops-view -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")
            if [[ -n "$external_ip" ]]; then
                log_info "External URL: http://$external_ip"
            else
                log_info "LoadBalancer IP is pending..."
            fi
            ;;
        NodePort)
            local node_port=$(kubectl get service kube-ops-view -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
            if [[ -n "$node_port" ]]; then
                log_info "NodePort access: http://<node-ip>:$node_port"
            fi
            ;;
        ClusterIP)
            log_info "Port-forward command: kubectl port-forward service/kube-ops-view 8080:80 -n $NAMESPACE"
            ;;
    esac

    # Check for ingress
    if kubectl get ingress -n "$NAMESPACE" &> /dev/null; then
        log_info "Ingress resources:"
        kubectl get ingress -n "$NAMESPACE"
    fi

    return $total_errors
}

# Main function
main() {
    log_info "Starting Kube Ops View deployment validation..."
    log_info "Namespace: $NAMESPACE"
    log_info "Timeout: ${TIMEOUT}s"

    local total_errors=0

    check_prerequisites

    # Run validation checks
    validate_namespace || ((total_errors++))
    validate_rbac || ((total_errors++))
    validate_config || ((total_errors++))
    validate_deployments || ((total_errors++))
    validate_pods || ((total_errors++))
    validate_services || ((total_errors++))
    test_health_endpoints || ((total_errors++))
    test_redis_connectivity || ((total_errors++))

    check_resource_usage
    generate_summary $total_errors

    exit $total_errors
}

# Parse arguments and run main function
parse_args "$@"
main
#!/bin/bash

# Kube Ops View Deployment Script
# This script provides a convenient way to deploy Kube Ops View using different methods

set -euo pipefail

# Default values
DEPLOYMENT_METHOD="kustomize"
ENVIRONMENT="production"
NAMESPACE="kube-ops-view"
IMAGE_TAG="latest"
DRY_RUN=false
VERBOSE=false

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
Kube Ops View Deployment Script

Usage: $0 [OPTIONS]

OPTIONS:
    -m, --method METHOD         Deployment method: kustomize, helm, kubectl (default: kustomize)
    -e, --environment ENV       Environment: development, staging, production (default: production)
    -n, --namespace NAMESPACE   Kubernetes namespace (default: kube-ops-view)
    -t, --tag TAG              Image tag (default: latest)
    -d, --dry-run              Perform a dry run without applying changes
    -v, --verbose              Enable verbose output
    -h, --help                 Show this help message

EXAMPLES:
    # Deploy to production using Kustomize
    $0 -m kustomize -e production

    # Deploy to development using Helm
    $0 -m helm -e development -t dev

    # Dry run deployment
    $0 -m kustomize -e staging --dry-run

    # Deploy with custom namespace
    $0 -m helm -e production -n my-kube-ops-view

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -m|--method)
                DEPLOYMENT_METHOD="$2"
                shift 2
                ;;
            -e|--environment)
                ENVIRONMENT="$2"
                shift 2
                ;;
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -t|--tag)
                IMAGE_TAG="$2"
                shift 2
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
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

# Validate prerequisites
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

    # Check method-specific tools
    case $DEPLOYMENT_METHOD in
        kustomize)
            if ! kubectl kustomize --help &> /dev/null; then
                log_error "kubectl kustomize is not available"
                exit 1
            fi
            ;;
        helm)
            if ! command -v helm &> /dev/null; then
                log_error "helm is not installed or not in PATH"
                exit 1
            fi
            ;;
    esac

    log_success "Prerequisites check passed"
}

# Validate inputs
validate_inputs() {
    log_info "Validating inputs..."

    # Validate deployment method
    case $DEPLOYMENT_METHOD in
        kustomize|helm|kubectl)
            ;;
        *)
            log_error "Invalid deployment method: $DEPLOYMENT_METHOD"
            log_error "Valid methods: kustomize, helm, kubectl"
            exit 1
            ;;
    esac

    # Validate environment
    case $ENVIRONMENT in
        development|staging|production)
            ;;
        *)
            log_error "Invalid environment: $ENVIRONMENT"
            log_error "Valid environments: development, staging, production"
            exit 1
            ;;
    esac

    # Validate namespace name
    if [[ ! $NAMESPACE =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
        log_error "Invalid namespace name: $NAMESPACE"
        exit 1
    fi

    log_success "Input validation passed"
}

# Deploy using Kustomize
deploy_kustomize() {
    log_info "Deploying using Kustomize..."

    local kustomize_path
    case $ENVIRONMENT in
        production)
            kustomize_path="deploy/production"
            ;;
        development)
            kustomize_path="deploy/overlays/development"
            ;;
        staging)
            kustomize_path="deploy/overlays/staging"
            ;;
    esac

    # Check if kustomization exists
    if [[ ! -f "$kustomize_path/kustomization.yaml" ]]; then
        log_error "Kustomization file not found: $kustomize_path/kustomization.yaml"
        exit 1
    fi

    # Create temporary kustomization with image tag
    local temp_dir=$(mktemp -d)
    cp -r "$kustomize_path"/* "$temp_dir/"

    # Update image tag if specified
    if [[ "$IMAGE_TAG" != "latest" ]]; then
        log_info "Updating image tag to: $IMAGE_TAG"
        cat >> "$temp_dir/kustomization.yaml" << EOF

# Image tag override
images:
- name: kube-ops-view
  newTag: $IMAGE_TAG
EOF
    fi

    # Update namespace if different
    if [[ "$NAMESPACE" != "kube-ops-view" ]]; then
        log_info "Updating namespace to: $NAMESPACE"
        sed -i.bak "s/namespace: kube-ops-view/namespace: $NAMESPACE/g" "$temp_dir/kustomization.yaml"
    fi

    # Apply or dry-run
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Performing dry run..."
        kubectl kustomize "$temp_dir" | kubectl apply --dry-run=client -f -
    else
        log_info "Applying Kustomize configuration..."
        kubectl apply -k "$temp_dir"
    fi

    # Cleanup
    rm -rf "$temp_dir"
}

# Deploy using Helm
deploy_helm() {
    log_info "Deploying using Helm..."

    local chart_path="deploy/helm/kube-ops-view"
    local values_file="$chart_path/values-$ENVIRONMENT.yaml"

    # Check if chart exists
    if [[ ! -f "$chart_path/Chart.yaml" ]]; then
        log_error "Helm chart not found: $chart_path/Chart.yaml"
        exit 1
    fi

    # Check if values file exists
    if [[ ! -f "$values_file" ]]; then
        log_warning "Environment-specific values file not found: $values_file"
        log_info "Using default values.yaml"
        values_file="$chart_path/values.yaml"
    fi

    # Prepare Helm command
    local helm_cmd="helm"
    local helm_args=()

    if [[ "$DRY_RUN" == "true" ]]; then
        helm_args+=("--dry-run")
        log_info "Performing dry run..."
    fi

    if [[ "$VERBOSE" == "true" ]]; then
        helm_args+=("--debug")
    fi

    # Set image tag
    helm_args+=("--set" "image.tag=$IMAGE_TAG")

    # Check if release exists
    if helm list -n "$NAMESPACE" | grep -q "kube-ops-view"; then
        log_info "Upgrading existing Helm release..."
        $helm_cmd upgrade kube-ops-view "$chart_path" \
            -n "$NAMESPACE" \
            -f "$values_file" \
            "${helm_args[@]}"
    else
        log_info "Installing new Helm release..."
        $helm_cmd install kube-ops-view "$chart_path" \
            -n "$NAMESPACE" \
            --create-namespace \
            -f "$values_file" \
            "${helm_args[@]}"
    fi
}

# Deploy using kubectl
deploy_kubectl() {
    log_info "Deploying using kubectl..."

    local manifest_path="deploy/production"

    # Check if manifests exist
    if [[ ! -d "$manifest_path" ]]; then
        log_error "Manifest directory not found: $manifest_path"
        exit 1
    fi

    # Apply manifests in order
    local manifests=(
        "namespace.yaml"
        "rbac.yaml"
        "configmap.yaml"
        "secret.yaml"
        "redis-pvc.yaml"
        "redis-deployment.yaml"
        "redis-service.yaml"
        "deployment.yaml"
        "service.yaml"
        "poddisruptionbudget.yaml"
        "networkpolicy.yaml"
    )

    for manifest in "${manifests[@]}"; do
        local manifest_file="$manifest_path/$manifest"
        if [[ -f "$manifest_file" ]]; then
            log_info "Applying $manifest..."
            if [[ "$DRY_RUN" == "true" ]]; then
                kubectl apply --dry-run=client -f "$manifest_file"
            else
                kubectl apply -f "$manifest_file"
            fi
        else
            log_warning "Manifest not found: $manifest_file"
        fi
    done
}

# Wait for deployment
wait_for_deployment() {
    if [[ "$DRY_RUN" == "true" ]]; then
        return 0
    fi

    log_info "Waiting for deployment to be ready..."

    # Wait for deployment rollout
    if kubectl rollout status deployment/kube-ops-view -n "$NAMESPACE" --timeout=300s; then
        log_success "Deployment is ready!"
    else
        log_error "Deployment failed to become ready"
        return 1
    fi

    # Wait for Redis if enabled
    if kubectl get deployment kube-ops-view-redis -n "$NAMESPACE" &> /dev/null; then
        log_info "Waiting for Redis deployment..."
        if kubectl rollout status deployment/kube-ops-view-redis -n "$NAMESPACE" --timeout=300s; then
            log_success "Redis deployment is ready!"
        else
            log_warning "Redis deployment failed to become ready"
        fi
    fi
}

# Show deployment status
show_status() {
    if [[ "$DRY_RUN" == "true" ]]; then
        return 0
    fi

    log_info "Deployment Status:"
    echo

    # Show pods
    kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=kube-ops-view
    echo

    # Show services
    kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/name=kube-ops-view
    echo

    # Show ingress if exists
    if kubectl get ingress -n "$NAMESPACE" &> /dev/null; then
        kubectl get ingress -n "$NAMESPACE"
        echo
    fi

    # Show access information
    local service_type=$(kubectl get service kube-ops-view -n "$NAMESPACE" -o jsonpath='{.spec.type}' 2>/dev/null || echo "")

    case $service_type in
        LoadBalancer)
            local external_ip=$(kubectl get service kube-ops-view -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")
            if [[ -n "$external_ip" ]]; then
                log_info "Access URL: http://$external_ip"
            else
                log_info "LoadBalancer IP is pending..."
            fi
            ;;
        NodePort)
            local node_port=$(kubectl get service kube-ops-view -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "")
            if [[ -n "$node_port" ]]; then
                log_info "Access via NodePort: http://<node-ip>:$node_port"
            fi
            ;;
        ClusterIP)
            log_info "Access via port-forward: kubectl port-forward service/kube-ops-view 8080:80 -n $NAMESPACE"
            ;;
    esac
}

# Main function
main() {
    log_info "Starting Kube Ops View deployment..."
    log_info "Method: $DEPLOYMENT_METHOD, Environment: $ENVIRONMENT, Namespace: $NAMESPACE"

    check_prerequisites
    validate_inputs

    # Deploy based on method
    case $DEPLOYMENT_METHOD in
        kustomize)
            deploy_kustomize
            ;;
        helm)
            deploy_helm
            ;;
        kubectl)
            deploy_kubectl
            ;;
    esac

    wait_for_deployment
    show_status

    if [[ "$DRY_RUN" == "false" ]]; then
        log_success "Deployment completed successfully!"
    else
        log_success "Dry run completed successfully!"
    fi
}

# Parse arguments and run main function
parse_args "$@"
main
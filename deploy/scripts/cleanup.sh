#!/bin/bash

# Kube Ops View Cleanup Script
# This script removes Kube Ops View deployments

set -euo pipefail

# Default values
DEPLOYMENT_METHOD="auto"
NAMESPACE="kube-ops-view"
FORCE=false
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
Kube Ops View Cleanup Script

Usage: $0 [OPTIONS]

OPTIONS:
    -m, --method METHOD         Cleanup method: auto, helm, kubectl (default: auto)
    -n, --namespace NAMESPACE   Kubernetes namespace (default: kube-ops-view)
    -f, --force                Force cleanup without confirmation
    -v, --verbose              Enable verbose output
    -h, --help                 Show this help message

EXAMPLES:
    # Auto-detect and cleanup
    $0

    # Cleanup Helm release
    $0 -m helm -n kube-ops-view

    # Force cleanup without confirmation
    $0 --force

    # Cleanup with custom namespace
    $0 -n my-kube-ops-view

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
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -f|--force)
                FORCE=true
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

# Detect deployment method
detect_deployment_method() {
    if [[ "$DEPLOYMENT_METHOD" != "auto" ]]; then
        return 0
    fi

    log_info "Auto-detecting deployment method..."

    # Check for Helm release
    if command -v helm &> /dev/null; then
        if helm list -n "$NAMESPACE" | grep -q "kube-ops-view"; then
            DEPLOYMENT_METHOD="helm"
            log_info "Detected Helm deployment"
            return 0
        fi
    fi

    # Check for kubectl resources
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        DEPLOYMENT_METHOD="kubectl"
        log_info "Detected kubectl deployment"
        return 0
    fi

    log_warning "No deployment detected in namespace: $NAMESPACE"
    DEPLOYMENT_METHOD="kubectl"  # Default to kubectl for cleanup
}

# Show what will be deleted
show_resources() {
    log_info "Resources that will be deleted:"
    echo

    case $DEPLOYMENT_METHOD in
        helm)
            if command -v helm &> /dev/null; then
                log_info "Helm releases:"
                helm list -n "$NAMESPACE" | grep kube-ops-view || log_warning "No Helm releases found"
                echo
            fi
            ;;
        kubectl)
            log_info "Kubernetes resources in namespace $NAMESPACE:"
            kubectl get all -n "$NAMESPACE" 2>/dev/null || log_warning "Namespace not found or empty"
            echo

            log_info "ConfigMaps and Secrets:"
            kubectl get configmap,secret -n "$NAMESPACE" 2>/dev/null || true
            echo

            log_info "PersistentVolumeClaims:"
            kubectl get pvc -n "$NAMESPACE" 2>/dev/null || true
            echo

            log_info "ClusterRole and ClusterRoleBinding:"
            kubectl get clusterrole,clusterrolebinding | grep kube-ops-view || log_warning "No cluster-level resources found"
            echo
            ;;
    esac
}

# Confirm deletion
confirm_deletion() {
    if [[ "$FORCE" == "true" ]]; then
        return 0
    fi

    echo
    log_warning "This will permanently delete all Kube Ops View resources!"
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Cleanup cancelled"
        exit 0
    fi
}

# Cleanup using Helm
cleanup_helm() {
    log_info "Cleaning up Helm deployment..."

    if ! command -v helm &> /dev/null; then
        log_error "helm is not installed or not in PATH"
        exit 1
    fi

    # Uninstall Helm release
    if helm list -n "$NAMESPACE" | grep -q "kube-ops-view"; then
        log_info "Uninstalling Helm release..."
        helm uninstall kube-ops-view -n "$NAMESPACE"
        log_success "Helm release uninstalled"
    else
        log_warning "No Helm release found"
    fi

    # Clean up cluster-level resources
    cleanup_cluster_resources

    # Delete namespace if empty
    cleanup_namespace
}

# Cleanup using kubectl
cleanup_kubectl() {
    log_info "Cleaning up kubectl deployment..."

    # Delete namespace resources
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Deleting namespace resources..."
        kubectl delete namespace "$NAMESPACE" --ignore-not-found=true
        log_success "Namespace deleted"
    else
        log_warning "Namespace not found: $NAMESPACE"
    fi

    # Clean up cluster-level resources
    cleanup_cluster_resources
}

# Cleanup cluster-level resources
cleanup_cluster_resources() {
    log_info "Cleaning up cluster-level resources..."

    # Delete ClusterRole
    if kubectl get clusterrole kube-ops-view &> /dev/null; then
        log_info "Deleting ClusterRole..."
        kubectl delete clusterrole kube-ops-view --ignore-not-found=true
    fi

    # Delete ClusterRoleBinding
    if kubectl get clusterrolebinding kube-ops-view &> /dev/null; then
        log_info "Deleting ClusterRoleBinding..."
        kubectl delete clusterrolebinding kube-ops-view --ignore-not-found=true
    fi

    # Delete any additional cluster resources with kube-ops-view label
    log_info "Cleaning up labeled cluster resources..."
    kubectl delete clusterrole,clusterrolebinding -l app.kubernetes.io/name=kube-ops-view --ignore-not-found=true
}

# Cleanup namespace if empty
cleanup_namespace() {
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        # Check if namespace has any resources left
        local resource_count=$(kubectl get all -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)

        if [[ $resource_count -eq 0 ]]; then
            log_info "Deleting empty namespace..."
            kubectl delete namespace "$NAMESPACE" --ignore-not-found=true
            log_success "Empty namespace deleted"
        else
            log_warning "Namespace still contains resources, not deleting"
        fi
    fi
}

# Verify cleanup
verify_cleanup() {
    log_info "Verifying cleanup..."

    local cleanup_success=true

    # Check namespace
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        local resource_count=$(kubectl get all -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
        if [[ $resource_count -gt 0 ]]; then
            log_warning "Namespace still contains resources"
            cleanup_success=false
        fi
    fi

    # Check cluster resources
    if kubectl get clusterrole kube-ops-view &> /dev/null; then
        log_warning "ClusterRole still exists"
        cleanup_success=false
    fi

    if kubectl get clusterrolebinding kube-ops-view &> /dev/null; then
        log_warning "ClusterRoleBinding still exists"
        cleanup_success=false
    fi

    # Check Helm release
    if command -v helm &> /dev/null; then
        if helm list -n "$NAMESPACE" | grep -q "kube-ops-view"; then
            log_warning "Helm release still exists"
            cleanup_success=false
        fi
    fi

    if [[ "$cleanup_success" == "true" ]]; then
        log_success "Cleanup verification passed"
    else
        log_warning "Some resources may still exist"
        return 1
    fi
}

# Main function
main() {
    log_info "Starting Kube Ops View cleanup..."

    check_prerequisites
    detect_deployment_method

    log_info "Using cleanup method: $DEPLOYMENT_METHOD"
    log_info "Target namespace: $NAMESPACE"

    show_resources
    confirm_deletion

    # Perform cleanup based on method
    case $DEPLOYMENT_METHOD in
        helm)
            cleanup_helm
            ;;
        kubectl)
            cleanup_kubectl
            ;;
        *)
            log_error "Invalid cleanup method: $DEPLOYMENT_METHOD"
            exit 1
            ;;
    esac

    verify_cleanup

    log_success "Cleanup completed!"
    log_info "All Kube Ops View resources have been removed"
}

# Parse arguments and run main function
parse_args "$@"
main
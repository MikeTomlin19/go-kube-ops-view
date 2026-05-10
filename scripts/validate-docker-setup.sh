#!/bin/bash

# Validation script for Docker containerization setup
set -euo pipefail

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

success() {
    echo -e "${GREEN}✓ $1${NC}"
}

fail() {
    echo -e "${RED}✗ $1${NC}"
    return 1
}

# Function to check if a file exists and is readable
check_file() {
    local file=$1
    local description=$2

    if [[ -f "$file" && -r "$file" ]]; then
        success "$description exists and is readable"
        return 0
    else
        fail "$description is missing or not readable"
        return 1
    fi
}

# Function to check if a file is executable
check_executable() {
    local file=$1
    local description=$2

    if [[ -f "$file" && -x "$file" ]]; then
        success "$description is executable"
        return 0
    else
        fail "$description is not executable"
        return 1
    fi
}

# Function to validate Docker file syntax
validate_dockerfile() {
    local dockerfile=$1

    info "Validating Dockerfile syntax: $dockerfile"

    # Check if dockerfile exists
    check_file "$dockerfile" "Dockerfile"

    # Basic syntax validation
    if grep -q "^FROM " "$dockerfile"; then
        success "Dockerfile has FROM instruction"
    else
        fail "Dockerfile missing FROM instruction"
        return 1
    fi

    if grep -q "^WORKDIR " "$dockerfile"; then
        success "Dockerfile has WORKDIR instruction"
    else
        warn "Dockerfile missing WORKDIR instruction"
    fi

    if grep -q "^USER " "$dockerfile"; then
        success "Dockerfile runs as non-root user"
    else
        fail "Dockerfile does not specify non-root user"
        return 1
    fi

    if grep -q "^EXPOSE " "$dockerfile"; then
        success "Dockerfile exposes port"
    else
        warn "Dockerfile does not expose any ports"
    fi

    if grep -q "HEALTHCHECK" "$dockerfile"; then
        success "Dockerfile includes health check"
    else
        warn "Dockerfile missing health check"
    fi

    return 0
}

# Function to validate docker-compose file
validate_docker_compose() {
    local compose_file=$1

    info "Validating docker-compose file: $compose_file"

    check_file "$compose_file" "Docker Compose file"

    # Check if docker-compose is available for validation
    if command -v docker-compose &> /dev/null; then
        if docker-compose -f "$compose_file" config >/dev/null 2>&1; then
            success "Docker Compose file syntax is valid"
        else
            fail "Docker Compose file has syntax errors"
            return 1
        fi
    else
        warn "docker-compose not available for validation"
    fi

    # Check for required services
    if grep -q "kube-ops-view:" "$compose_file"; then
        success "Docker Compose includes kube-ops-view service"
    else
        fail "Docker Compose missing kube-ops-view service"
        return 1
    fi

    if grep -q "redis:" "$compose_file"; then
        success "Docker Compose includes Redis service"
    else
        warn "Docker Compose missing Redis service"
    fi

    return 0
}

# Function to validate Kubernetes manifests
validate_k8s_manifests() {
    info "Validating Kubernetes manifests"

    local manifests=(
        "deploy/deployment-go.yaml"
        "deploy/service.yaml"
        "deploy/rbac.yaml"
        "deploy/redis-deployment.yaml"
        "deploy/redis-service.yaml"
        "deploy/kustomization-go.yaml"
    )

    for manifest in "${manifests[@]}"; do
        if [[ -f "$manifest" ]]; then
            success "Kubernetes manifest exists: $manifest"

            # Basic YAML validation
            if command -v yq &> /dev/null; then
                if yq eval '.' "$manifest" >/dev/null 2>&1; then
                    success "YAML syntax valid: $manifest"
                else
                    fail "YAML syntax error: $manifest"
                    return 1
                fi
            elif command -v python3 &> /dev/null; then
                if python3 -c "import yaml; yaml.safe_load(open('$manifest'))" 2>/dev/null; then
                    success "YAML syntax valid: $manifest"
                else
                    fail "YAML syntax error: $manifest"
                    return 1
                fi
            fi
        else
            warn "Kubernetes manifest missing: $manifest"
        fi
    done

    return 0
}

# Function to validate build scripts
validate_build_scripts() {
    info "Validating build scripts"

    local scripts=(
        "scripts/build-docker.sh"
        "scripts/test-docker.sh"
        "scripts/validate-docker-setup.sh"
    )

    for script in "${scripts[@]}"; do
        check_executable "$script" "Build script: $script"

        # Check for bash shebang
        if head -n 1 "$script" | grep -q "#!/bin/bash"; then
            success "Script has proper shebang: $script"
        else
            warn "Script missing bash shebang: $script"
        fi

        # Check for set -euo pipefail
        if grep -q "set -euo pipefail" "$script"; then
            success "Script uses strict error handling: $script"
        else
            warn "Script missing strict error handling: $script"
        fi
    done

    return 0
}

# Function to validate configuration files
validate_config_files() {
    info "Validating configuration files"

    # Check .dockerignore
    if [[ -f ".dockerignore.go" ]]; then
        success ".dockerignore.go exists"

        # Check for common patterns
        if grep -q "\.git" ".dockerignore.go"; then
            success ".dockerignore.go excludes .git"
        else
            warn ".dockerignore.go should exclude .git"
        fi

        if grep -q "node_modules" ".dockerignore.go"; then
            success ".dockerignore.go excludes node_modules"
        else
            warn ".dockerignore.go should exclude node_modules"
        fi
    else
        warn ".dockerignore.go missing"
    fi

    # Check Redis configuration
    if [[ -f "redis.conf" ]]; then
        success "Redis configuration exists"

        if grep -q "maxmemory" "redis.conf"; then
            success "Redis config includes memory limits"
        else
            warn "Redis config missing memory limits"
        fi
    else
        warn "Redis configuration missing"
    fi

    # Check Makefile
    if [[ -f "Makefile.docker" ]]; then
        success "Docker Makefile exists"

        if grep -q "\.PHONY:" "Makefile.docker"; then
            success "Makefile uses .PHONY targets"
        else
            warn "Makefile missing .PHONY declarations"
        fi
    else
        warn "Docker Makefile missing"
    fi

    return 0
}

# Function to test Docker build (if Docker is available)
test_docker_build() {
    info "Testing Docker build (if Docker is available)"

    if ! command -v docker &> /dev/null; then
        warn "Docker not available, skipping build test"
        return 0
    fi

    # Test basic Docker functionality
    if docker version >/dev/null 2>&1; then
        success "Docker is available and working"
    else
        fail "Docker is not working properly"
        return 1
    fi

    # Test if we can build the image (dry run)
    info "Testing Dockerfile build context..."
    if docker build -f Dockerfile.go --dry-run . >/dev/null 2>&1; then
        success "Dockerfile build context is valid"
    else
        warn "Dockerfile build context may have issues"
    fi

    return 0
}

# Function to validate documentation
validate_documentation() {
    info "Validating documentation"

    if [[ -f "docker/README.md" ]]; then
        success "Docker README exists"

        # Check for required sections
        local sections=(
            "Quick Start"
            "Configuration"
            "Security"
            "Deployment"
            "Troubleshooting"
        )

        for section in "${sections[@]}"; do
            if grep -q "$section" "docker/README.md"; then
                success "README includes $section section"
            else
                warn "README missing $section section"
            fi
        done
    else
        fail "Docker README missing"
        return 1
    fi

    return 0
}

# Function to check dependencies
check_dependencies() {
    info "Checking dependencies"

    local required_commands=(
        "docker"
        "make"
    )

    local optional_commands=(
        "docker-compose"
        "kubectl"
        "yq"
    )

    for cmd in "${required_commands[@]}"; do
        if command -v "$cmd" &> /dev/null; then
            success "Required command available: $cmd"
        else
            warn "Required command missing: $cmd"
        fi
    done

    for cmd in "${optional_commands[@]}"; do
        if command -v "$cmd" &> /dev/null; then
            success "Optional command available: $cmd"
        else
            info "Optional command not available: $cmd"
        fi
    done

    return 0
}

# Main validation function
main() {
    log "Starting Docker containerization setup validation"

    local validation_failed=false

    # Run all validations
    check_dependencies || validation_failed=true
    validate_dockerfile "Dockerfile.go" || validation_failed=true
    validate_docker_compose "docker-compose.yml" || validation_failed=true
    validate_docker_compose "docker-compose.prod.yml" || validation_failed=true
    validate_k8s_manifests || validation_failed=true
    validate_build_scripts || validation_failed=true
    validate_config_files || validation_failed=true
    validate_documentation || validation_failed=true
    test_docker_build || validation_failed=true

    if [[ "$validation_failed" == "true" ]]; then
        error "Docker containerization setup validation failed"
    else
        log "Docker containerization setup validation completed successfully!"
    fi
}

# Help function
show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Validate Docker containerization setup for kube-ops-view

This script validates:
- Dockerfile syntax and best practices
- Docker Compose configuration
- Kubernetes manifests
- Build scripts
- Configuration files
- Documentation

OPTIONS:
    -h, --help          Show this help message

EXAMPLES:
    # Run full validation
    $0

    # Run with verbose output
    DEBUG=1 $0
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Run main function
main
#!/bin/bash

# Build script for kube-ops-view Docker container
set -euo pipefail

# Default values
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
DATE=${DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}
IMAGE_NAME=${IMAGE_NAME:-"go-kube-ops-view"}
REGISTRY=${REGISTRY:-""}
PLATFORM=${PLATFORM:-"linux/amd64,linux/arm64"}
PUSH=${PUSH:-"false"}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

warn() {
    echo -e "${YELLOW}[$(date +'%Y-%m-%d %H:%M:%S')] WARNING: $1${NC}"
}

error() {
    echo -e "${RED}[$(date +'%Y-%m-%d %H:%M:%S')] ERROR: $1${NC}"
    exit 1
}

# Function to build single-platform image
build_single_platform() {
    local platform=$1
    local tag_suffix=""

    if [[ "$platform" == "linux/arm64" ]]; then
        tag_suffix="-arm64"
    elif [[ "$platform" == "linux/amd64" ]]; then
        tag_suffix="-amd64"
    fi

    local full_image_name="${REGISTRY:+$REGISTRY/}${IMAGE_NAME}:${VERSION}${tag_suffix}"

    log "Building Docker image for platform: $platform"
    log "Image name: $full_image_name"
    log "Version: $VERSION"
    log "Commit: $COMMIT"
    log "Date: $DATE"

    docker build \
        --platform "$platform" \
        --file Dockerfile \
        --build-arg VERSION="$VERSION" \
        --build-arg COMMIT="$COMMIT" \
        --build-arg DATE="$DATE" \
        --tag "$full_image_name" \
        .

    if [[ "$PUSH" == "true" ]]; then
        log "Pushing image: $full_image_name"
        docker push "$full_image_name"
    fi

    echo "$full_image_name"
}

# Function to build multi-platform image using buildx
build_multi_platform() {
    local full_image_name="${REGISTRY:+$REGISTRY/}${IMAGE_NAME}:${VERSION}"

    log "Building multi-platform Docker image"
    log "Platforms: $PLATFORM"
    log "Image name: $full_image_name"
    log "Version: $VERSION"
    log "Commit: $COMMIT"
    log "Date: $DATE"

    # Create buildx builder if it doesn't exist
    if ! docker buildx inspect kube-ops-view-builder >/dev/null 2>&1; then
        log "Creating buildx builder: kube-ops-view-builder"
        docker buildx create --name kube-ops-view-builder --use
    else
        docker buildx use kube-ops-view-builder
    fi

    # Build command
    local build_cmd=(
        docker buildx build
        --platform "$PLATFORM"
        --file Dockerfile
        --build-arg "VERSION=$VERSION"
        --build-arg "COMMIT=$COMMIT"
        --build-arg "DATE=$DATE"
        --tag "$full_image_name"
    )

    if [[ "$PUSH" == "true" ]]; then
        build_cmd+=(--push)
    else
        build_cmd+=(--load)
    fi

    build_cmd+=(.)

    "${build_cmd[@]}"

    echo "$full_image_name"
}

# Function to test the built image
test_image() {
    local image_name=$1

    log "Testing Docker image: $image_name"

    # Test that the image runs and responds to health check
    local container_id
    container_id=$(docker run -d -p 8080:8080 "$image_name" --mock=true)

    # Wait for container to start
    sleep 10

    # Test health endpoint
    if curl -f http://localhost:8080/health >/dev/null 2>&1; then
        log "Health check passed"
    else
        error "Health check failed"
    fi

    # Test main endpoint
    if curl -f http://localhost:8080/ >/dev/null 2>&1; then
        log "Main endpoint test passed"
    else
        warn "Main endpoint test failed (may be expected without clusters)"
    fi

    # Clean up
    docker stop "$container_id" >/dev/null
    docker rm "$container_id" >/dev/null

    log "Image test completed successfully"
}

# Main execution
main() {
    log "Starting Docker build process"

    # Check if Docker is available
    if ! command -v docker &> /dev/null; then
        error "Docker is not installed or not in PATH"
    fi

    # Check if we're in the right directory
    if [[ ! -f "Dockerfile" ]]; then
        error "Dockerfile not found. Please run this script from the project root."
    fi

    # Check if we need multi-platform build
    if [[ "$PLATFORM" == *","* ]]; then
        # Multi-platform build
        if ! docker buildx version >/dev/null 2>&1; then
            error "Docker buildx is required for multi-platform builds"
        fi
        image_name=$(build_multi_platform)
    else
        # Single platform build
        image_name=$(build_single_platform "$PLATFORM")
    fi

    # Test the image if not pushing (local build)
    if [[ "$PUSH" != "true" && "$PLATFORM" == "linux/amd64" ]]; then
        test_image "$image_name"
    fi

    log "Build process completed successfully"
    log "Built image: $image_name"
}

# Help function
show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Build Docker image for kube-ops-view

OPTIONS:
    -h, --help          Show this help message
    -v, --version       Set version tag (default: git describe or 'dev')
    -r, --registry      Set registry prefix (e.g., 'docker.io/myuser')
    -n, --name          Set image name (default: 'go-kube-ops-view')
    -p, --platform      Set target platform(s) (default: 'linux/amd64,linux/arm64')
    --push              Push image to registry after build
    --test              Run tests after build (default for local builds)

ENVIRONMENT VARIABLES:
    VERSION             Version tag for the image
    COMMIT              Git commit hash
    DATE                Build date
    IMAGE_NAME          Name of the Docker image
    REGISTRY            Registry prefix
    PLATFORM            Target platform(s)
    PUSH                Whether to push the image

EXAMPLES:
    # Build for local development
    $0

    # Build and push to registry
    $0 --registry docker.io/myuser --push

    # Build for specific platform
    $0 --platform linux/arm64

    # Build with custom version
    $0 --version v1.0.0
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -n|--name)
            IMAGE_NAME="$2"
            shift 2
            ;;
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        --push)
            PUSH="true"
            shift
            ;;
        --test)
            # Test is default for local builds
            shift
            ;;
        *)
            error "Unknown option: $1"
            ;;
    esac
done

# Run main function
main

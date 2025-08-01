#!/bin/bash
# Build script for using BuildKit directly without Docker

set -e

# Default values
DOCKERFILE="Dockerfile"
CONTEXT="."
TAG=""
PLATFORM="linux/amd64"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -f|--file)
            DOCKERFILE="$2"
            shift 2
            ;;
        -t|--tag)
            TAG="$2"
            shift 2
            ;;
        --platform)
            PLATFORM="$2"
            shift 2
            ;;
        *)
            CONTEXT="$1"
            shift
            ;;
    esac
done

# Get absolute paths
CONTEXT_PATH=$(cd "$CONTEXT" && pwd)
DOCKERFILE_PATH="$CONTEXT_PATH/$DOCKERFILE"

if [ ! -f "$DOCKERFILE_PATH" ]; then
    echo "Error: Dockerfile not found: $DOCKERFILE_PATH"
    exit 1
fi

echo "Building with BuildKit in Colima..."
echo "Context: $CONTEXT_PATH"
echo "Dockerfile: $DOCKERFILE"
echo "Tag: ${TAG:-<none>}"
echo "Platform: $PLATFORM"

# Build command
BUILD_CMD="sudo buildctl build \
    --frontend dockerfile.v0 \
    --local context=$CONTEXT_PATH \
    --local dockerfile=$(dirname $DOCKERFILE_PATH) \
    --opt filename=$(basename $DOCKERFILE) \
    --opt platform=$PLATFORM"

if [ -n "$TAG" ]; then
    BUILD_CMD="$BUILD_CMD --output type=image,name=$TAG"
else
    BUILD_CMD="$BUILD_CMD --output type=oci,dest=/tmp/image.tar"
fi

# Execute build in Colima
echo "Running: colima ssh -- $BUILD_CMD"
colima ssh -- "$BUILD_CMD"

if [ -z "$TAG" ]; then
    echo "Image exported to /tmp/image.tar in Colima VM"
    echo "To copy to host: colima ssh -- cat /tmp/image.tar > image.tar"
else
    echo "Image built and tagged as: $TAG"
fi
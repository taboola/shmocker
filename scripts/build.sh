#!/bin/bash

# Build script for shmocker - handles platform-specific build constraints

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Building shmocker..."

# Set build tags for the current platform
BUILDTAGS=""
case "$(uname -s)" in
    Linux*)
        BUILDTAGS="linux"
        echo "Building for Linux platform"
        ;;
    Darwin*)
        BUILDTAGS="darwin"
        echo "Building for macOS platform (some BuildKit features may be limited)"
        ;;
    *)
        echo "Unsupported platform: $(uname -s)"
        exit 1
        ;;
esac

# Build the main binary
echo "Building main binary..."
cd "$PROJECT_ROOT"
go build -tags "$BUILDTAGS" -o bin/shmocker ./cmd/shmocker

echo "Build successful! Binary available at: bin/shmocker"

# Run unit tests (exclude integration tests)
echo "Running unit tests..."
go test -tags "$BUILDTAGS" -short ./pkg/...

echo "Unit tests passed!"

echo ""
echo "To run integration tests (Linux only):"
echo "  go test -tags integration ./pkg/builder/..."
echo ""
echo "To run all tests:"
echo "  go test ./pkg/..."
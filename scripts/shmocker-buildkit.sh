#!/bin/bash
# Simple wrapper to use shmocker with BuildKit in Colima

# First, ensure Colima is running with containerd
if ! colima status &>/dev/null; then
    echo "Error: Colima is not running. Please start it with: colima start --runtime containerd"
    exit 1
fi

# Check if BuildKit is running in Colima
if ! colima ssh -- ps aux | grep -q '[b]uildkitd'; then
    echo "Error: BuildKit is not running in Colima"
    echo "You can start it with: colima ssh -- sudo buildkitd &"
    exit 1
fi

# For now, we'll demonstrate building with buildctl directly
echo "=== Building with BuildKit (no Docker required!) ==="
echo
echo "Example commands:"
echo
echo "1. Build a simple image:"
echo "   colima cp examples/Dockerfile.simple colima:/tmp/"
echo "   colima cp -r examples colima:/tmp/"
echo "   colima ssh -- 'cd /tmp && sudo buildctl build --frontend dockerfile.v0 --local context=examples --local dockerfile=examples --opt filename=Dockerfile.simple'"
echo
echo "2. Export to tar:"
echo "   Add: --output type=oci,dest=/tmp/image.tar"
echo
echo "3. Push to registry:"
echo "   Add: --output type=image,name=registry.com/image:tag,push=true"
echo

# If arguments provided, try to build
if [ $# -gt 0 ]; then
    echo "Note: Direct building through this script is not yet implemented."
    echo "For now, use the buildctl commands shown above."
fi
#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default values
NAMESPACE="${K8S_NAMESPACE:-eir}"
IMAGE_NAME="shmocker-build-$$"  # Use PID to make unique
TIMEOUT=300

# Function to print colored output
log() {
    local level=$1
    shift
    case $level in
        ERROR) echo -e "${RED}[ERROR]${NC} $*" >&2 ;;
        SUCCESS) echo -e "${GREEN}[SUCCESS]${NC} $*" ;;
        INFO) echo -e "${BLUE}[INFO]${NC} $*" ;;
        WARN) echo -e "${YELLOW}[WARN]${NC} $*" ;;
    esac
}

# Function to cleanup resources
cleanup() {
    log INFO "Cleaning up Kubernetes resources..."
    kubectl delete job ${IMAGE_NAME} -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    kubectl delete configmap ${IMAGE_NAME}-dockerfile -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    kubectl delete configmap ${IMAGE_NAME}-context -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    pkill -f "kubectl port-forward.*${IMAGE_NAME}" 2>/dev/null || true
}

trap cleanup EXIT

# Create build job
create_build_job() {
    local dockerfile_path=$1
    local context_dir=$2
    local output_file=$3
    
    log INFO "Creating Kubernetes resources..."
    
    # Create ConfigMap from Dockerfile
    kubectl create configmap ${IMAGE_NAME}-dockerfile \
        --from-file=Dockerfile=${dockerfile_path} \
        -n ${NAMESPACE} \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Check for context files
    local context_files=$(find ${context_dir} -type f ! -name "*.dockerfile" ! -name "Dockerfile" 2>/dev/null | head -5)
    if [ -n "$context_files" ]; then
        log INFO "Found context files: $(echo $context_files | tr '\n' ' ')"
        kubectl create configmap ${IMAGE_NAME}-context \
            --from-file=${context_dir} \
            -n ${NAMESPACE} \
            --dry-run=client -o yaml | kubectl apply -f -
    fi
    
    # Create the job
    cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${IMAGE_NAME}
  namespace: ${NAMESPACE}
spec:
  ttlSecondsAfterFinished: 600
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      initContainers:
      - name: prepare
        image: busybox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "=== Preparing build context ==="
          cp /dockerfile/Dockerfile /workspace/
          if [ -d /context ]; then
            echo "Copying context files..."
            cd /context
            for f in *; do
              if [ -f "\$f" ] && [ "\$f" != "Dockerfile" ]; then
                cp "\$f" /workspace/
              fi
            done
          fi
          echo "Workspace contents:"
          ls -la /workspace/
        resources:
          limits:
            cpu: "0.5"
            memory: "256Mi"
          requests:
            cpu: "0.1"
            memory: "64Mi"
        volumeMounts:
        - name: dockerfile
          mountPath: /dockerfile
        - name: workspace
          mountPath: /workspace
        - name: context
          mountPath: /context
      containers:
      - name: build
        image: moby/buildkit:v0.17.0-rootless
        env:
        - name: BUILDKITD_FLAGS
          value: --oci-worker-no-process-sandbox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "=== Starting BuildKit build ==="
          buildctl-daemonless.sh build \
            --frontend dockerfile.v0 \
            --local context=/workspace \
            --local dockerfile=/workspace \
            --output type=oci,dest=/output/${output_file} \
            --progress plain
          
          if [ -f /output/${output_file} ]; then
            echo "=== Build successful ==="
            ls -lh /output/
            # Keep container running for download
            echo "Keeping container alive for download..."
            sleep 300
          else
            echo "=== Build FAILED ==="
            exit 1
          fi
        resources:
          limits:
            cpu: "4"
            memory: "4Gi"
          requests:
            cpu: "1"
            memory: "1Gi"
        securityContext:
          seccompProfile:
            type: Unconfined
          runAsUser: 1000
          runAsGroup: 1000
        volumeMounts:
        - name: workspace
          mountPath: /workspace
        - name: output
          mountPath: /output
        - name: cache
          mountPath: /home/user/.local/share/buildkit
      volumes:
      - name: dockerfile
        configMap:
          name: ${IMAGE_NAME}-dockerfile
      - name: context
        configMap:
          name: ${IMAGE_NAME}-context
          optional: true
      - name: workspace
        emptyDir: {}
      - name: output
        emptyDir: {}
      - name: cache
        emptyDir: {}
EOF
}

# Wait for build and download
wait_and_download() {
    local output_file=$1
    local local_path=$2
    
    log INFO "Waiting for pod to start..."
    
    # Wait for pod
    local pod_name=""
    for i in {1..30}; do
        pod_name=$(kubectl get pods -n ${NAMESPACE} -l job-name=${IMAGE_NAME} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$pod_name" ]; then
            break
        fi
        sleep 1
    done
    
    if [ -z "$pod_name" ]; then
        log ERROR "Pod not created within 30 seconds"
        return 1
    fi
    
    log INFO "Pod started: $pod_name"
    
    # Wait for init to complete
    log INFO "Waiting for init container..."
    kubectl wait --for=condition=Initialized pod/$pod_name -n ${NAMESPACE} --timeout=60s
    
    # Stream logs
    log INFO "Following build logs..."
    
    # Wait for container to start
    local retries=0
    while [ $retries -lt 10 ]; do
        if kubectl logs $pod_name -c build -n ${NAMESPACE} >/dev/null 2>&1; then
            break
        fi
        sleep 1
        retries=$((retries + 1))
    done
    
    # Now stream the logs
    kubectl logs -f $pod_name -c build -n ${NAMESPACE} 2>&1 | while read line; do
        echo "  $line"
    done || true
    
    # Check if build was successful by looking for the success message
    sleep 2
    local build_status=$(kubectl logs $pod_name -c build -n ${NAMESPACE} --tail=20 | grep -c "Build successful" || true)
    if [ "$build_status" -eq 0 ]; then
        log ERROR "Build failed!"
        return 1
    fi
    
    log SUCCESS "Build completed successfully"
    
    # Download the image while container is still running
    log INFO "Downloading image..."
    kubectl cp ${NAMESPACE}/${pod_name}:/output/${output_file} ${local_path} -c build
    
    if [ -f "$local_path" ] && [ -s "$local_path" ]; then
        log SUCCESS "Image downloaded: $local_path"
        return 0
    else
        log ERROR "Failed to download image or image is empty"
        return 1
    fi
}

# Validate OCI image
validate_image() {
    local image_path=$1
    
    log INFO "Validating OCI image..."
    
    # Basic tar check
    if ! tar -tf "$image_path" >/dev/null 2>&1; then
        log ERROR "Not a valid tar archive"
        return 1
    fi
    
    # Check OCI structure
    local has_layout=$(tar -tf "$image_path" | grep -c "^oci-layout$" || true)
    local has_index=$(tar -tf "$image_path" | grep -c "^index.json$" || true)
    local has_blobs=$(tar -tf "$image_path" | grep -c "^blobs/" || true)
    
    if [ "$has_layout" -eq 0 ] || [ "$has_index" -eq 0 ] || [ "$has_blobs" -eq 0 ]; then
        log ERROR "Invalid OCI image structure"
        return 1
    fi
    
    # Extract details
    local size=$(ls -lh "$image_path" | awk '{print $5}')
    local blobs=$(tar -tf "$image_path" | grep -c "^blobs/sha256/" || true)
    
    log SUCCESS "Valid OCI image!"
    log INFO "  Size: $size"
    log INFO "  Blobs: $blobs"
    
    # Try to show config
    if command -v jq >/dev/null 2>&1; then
        local index_json=$(tar -xOf "$image_path" index.json 2>/dev/null)
        if [ -n "$index_json" ]; then
            echo "$index_json" | jq -r '.manifests[0] | "  Platform: \(.platform.os)/\(.platform.architecture)"' 2>/dev/null || true
        fi
    fi
    
    return 0
}

# Main
main() {
    if [ $# -lt 1 ]; then
        echo "Usage: $0 <dockerfile> [context-dir] [output-file]"
        echo ""
        echo "Build a container image using BuildKit on Kubernetes"
        echo ""
        echo "Arguments:"
        echo "  dockerfile    Path to Dockerfile"
        echo "  context-dir   Build context directory (default: dockerfile directory)"
        echo "  output-file   Output filename (default: image.tar)"
        echo ""
        echo "Environment:"
        echo "  K8S_NAMESPACE Kubernetes namespace (default: eir)"
        exit 1
    fi
    
    local dockerfile=$1
    local context_dir=${2:-$(dirname "$dockerfile")}
    local output_file=${3:-image.tar}
    local output_path="$(pwd)/$output_file"
    
    # Validate inputs
    if [ ! -f "$dockerfile" ]; then
        log ERROR "Dockerfile not found: $dockerfile"
        exit 1
    fi
    
    if [ ! -d "$context_dir" ]; then
        log ERROR "Context directory not found: $context_dir"
        exit 1
    fi
    
    log INFO "Shmocker Kubernetes Build"
    log INFO "========================="
    log INFO "Dockerfile: $dockerfile"
    log INFO "Context: $context_dir"
    log INFO "Output: $output_path"
    log INFO "Namespace: $NAMESPACE"
    echo ""
    
    # Build
    create_build_job "$dockerfile" "$context_dir" "$(basename $output_file)"
    
    if wait_and_download "$(basename $output_file)" "$output_path"; then
        validate_image "$output_path"
        echo ""
        log SUCCESS "Build complete! Image saved to: $output_path"
        log INFO "Load with: docker load < $output_path"
        log INFO "Or push with: skopeo copy oci-archive:$output_path docker://registry/image:tag"
    else
        exit 1
    fi
}

main "$@"
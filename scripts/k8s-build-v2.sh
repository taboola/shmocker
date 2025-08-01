#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
NAMESPACE="${K8S_NAMESPACE:-eir}"
IMAGE_NAME="shmocker-build-$$"
BUILD_TIMEOUT=${BUILD_TIMEOUT:-300}
DOWNLOAD_TIMEOUT=${DOWNLOAD_TIMEOUT:-60}

# Logging
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

# Cleanup
cleanup() {
    log INFO "Cleaning up..."
    kubectl delete job ${IMAGE_NAME} -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    kubectl delete configmap ${IMAGE_NAME}-dockerfile -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    kubectl delete configmap ${IMAGE_NAME}-context -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
}

trap cleanup EXIT

# Create build job with completion signaling
create_job() {
    local dockerfile_path=$1
    local context_dir=$2
    local output_file=$3
    
    log INFO "Creating build job..."
    
    # Create Dockerfile ConfigMap
    kubectl create configmap ${IMAGE_NAME}-dockerfile \
        --from-file=Dockerfile=${dockerfile_path} \
        -n ${NAMESPACE} \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Create context ConfigMap if needed
    local has_context=false
    if [ -n "$(find ${context_dir} -type f ! -name "*.dockerfile" ! -name "Dockerfile" 2>/dev/null | head -1)" ]; then
        has_context=true
        kubectl create configmap ${IMAGE_NAME}-context \
            --from-file=${context_dir} \
            -n ${NAMESPACE} \
            --dry-run=client -o yaml | kubectl apply -f -
    fi
    
    # Job manifest with sidecar pattern
    cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${IMAGE_NAME}
  namespace: ${NAMESPACE}
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 300
  template:
    spec:
      restartPolicy: Never
      initContainers:
      - name: prepare
        image: busybox
        command: ["/bin/sh", "-c"]
        args:
        - |
          cp /dockerfile/Dockerfile /workspace/
          if [ -d /context ]; then
            cd /context
            for f in *; do
              [ -f "\$f" ] && [ "\$f" != "Dockerfile" ] && cp "\$f" /workspace/
            done
          fi
          ls -la /workspace/
        resources:
          requests:
            cpu: "0.1"
            memory: "64Mi"
          limits:
            cpu: "0.5"
            memory: "256Mi"
        volumeMounts:
        - name: dockerfile
          mountPath: /dockerfile
        - name: context
          mountPath: /context
        - name: workspace
          mountPath: /workspace
      containers:
      # Build container
      - name: build
        image: moby/buildkit:v0.17.0-rootless
        env:
        - name: BUILDKITD_FLAGS
          value: --oci-worker-no-process-sandbox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "[BUILD] Starting BuildKit..."
          
          # Run the build
          if buildctl-daemonless.sh build \
            --frontend dockerfile.v0 \
            --local context=/workspace \
            --local dockerfile=/workspace \
            --output type=oci,dest=/output/${output_file} \
            --progress plain; then
            
            echo "[BUILD] Success! Image size: \$(ls -lh /output/${output_file} | awk '{print \$5}')"
            touch /output/build.success
            
            # Signal completion to download container
            echo "ready" > /output/download.signal
          else
            echo "[BUILD] Failed!"
            touch /output/build.failed
            echo "failed" > /output/download.signal
          fi
          
          # Wait for download to complete or timeout
          echo "[BUILD] Waiting for download completion..."
          timeout=60
          while [ \$timeout -gt 0 ]; do
            if [ -f /output/download.done ]; then
              echo "[BUILD] Download completed, exiting"
              exit 0
            fi
            sleep 1
            timeout=\$((timeout - 1))
          done
          echo "[BUILD] Timeout waiting for download"
          exit 1
        resources:
          requests:
            cpu: "1"
            memory: "1Gi"
          limits:
            cpu: "4"
            memory: "4Gi"
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
      
      # Download monitor container
      - name: monitor
        image: busybox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "[MONITOR] Waiting for build to complete..."
          
          # Wait for build signal
          while [ ! -f /output/download.signal ]; do
            sleep 1
          done
          
          # Check build status
          if grep -q "failed" /output/download.signal; then
            echo "[MONITOR] Build failed, exiting"
            exit 1
          fi
          
          echo "[MONITOR] Build succeeded, ready for download"
          
          # Keep alive until download is done
          timeout=${DOWNLOAD_TIMEOUT}
          while [ \$timeout -gt 0 ]; do
            if [ -f /output/download.done ]; then
              echo "[MONITOR] Download completed"
              exit 0
            fi
            sleep 1
            timeout=\$((timeout - 1))
          done
          
          echo "[MONITOR] Download timeout"
          exit 1
        resources:
          requests:
            cpu: "0.1"
            memory: "64Mi"
          limits:
            cpu: "0.5"
            memory: "256Mi"
        volumeMounts:
        - name: output
          mountPath: /output
      
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

# Monitor build and download when ready
monitor_and_download() {
    local output_file=$1
    local local_path=$2
    
    # Wait for pod to start
    log INFO "Waiting for pod to start..."
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
    kubectl wait --for=condition=Initialized pod/$pod_name -n ${NAMESPACE} --timeout=60s || {
        log ERROR "Init containers failed"
        return 1
    }
    
    # Monitor build progress
    log INFO "Monitoring build progress..."
    local last_line=""
    local build_done=false
    local start_time=$(date +%s)
    
    while [ "$build_done" = false ]; do
        # Check timeout
        local current_time=$(date +%s)
        if [ $((current_time - start_time)) -gt $BUILD_TIMEOUT ]; then
            log ERROR "Build timeout after ${BUILD_TIMEOUT}s"
            return 1
        fi
        
        # Get latest logs
        local logs=$(kubectl logs $pod_name -c build -n ${NAMESPACE} --tail=20 2>/dev/null || echo "")
        
        # Check for completion markers
        if echo "$logs" | grep -q "\[BUILD\] Success!"; then
            build_done=true
            log SUCCESS "Build completed successfully!"
        elif echo "$logs" | grep -q "\[BUILD\] Failed!"; then
            log ERROR "Build failed!"
            echo "$logs" | tail -10
            return 1
        else
            # Show progress
            local new_line=$(echo "$logs" | grep -E "^#[0-9]+" | tail -1)
            if [ "$new_line" != "$last_line" ] && [ -n "$new_line" ]; then
                echo "  $new_line"
                last_line="$new_line"
            fi
        fi
        
        sleep 1
    done
    
    # Download immediately
    log INFO "Downloading image..."
    if kubectl cp ${NAMESPACE}/${pod_name}:/output/${output_file} ${local_path} -c build; then
        # Signal download complete
        kubectl exec $pod_name -c build -n ${NAMESPACE} -- touch /output/download.done 2>/dev/null || true
        
        if [ -f "$local_path" ] && [ -s "$local_path" ]; then
            log SUCCESS "Image downloaded: $local_path ($(ls -lh "$local_path" | awk '{print $5}'))"
            return 0
        fi
    fi
    
    log ERROR "Failed to download image"
    return 1
}

# Validate OCI image
validate_image() {
    local image_path=$1
    
    log INFO "Validating OCI image..."
    
    if ! tar -tf "$image_path" >/dev/null 2>&1; then
        log ERROR "Invalid tar archive"
        return 1
    fi
    
    # Check required files
    local has_layout=$(tar -tf "$image_path" | grep -c "^oci-layout$" || true)
    local has_index=$(tar -tf "$image_path" | grep -c "^index.json$" || true)
    local has_blobs=$(tar -tf "$image_path" | grep -c "^blobs/" || true)
    
    if [ "$has_layout" -eq 0 ] || [ "$has_index" -eq 0 ] || [ "$has_blobs" -eq 0 ]; then
        log ERROR "Invalid OCI structure"
        return 1
    fi
    
    log SUCCESS "Valid OCI image"
    log INFO "  Blobs: $(tar -tf "$image_path" | grep -c "^blobs/sha256/" || echo 0)"
    
    # Show manifest info if jq available
    if command -v jq >/dev/null 2>&1; then
        local platform=$(tar -xOf "$image_path" index.json 2>/dev/null | jq -r '.manifests[0].platform | "\(.os)/\(.architecture)"' 2>/dev/null || echo "unknown")
        log INFO "  Platform: $platform"
    fi
    
    return 0
}

# Main
main() {
    if [ $# -lt 1 ]; then
        cat <<EOF
Usage: $0 <dockerfile> [context-dir] [output-file]

Build container images using BuildKit on Kubernetes (rootless)

Arguments:
  dockerfile    Path to Dockerfile
  context-dir   Build context directory (default: dockerfile directory)  
  output-file   Output filename (default: image.tar)

Environment:
  K8S_NAMESPACE    Kubernetes namespace (default: eir)
  BUILD_TIMEOUT    Build timeout in seconds (default: 300)
  DOWNLOAD_TIMEOUT Download timeout in seconds (default: 60)

Example:
  $0 Dockerfile . myimage.tar
EOF
        exit 1
    fi
    
    local dockerfile=$1
    local context_dir=${2:-$(dirname "$dockerfile")}
    local output_file=${3:-image.tar}
    local output_path="$(pwd)/$output_file"
    
    # Validate
    if [ ! -f "$dockerfile" ]; then
        log ERROR "Dockerfile not found: $dockerfile"
        exit 1
    fi
    
    if [ ! -d "$context_dir" ]; then
        log ERROR "Context directory not found: $context_dir"
        exit 1
    fi
    
    # Start build
    local start_time=$(date +%s)
    
    echo ""
    log INFO "🚀 Shmocker Kubernetes Build"
    log INFO "Dockerfile: $dockerfile"
    log INFO "Context: $context_dir"  
    log INFO "Output: $output_path"
    echo ""
    
    # Execute
    create_job "$dockerfile" "$context_dir" "$(basename $output_file)"
    
    if monitor_and_download "$(basename $output_file)" "$output_path"; then
        validate_image "$output_path"
        
        local duration=$(($(date +%s) - start_time))
        echo ""
        log SUCCESS "✅ Build complete in ${duration}s"
        log INFO "📦 Load with: docker load < $output_path"
        log INFO "🚢 Push with: skopeo copy oci-archive:$output_path docker://registry/image:tag"
    else
        exit 1
    fi
}

main "$@"
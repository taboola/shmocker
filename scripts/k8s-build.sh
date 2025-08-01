#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m'

# Configuration
NAMESPACE="${K8S_NAMESPACE:-default}"
IMAGE_NAME="shmocker-build-$$"
BUILD_TIMEOUT=${BUILD_TIMEOUT:-300}
DOWNLOAD_TIMEOUT=${DOWNLOAD_TIMEOUT:-60}

# Banner
show_banner() {
    echo ""
    echo -e "${CYAN}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}  🚀 ${BLUE}Shmocker${NC} - Because reinventing Docker is easier      ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}     than reading the docs™                                 ${CYAN}║${NC}"
    echo -e "${CYAN}╚════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# Logging with context
log() {
    local level=$1
    shift
    case $level in
        ERROR) echo -e "${RED}[ERROR]${NC} $*" >&2 ;;
        SUCCESS) echo -e "${GREEN}[SUCCESS]${NC} $*" ;;
        INFO) echo -e "${BLUE}[INFO]${NC} $*" ;;
        WARN) echo -e "${YELLOW}[WARN]${NC} $*" ;;
        DEBUG) echo -e "${GRAY}[DEBUG]${NC} $*" ;;
        STEP) echo -e "${CYAN}[STEP]${NC} $*" ;;
    esac
}

# Cleanup
cleanup() {
    if [ -n "$CLEANUP_DONE" ]; then
        return
    fi
    CLEANUP_DONE=1
    
    echo ""
    log STEP "Cleaning up Kubernetes resources..."
    kubectl delete job ${IMAGE_NAME} -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    kubectl delete configmap ${IMAGE_NAME}-dockerfile -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    kubectl delete configmap ${IMAGE_NAME}-context -n ${NAMESPACE} --ignore-not-found=true 2>/dev/null
    log DEBUG "Cleanup complete. Thanks for choosing the Docker replacement that Docker doesn't know about!"
}

trap cleanup EXIT

# Create build job with completion signaling
create_job() {
    local dockerfile_path=$1
    local context_dir=$2
    local output_file=$3
    
    log STEP "Preparing to build without Docker (yes, really!)..."
    log DEBUG "Creating ConfigMaps for your Dockerfile and context"
    
    # Create Dockerfile ConfigMap
    kubectl create configmap ${IMAGE_NAME}-dockerfile \
        --from-file=Dockerfile=${dockerfile_path} \
        -n ${NAMESPACE} \
        --dry-run=client -o yaml | kubectl apply -f - >/dev/null
    
    # Create context ConfigMap if needed
    local has_context=false
    # Find relevant context files, excluding common build artifacts
    local context_files=$(find ${context_dir} -type f \
        ! -name "*.dockerfile" \
        ! -name "Dockerfile" \
        ! -name "*.tar" \
        ! -name "*.tar.gz" \
        ! -name "*.tgz" \
        ! -name "*.zip" \
        ! -name ".git" \
        ! -path "*/.*" \
        -size -1M \
        2>/dev/null | head -20)
    
    if [ -n "$context_files" ]; then
        has_context=true
        log DEBUG "Found context files to include:"
        echo "$context_files" | while read f; do
            [ -n "$f" ] && log DEBUG "  • $(basename "$f") ($(ls -lh "$f" 2>/dev/null | awk '{print $5}'))"
        done
        
        # Create a temporary directory with only the needed files
        local temp_context=$(mktemp -d)
        echo "$context_files" | while read f; do
            [ -n "$f" ] && cp "$f" "$temp_context/" 2>/dev/null
        done
        
        # Check total size
        local total_size=$(du -sh "$temp_context" | awk '{print $1}')
        log DEBUG "Total context size: $total_size"
        
        if kubectl create configmap ${IMAGE_NAME}-context \
            --from-file=${temp_context} \
            -n ${NAMESPACE} \
            --dry-run=client -o yaml | kubectl apply -f - >/dev/null 2>&1; then
            log DEBUG "Context ConfigMap created successfully"
        else
            log WARN "Failed to create context ConfigMap (possibly too large)"
            log WARN "Building with Dockerfile only, no additional context files"
            has_context=false
        fi
        
        rm -rf "$temp_context"
    fi
    
    log STEP "Deploying BuildKit pod on Kubernetes (rootless, because we're fancy)..."
    
    # Job manifest with sidecar pattern
    cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${IMAGE_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: shmocker
    purpose: "docker-replacement"
    irony-level: "high"
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
          echo "📦 Preparing build context..."
          cp /dockerfile/Dockerfile /workspace/
          if [ -d /context ]; then
            cd /context
            for f in *; do
              [ -f "\$f" ] && [ "\$f" != "Dockerfile" ] && cp "\$f" /workspace/
            done
          fi
          echo "✓ Workspace ready with $(ls -1 /workspace 2>/dev/null | wc -l) files"
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
          echo "[BUILD] 🔨 Starting BuildKit (the engine that Docker wishes it invented)..."
          
          # Run the build
          if buildctl-daemonless.sh build \
            --frontend dockerfile.v0 \
            --local context=/workspace \
            --local dockerfile=/workspace \
            --output type=oci,dest=/output/${output_file} \
            --progress plain; then
            
            echo "[BUILD] ✅ Success! Image size: \$(ls -lh /output/${output_file} | awk '{print \$5}')"
            echo "[BUILD] 🎉 Built without Docker, root, or regrets!"
            touch /output/build.success
            
            # Signal completion
            echo "ready" > /output/download.signal
          else
            echo "[BUILD] ❌ Failed! But hey, at least we tried without Docker..."
            touch /output/build.failed
            echo "failed" > /output/download.signal
            exit 1
          fi
          
          # Wait for download
          echo "[BUILD] 📡 Waiting for download confirmation..."
          timeout=60
          while [ \$timeout -gt 0 ]; do
            if [ -f /output/download.done ]; then
              echo "[BUILD] 👋 Download complete, mission accomplished!"
              exit 0
            fi
            sleep 1
            timeout=\$((timeout - 1))
          done
          echo "[BUILD] ⏰ Download timeout - but the image is ready!"
          exit 0
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
      
      # Monitor sidecar
      - name: monitor
        image: busybox
        command: ["/bin/sh", "-c"]
        args:
        - |
          echo "[MONITOR] 👀 Watching for build completion..."
          
          # Wait for build signal
          while [ ! -f /output/download.signal ]; do
            sleep 1
          done
          
          # Check build status
          if grep -q "failed" /output/download.signal; then
            echo "[MONITOR] 💔 Build failed"
            exit 1
          fi
          
          echo "[MONITOR] 🎯 Build succeeded, standing by..."
          
          # Keep alive for download
          timeout=${DOWNLOAD_TIMEOUT}
          while [ \$timeout -gt 0 ]; do
            if [ -f /output/download.done ]; then
              exit 0
            fi
            sleep 1
            timeout=\$((timeout - 1))
          done
          exit 0
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

    log SUCCESS "BuildKit pod deployed! (No Docker daemon harmed in this process)"
}

# Monitor build and download when ready
monitor_and_download() {
    local output_file=$1
    local local_path=$2
    
    # Wait for pod to start
    log STEP "Waiting for BuildKit pod to start..."
    log DEBUG "Finding your pod among the Kubernetes wilderness..."
    
    local pod_name=""
    local dots=""
    for i in {1..30}; do
        pod_name=$(kubectl get pods -n ${NAMESPACE} -l job-name=${IMAGE_NAME} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        if [ -n "$pod_name" ]; then
            echo ""
            break
        fi
        dots="${dots}."
        printf "\r${GRAY}Searching for pod${dots}${NC}"
        sleep 1
    done
    
    if [ -z "$pod_name" ]; then
        echo ""
        log ERROR "Pod didn't show up. Even Kubernetes is confused about building without Docker!"
        return 1
    fi
    
    log SUCCESS "Pod discovered: ${pod_name}"
    log DEBUG "It's alive! (And rootless, which is the important part)"
    
    # Wait for init
    log STEP "Initializing build environment..."
    kubectl wait --for=condition=Initialized pod/$pod_name -n ${NAMESPACE} --timeout=60s >/dev/null 2>&1 || {
        log ERROR "Initialization failed. The irony is not lost on us."
        return 1
    }
    
    # Monitor build
    log STEP "Building your image (without Docker, as promised)..."
    echo -e "${GRAY}Build progress:${NC}"
    
    local last_line=""
    local build_done=false
    local start_time=$(date +%s)
    local spinner=('⠋' '⠙' '⠹' '⠸' '⠼' '⠴' '⠦' '⠧' '⠇' '⠏')
    local spin_idx=0
    
    while [ "$build_done" = false ]; do
        # Check timeout
        local current_time=$(date +%s)
        if [ $((current_time - start_time)) -gt $BUILD_TIMEOUT ]; then
            echo ""
            log ERROR "Build timeout. Even BuildKit has limits!"
            return 1
        fi
        
        # Get logs
        local logs=$(kubectl logs $pod_name -c build -n ${NAMESPACE} --tail=30 2>/dev/null || echo "")
        
        # Check completion
        if echo "$logs" | grep -q "Success!"; then
            build_done=true
            echo ""
            log SUCCESS "Build completed! 🎉"
            log DEBUG "See? Who needs Docker when you have BuildKit and determination?"
        elif echo "$logs" | grep -q "Failed!"; then
            echo ""
            log ERROR "Build failed. But at least it failed without Docker!"
            echo "$logs" | grep -A5 "error:" | tail -10
            return 1
        else
            # Show progress
            local new_line=$(echo "$logs" | grep -E "^#[0-9]+" | tail -1)
            if [ "$new_line" != "$last_line" ] && [ -n "$new_line" ]; then
                printf "\r${GRAY}  %s${NC}\n" "$new_line"
                last_line="$new_line"
            else
                printf "\r${spinner[$spin_idx]} Building..."
                spin_idx=$(( (spin_idx + 1) % ${#spinner[@]} ))
            fi
        fi
        
        sleep 0.5
    done
    
    # Download
    log STEP "Downloading your freshly-built OCI image..."
    log DEBUG "Transferring from Kubernetes ephemeral storage to your disk"
    
    # Use a temporary file to capture kubectl cp output
    local temp_output=$(mktemp)
    
    # Run kubectl cp and capture both stdout and stderr
    if kubectl cp ${NAMESPACE}/${pod_name}:/output/${output_file} ${local_path} -c build 2>&1 | tee "$temp_output" | grep -v "tar: removing" || true; then
        rm -f "$temp_output"
    else
        # If kubectl cp failed, show the actual error
        local exit_code=$?
        log DEBUG "kubectl cp exit code: $exit_code"
        cat "$temp_output" >&2
        rm -f "$temp_output"
    fi
    
    # Check if download succeeded by verifying the file exists
    if [ -f "$local_path" ] && [ -s "$local_path" ]; then
        # Signal completion
        kubectl exec $pod_name -c build -n ${NAMESPACE} -- touch /output/download.done 2>/dev/null || true
        
        local size=$(stat -f%z "$local_path" 2>/dev/null | awk '{size=$1/1048576; printf "%.1fM", size}' || echo "unknown")
        log SUCCESS "Image saved: $(basename $local_path) (${size})"
        return 0
    else
        # Try to understand why it failed
        log DEBUG "Checking pod status..."
        kubectl get pod $pod_name -n ${NAMESPACE} -o wide 2>&1 || true
        
        log DEBUG "Checking if output file exists in pod..."
        kubectl exec $pod_name -c build -n ${NAMESPACE} -- ls -la /output/ 2>&1 || true
        
        log ERROR "Download failed. The image exists but refuses to leave Kubernetes."
        return 1
    fi
}

# Validate OCI image
validate_image() {
    local image_path=$1
    
    log STEP "Validating OCI image structure..."
    log DEBUG "Making sure it's a real container image, not just a tarball with dreams"
    
    if ! tar -tf "$image_path" >/dev/null 2>&1; then
        log ERROR "Not a valid tar archive. BuildKit betrayed us!"
        return 1
    fi
    
    # Check OCI compliance
    local has_layout=$(tar -tf "$image_path" | grep -c "^oci-layout$" || true)
    local has_index=$(tar -tf "$image_path" | grep -c "^index.json$" || true)
    local has_blobs=$(tar -tf "$image_path" | grep -c "^blobs/" || true)
    
    if [ "$has_layout" -eq 0 ] || [ "$has_index" -eq 0 ] || [ "$has_blobs" -eq 0 ]; then
        log ERROR "Invalid OCI structure. This isn't the image you're looking for."
        return 1
    fi
    
    log SUCCESS "Valid OCI image confirmed! ✅"
    
    # Show details
    local blob_count=$(tar -tf "$image_path" | grep -c "^blobs/sha256/" || echo 0)
    log INFO "Image details:"
    log INFO "  • Format: OCI (Open Container Initiative)"
    log INFO "  • Layers: ${blob_count}"
    
    if command -v jq >/dev/null 2>&1; then
        local platform=$(tar -xOf "$image_path" index.json 2>/dev/null | jq -r '.manifests[0].platform | "\(.os)/\(.architecture)"' 2>/dev/null || echo "unknown")
        log INFO "  • Platform: ${platform}"
        log INFO "  • Compatible with: Any OCI-compliant runtime (Docker, Podman, etc.)"
    fi
    
    return 0
}

# Main
main() {
    if [ $# -lt 1 ]; then
        show_banner
        cat <<EOF
${BLUE}Welcome to Shmocker!${NC} The rootless, daemonless, Docker-less way to build
container images. Because sometimes you just need to build an image without
installing Docker, gaining root access, or questioning your life choices.

${CYAN}Usage:${NC} $0 <dockerfile> [context-dir] [output-file]

${CYAN}Arguments:${NC}
  ${GREEN}dockerfile${NC}    Path to your Dockerfile
  ${GREEN}context-dir${NC}   Build context directory (default: dockerfile's directory)  
  ${GREEN}output-file${NC}   Output filename (default: image.tar)

${CYAN}What it does:${NC}
  1. Uploads your Dockerfile to Kubernetes (as a ConfigMap)
  2. Spins up a rootless BuildKit pod (no Docker daemon!)
  3. Builds your image in a secure, isolated environment
  4. Downloads the OCI image to your local machine
  5. Cleans up, leaving no trace (except your image)

${CYAN}Environment Variables:${NC}
  ${YELLOW}K8S_NAMESPACE${NC}    Target namespace (default: default)
  ${YELLOW}BUILD_TIMEOUT${NC}    Max build time in seconds (default: 300)
  ${YELLOW}DOWNLOAD_TIMEOUT${NC} Max download time in seconds (default: 60)

${CYAN}Example:${NC}
  $0 Dockerfile . my-awesome-image.tar

${GRAY}Remember: Friends don't let friends run Docker daemons in production.${NC}
EOF
        exit 1
    fi
    
    local dockerfile=$1
    local context_dir=${2:-$(dirname "$dockerfile")}
    local output_file=${3:-image.tar}
    local output_path="$(pwd)/$output_file"
    
    # Validate inputs
    if [ ! -f "$dockerfile" ]; then
        log ERROR "Dockerfile not found: $dockerfile"
        log DEBUG "Can't build without instructions. We're good, but not that good."
        exit 1
    fi
    
    if [ ! -d "$context_dir" ]; then
        log ERROR "Context directory not found: $context_dir"
        exit 1
    fi
    
    # Start
    show_banner
    local start_time=$(date +%s)
    
    log INFO "🎯 Build Request:"
    log INFO "  • Dockerfile: ${CYAN}$dockerfile${NC}"
    log INFO "  • Context: ${CYAN}$context_dir${NC}"
    log INFO "  • Output: ${CYAN}$output_path${NC}"
    log INFO "  • Namespace: ${CYAN}$NAMESPACE${NC}"
    echo ""
    
    # Execute
    create_job "$dockerfile" "$context_dir" "$(basename $output_file)"
    
    if monitor_and_download "$(basename $output_file)" "$output_path"; then
        echo ""
        validate_image "$output_path"
        
        local duration=$(($(date +%s) - start_time))
        echo ""
        log SUCCESS "✨ Build completed in ${duration} seconds!"
        echo ""
        log INFO "📦 To use your image:"
        log INFO "  • With Docker: ${CYAN}docker load < $output_path${NC}"
        log INFO "  • With Podman: ${CYAN}podman load --input $output_path${NC}"
        log INFO "  • With Skopeo: ${CYAN}skopeo copy oci-archive:$output_path docker://registry/repo:tag${NC}"
        echo ""
        log DEBUG "Remember: This image was built without Docker. Share responsibly! 🚀"
    else
        echo ""
        log ERROR "Build failed. Check the logs above for details."
        log DEBUG "Even the best Docker alternatives have bad days."
        exit 1
    fi
}

main "$@"
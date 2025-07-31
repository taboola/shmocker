#!/bin/bash
# Setup script for shmocker on macOS using Colima
# This script installs Colima and sets up the BuildKit environment for shmocker

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COLIMA_PROFILE="shmocker"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
COLIMA_CONFIG_DIR="$PROJECT_ROOT/colima"

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

# Check if running on macOS
check_macos() {
    if [[ "$OSTYPE" != "darwin"* ]]; then
        log_error "This script is designed for macOS only"
        exit 1
    fi
}

# Check system requirements
check_requirements() {
    log_info "Checking system requirements..."
    
    # Check macOS version (Colima requires macOS 11+)
    macos_version=$(sw_vers -productVersion)
    log_info "macOS version: $macos_version"
    
    # Extract major version number
    major_version=$(echo "$macos_version" | cut -d '.' -f 1)
    if [[ $major_version -lt 11 ]]; then
        log_error "Colima requires macOS 11.0 or later. Found: $macos_version"
        exit 1
    fi
    
    # Check if we have enough resources
    cpu_count=$(sysctl -n hw.ncpu)
    memory_gb=$(($(sysctl -n hw.memsize) / 1024 / 1024 / 1024))
    
    log_info "System resources: ${cpu_count} CPUs, ${memory_gb}GB RAM"
    
    if [[ $cpu_count -lt 4 ]]; then
        log_warning "System has less than 4 CPUs. BuildKit performance may be limited."
    fi
    
    if [[ $memory_gb -lt 8 ]]; then
        log_warning "System has less than 8GB RAM. Consider reducing VM memory allocation."
    fi
    
    # Check available disk space
    available_space=$(df -g / | awk 'NR==2 {print $4}')
    if [[ $available_space -lt 50 ]]; then
        log_warning "Less than 50GB free disk space available. BuildKit may run out of space."
    fi
    
    # Check for virtualization support
    # Apple Silicon Macs use Hypervisor.framework, not VMX
    if [[ $(uname -m) == "arm64" ]]; then
        # Apple Silicon - virtualization is always available
        log_info "Apple Silicon detected - virtualization supported via Hypervisor.framework"
    elif ! sysctl -n machdep.cpu.features 2>/dev/null | grep -q VMX; then
        # Intel Mac without VMX
        log_error "Hardware virtualization (VMX) not supported on Intel Mac"
        exit 1
    fi
}

# Install Homebrew if not present
install_homebrew() {
    if ! command -v brew &> /dev/null; then
        log_info "Installing Homebrew..."
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
        
        # Add Homebrew to PATH for the current session
        if [[ -f "/opt/homebrew/bin/brew" ]]; then
            # Apple Silicon Mac
            eval "$(/opt/homebrew/bin/brew shellenv)"
        elif [[ -f "/usr/local/bin/brew" ]]; then
            # Intel Mac
            eval "$(/usr/local/bin/brew shellenv)"
        fi
        
        log_success "Homebrew installed successfully"
    else
        log_info "Homebrew already installed"
    fi
}

# Install Colima and dependencies
install_colima() {
    log_info "Installing Colima and dependencies..."
    
    # Install Colima
    if ! command -v colima &> /dev/null; then
        log_info "Installing Colima..."
        brew install colima
        log_success "Colima installed successfully"
    else
        log_info "Colima already installed"
        colima_version=$(colima version 2>/dev/null || echo "unknown")
        log_info "Colima version: $colima_version"
        
        # Check if Colima needs updating
        if brew outdated colima &> /dev/null; then
            log_info "Updating Colima to latest version..."
            brew upgrade colima
            log_success "Colima updated successfully"
        fi
    fi
    
    # Install Docker CLI (needed for container operations)
    if ! command -v docker &> /dev/null; then
        log_info "Installing Docker CLI..."
        brew install docker
        log_success "Docker CLI installed successfully"
    else
        log_info "Docker CLI already installed"
    fi
    
    # Install BuildKit client
    if ! command -v buildctl &> /dev/null; then
        log_info "Installing BuildKit client..."
        brew install buildkit
        log_success "BuildKit client installed successfully"
    else
        log_info "BuildKit client already installed"
    fi
}

# Create required directories
create_directories() {
    log_info "Creating required directories..."
    
    # Create shmocker cache directory
    mkdir -p "$HOME/.shmocker/cache"
    
    # Create temporary build directory
    mkdir -p "/tmp/shmocker-builds"
    
    # Create Docker config directory if it doesn't exist
    mkdir -p "$HOME/.docker"
    
    # Create Colima config directory
    mkdir -p "$COLIMA_CONFIG_DIR"
    
    log_success "Directories created successfully"
}

# Check if Colima profile already exists
check_existing_profile() {
    if colima list | grep -q "^$COLIMA_PROFILE"; then
        log_warning "Colima profile '$COLIMA_PROFILE' already exists"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Stopping and deleting existing profile..."
            colima stop "$COLIMA_PROFILE" 2>/dev/null || true
            colima delete "$COLIMA_PROFILE" 2>/dev/null || true
            log_success "Existing profile removed"
        else
            log_info "Using existing profile"
            return 1
        fi
    fi
    return 0
}

# Create Colima configuration
create_colima_config() {
    log_info "Creating Colima configuration..."
    
    # Determine optimal resource allocation
    cpu_count=$(sysctl -n hw.ncpu)
    memory_gb=$(($(sysctl -n hw.memsize) / 1024 / 1024 / 1024))
    
    # Use 75% of available CPUs, min 2, max 8
    vm_cpus=$(( cpu_count * 3 / 4 ))
    if [[ $vm_cpus -lt 2 ]]; then vm_cpus=2; fi
    if [[ $vm_cpus -gt 8 ]]; then vm_cpus=8; fi
    
    # Use 50% of available memory, min 4GB, max 16GB
    vm_memory=$(( memory_gb / 2 ))
    if [[ $vm_memory -lt 4 ]]; then vm_memory=4; fi
    if [[ $vm_memory -gt 16 ]]; then vm_memory=16; fi
    
    log_info "Configuring VM with ${vm_cpus} CPUs and ${vm_memory}GB RAM"
    
    # Create Colima configuration file
    cat > "$COLIMA_CONFIG_DIR/colima.yaml" << EOF
# Colima configuration for shmocker BuildKit
# This file configures a virtual machine optimized for container builds

# VM configuration
cpu: $vm_cpus
memory: $vm_memory
disk: 64

# Use QEMU for better performance and compatibility
vmType: qemu

# Container runtime - use containerd for BuildKit compatibility
runtime: containerd

# Network configuration
network:
  address: true
  dns: []
  dnsHosts: {}

# Enable BuildKit features
kubernetes:
  enabled: false

# Docker daemon configuration
docker:
  features:
    buildkit: true

# Port forwarding for BuildKit daemon
forwardAgent: false

# Mount configuration
mounts:
  - location: ~/.shmocker/cache
    mountPoint: /var/lib/buildkit/cache
    writable: true
  - location: /tmp/shmocker-builds
    mountPoint: /tmp/builds
    writable: true
  - location: ~/.docker
    mountPoint: /home/user/.docker
    writable: true

# Provisioning scripts
provision:
  - mode: system
    script: |
      #!/bin/bash
      set -eux -o pipefail
      
      # Update package lists
      apt-get update
      
      # Install required packages for BuildKit
      apt-get install -y \\
        curl \\
        wget \\
        ca-certificates \\
        gnupg \\
        lsb-release \\
        jq \\
        uidmap \\
        dbus-user-session \\
        fuse-overlayfs \\
        slirp4netns
      
      # Setup directories with proper permissions
      mkdir -p /var/lib/buildkit/cache
      mkdir -p /tmp/builds
      chown -R 1000:1000 /var/lib/buildkit
      chown -R 1000:1000 /tmp/builds

  - mode: user
    script: |
      #!/bin/bash
      set -eux -o pipefail
      
      # Install BuildKit from official releases
      BUILDKIT_VERSION="v0.12.4"
      BUILDKIT_ARCH="amd64"
      
      # Download and install BuildKit
      cd /tmp
      wget -q "https://github.com/moby/buildkit/releases/download/\${BUILDKIT_VERSION}/buildkit-\${BUILDKIT_VERSION}.linux-\${BUILDKIT_ARCH}.tar.gz"
      tar -xzf "buildkit-\${BUILDKIT_VERSION}.linux-\${BUILDKIT_ARCH}.tar.gz"
      sudo mv bin/* /usr/local/bin/
      rm -rf bin/ "buildkit-\${BUILDKIT_VERSION}.linux-\${BUILDKIT_ARCH}.tar.gz"
      
      # Create BuildKit configuration directory
      mkdir -p ~/.config/buildkit
      
      # Create BuildKit daemon configuration
      cat > ~/.config/buildkit/buildkitd.toml << 'BUILDKIT_EOF'
      # BuildKit daemon configuration for shmocker with Colima
      debug = false
      insecure-entitlements = []
      
      [grpc]
        address = ["unix:///run/user/1000/buildkit/buildkitd.sock"]
        debugAddress = "0.0.0.0:6060"
      
      [worker.oci]
        enabled = true
        platforms = ["linux/amd64", "linux/arm64", "linux/arm/v7", "linux/arm/v6", "linux/386", "linux/ppc64le", "linux/s390x"]
        snapshotter = "overlayfs"
        rootless = true
        
        # Configure for optimal performance
        [worker.oci.gc]
          gc = true
          gckeepstorage = "10gb"
          
        [worker.oci.gcpolicy]
          keepDuration = "168h"  # 1 week
          keepBytes = "5gb"
          filters = ["until=168h"]
      
      [worker.containerd]
        enabled = false
      
      # Registry configuration
      [registry]
        [registry."docker.io"]
          mirrors = ["https://registry-1.docker.io"]
        [registry."gcr.io"]
          mirrors = ["https://gcr.io"]
        [registry."ghcr.io"]
          mirrors = ["https://ghcr.io"]
      BUILDKIT_EOF
      
      # Create systemd user service for BuildKit daemon
      mkdir -p ~/.config/systemd/user
      cat > ~/.config/systemd/user/buildkit.service << 'SERVICE_EOF'
      [Unit]
      Description=BuildKit daemon for shmocker
      Documentation=https://github.com/moby/buildkit
      
      [Service]
      Type=simple
      ExecStart=/usr/local/bin/buildkitd --config=%h/.config/buildkit/buildkitd.toml
      Restart=always
      RestartSec=5
      KillMode=mixed
      KillSignal=SIGTERM
      TimeoutStopSec=30
      
      # Resource limits
      LimitNOFILE=65536
      LimitNPROC=8192
      
      # Security settings
      NoNewPrivileges=true
      ProtectSystem=strict
      ProtectHome=false
      PrivateTmp=true
      
      # Environment
      Environment=BUILDKIT_HOST=unix:///run/user/1000/buildkit/buildkitd.sock
      Environment=XDG_RUNTIME_DIR=/run/user/1000
      
      [Install]
      WantedBy=default.target
      SERVICE_EOF
      
      # Create runtime directory for BuildKit
      mkdir -p /run/user/1000/buildkit
      
      # Enable systemd user services
      systemctl --user daemon-reload
      systemctl --user enable buildkit.service
      systemctl --user start buildkit.service
      
      # Verify BuildKit is running
      sleep 5
      if systemctl --user is-active --quiet buildkit.service; then
        echo "BuildKit daemon started successfully"
        /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock debug workers || true
      else
        echo "Failed to start BuildKit daemon"
        systemctl --user status buildkit.service || true
      fi
      
      # Create convenience script for BuildKit client
      cat > ~/buildctl.sh << 'BUILDCTL_EOF'
      #!/bin/bash
      # Convenience script to run buildctl with correct socket
      exec /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "\$@"
      BUILDCTL_EOF
      chmod +x ~/buildctl.sh
EOF
    
    log_success "Colima configuration created"
}

# Start Colima with BuildKit configuration
start_colima() {
    log_info "Starting Colima with BuildKit configuration..."
    
    if ! check_existing_profile; then
        # Profile exists and user chose not to recreate
        log_info "Ensuring existing profile is running..."
        if ! colima status "$COLIMA_PROFILE" &>/dev/null; then
            colima start "$COLIMA_PROFILE"
        fi
        return 0
    fi
    
    # Start new profile with our configuration
    log_info "This may take several minutes on first run..."
    if colima start --profile "$COLIMA_PROFILE" --config "$COLIMA_CONFIG_DIR/colima.yaml"; then
        log_success "Colima started successfully"
    else
        log_error "Failed to start Colima"
        log_info "Check the logs with: colima logs $COLIMA_PROFILE"
        exit 1
    fi
}

# Verify BuildKit is running
verify_buildkit() {
    log_info "Verifying BuildKit daemon is running..."
    
    # Wait for VM to be fully ready
    sleep 10
    
    # Check if BuildKit service is active
    if colima ssh --profile "$COLIMA_PROFILE" -- systemctl --user is-active --quiet buildkit.service; then
        log_success "BuildKit daemon is running"
        
        # Test BuildKit connectivity
        log_info "Testing BuildKit connectivity..."
        if colima ssh --profile "$COLIMA_PROFILE" -- /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock debug workers &> /dev/null; then
            log_success "BuildKit is responding to requests"
        else
            log_warning "BuildKit daemon is running but not responding to requests"
            log_info "This might resolve after a few seconds. You can test manually with:"
            log_info "  ./scripts/colima-buildctl.sh debug workers"
        fi
    else
        log_error "BuildKit daemon is not running"
        log_info "Check the service status with:"
        log_info "  colima ssh --profile $COLIMA_PROFILE -- systemctl --user status buildkit.service"
        exit 1
    fi
}

# Create convenience scripts
create_convenience_scripts() {
    log_info "Creating convenience scripts..."
    
    # Create buildctl wrapper script
    cat > "$PROJECT_ROOT/scripts/colima-buildctl.sh" << 'EOF'
#!/bin/bash
# Wrapper script for buildctl that connects to Colima VM
COLIMA_PROFILE="shmocker"

if ! colima status "$COLIMA_PROFILE" &>/dev/null; then
    echo "Error: Colima profile '$COLIMA_PROFILE' is not running"
    echo "Start it with: colima start $COLIMA_PROFILE"
    exit 1
fi

exec colima ssh --profile "$COLIMA_PROFILE" -- /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "$@"
EOF
    chmod +x "$PROJECT_ROOT/scripts/colima-buildctl.sh"
    
    # Create Colima management script
    cat > "$PROJECT_ROOT/scripts/colima-vm.sh" << 'EOF'
#!/bin/bash
# Colima VM management script for shmocker
COLIMA_PROFILE="shmocker"

case "$1" in
    start)
        echo "Starting Colima profile..."
        colima start "$COLIMA_PROFILE"
        ;;
    stop)
        echo "Stopping Colima profile..."
        colima stop "$COLIMA_PROFILE"
        ;;
    restart)
        echo "Restarting Colima profile..."
        colima stop "$COLIMA_PROFILE"
        colima start "$COLIMA_PROFILE"
        ;;
    status)
        colima status "$COLIMA_PROFILE"
        ;;
    list)
        colima list
        ;;
    shell)
        colima ssh --profile "$COLIMA_PROFILE"
        ;;
    logs)
        if [[ -n "$2" ]]; then
            colima ssh --profile "$COLIMA_PROFILE" -- systemctl --user status buildkit -n "$2"
        else
            colima ssh --profile "$COLIMA_PROFILE" -- systemctl --user status buildkit
        fi
        ;;
    buildctl)
        shift
        colima ssh --profile "$COLIMA_PROFILE" -- /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "$@"
        ;;
    delete)
        echo "Deleting Colima profile..."
        colima delete "$COLIMA_PROFILE"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|list|shell|logs|buildctl|delete}"
        echo ""
        echo "Commands:"
        echo "  start    - Start the Colima profile"
        echo "  stop     - Stop the Colima profile"
        echo "  restart  - Restart the Colima profile"
        echo "  status   - Show profile status"
        echo "  list     - List all Colima profiles"
        echo "  shell    - Open shell in the VM"
        echo "  logs     - Show BuildKit daemon logs"
        echo "  buildctl - Run buildctl command in VM"
        echo "  delete   - Delete the Colima profile"
        exit 1
        ;;
esac
EOF
    chmod +x "$PROJECT_ROOT/scripts/colima-vm.sh"
    
    log_success "Convenience scripts created"
}

# Set up shell environment
setup_environment() {
    log_info "Setting up environment variables..."
    
    # Create environment file
    cat > "$PROJECT_ROOT/.env.colima" << EOF
# Colima BuildKit environment for shmocker
export COLIMA_PROFILE=shmocker
export BUILDKIT_HOST=unix:///run/user/1000/buildkit/buildkitd.sock
export SHMOCKER_BACKEND=colima
export DOCKER_HOST=unix://$HOME/.colima/shmocker/docker.sock
EOF
    
    log_info "Environment file created at $PROJECT_ROOT/.env.colima"
    log_info "Source it in your shell with: source .env.colima"
}

# Display final instructions
show_final_instructions() {
    log_success "shmocker Colima setup completed successfully!"
    echo
    log_info "Next steps:"
    echo "1. Source the environment file: source .env.colima"
    echo "2. Test BuildKit: ./scripts/colima-buildctl.sh debug workers"
    echo "3. Build an image: shmocker build ./examples"
    echo
    log_info "Profile Management:"
    echo "• Start Profile:    ./scripts/colima-vm.sh start"
    echo "• Stop Profile:     ./scripts/colima-vm.sh stop"
    echo "• Profile Status:   ./scripts/colima-vm.sh status"
    echo "• Open Shell:       ./scripts/colima-vm.sh shell"
    echo "• View Logs:        ./scripts/colima-vm.sh logs"
    echo
    log_info "Colima Profile: $COLIMA_PROFILE"
    log_info "BuildKit Socket: unix:///run/user/1000/buildkit/buildkitd.sock"
    echo
    log_warning "Note: The Colima VM will consume ~${vm_memory:-8}GB RAM and ~64GB disk space"
    log_warning "Stop the profile when not in use to free up resources: ./scripts/colima-vm.sh stop"
}

# Handle cleanup on script exit
cleanup() {
    local exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
        log_error "Setup failed with exit code $exit_code"
        log_info "You can re-run this script to retry the setup"
        log_info "For manual troubleshooting, check: colima list"
    fi
}

# Main setup function
main() {
    echo
    log_info "🚀 Setting up shmocker with Colima BuildKit on macOS"
    echo
    
    trap cleanup EXIT
    
    check_macos
    check_requirements
    install_homebrew
    install_colima
    create_directories
    create_colima_config
    start_colima
    verify_buildkit
    create_convenience_scripts
    setup_environment
    show_final_instructions
}

# Help function
show_help() {
    echo "shmocker macOS Colima Setup Script"
    echo
    echo "This script sets up shmocker to use BuildKit running in a Colima VM on macOS."
    echo "Colima is a simpler alternative to Lima, specifically designed for container workloads."
    echo
    echo "Usage: $0 [OPTION]"
    echo
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --version  Show version information"
    echo
    echo "Requirements:"
    echo "  • macOS 11.0 or later"
    echo "  • At least 4GB RAM available for VM (8GB recommended)"
    echo "  • At least 50GB free disk space"
    echo "  • Internet connection for downloading VM image and packages"
    echo
    echo "What this script does:"
    echo "  1. Installs Homebrew (if not present)"
    echo "  2. Installs Colima VM runtime and Docker CLI"
    echo "  3. Creates and starts a Linux VM with BuildKit"
    echo "  4. Configures BuildKit for rootless container builds"
    echo "  5. Creates convenience scripts for profile management"
    echo "  6. Sets up environment for shmocker to use Colima BuildKit"
    echo
    echo "Advantages of Colima over Lima:"
    echo "  • Simpler configuration and management"
    echo "  • Optimized for container workloads"
    echo "  • Better Docker compatibility"
    echo "  • Automatic resource management"
    echo "  • Built-in BuildKit support"
    echo
}

# Parse command line arguments
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -v|--version)
        echo "shmocker Colima setup script v1.0.0"
        exit 0
        ;;
    "")
        main
        ;;
    *)
        log_error "Unknown option: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac
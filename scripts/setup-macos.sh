#!/bin/bash
# Setup script for shmocker on macOS using Lima
# This script installs Lima and sets up the BuildKit VM for shmocker

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
LIMA_VM_NAME="shmocker-buildkit"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
LIMA_CONFIG="$PROJECT_ROOT/lima/buildkit.yaml"

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
    
    # Check macOS version (Lima requires macOS 10.15+)
    macos_version=$(sw_vers -productVersion)
    log_info "macOS version: $macos_version"
    
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
    if [[ $available_space -lt 80 ]]; then
        log_warning "Less than 80GB free disk space available. BuildKit may run out of space."
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

# Install Lima
install_lima() {
    if ! command -v lima &> /dev/null; then
        log_info "Installing Lima..."
        brew install lima
        log_success "Lima installed successfully"
    else
        log_info "Lima already installed"
        lima_version=$(lima --version 2>/dev/null || echo "unknown")
        log_info "Lima version: $lima_version"
        
        # Check if Lima needs updating
        if brew outdated lima &> /dev/null; then
            log_info "Updating Lima to latest version..."
            brew upgrade lima
            log_success "Lima updated successfully"
        fi
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
    
    log_success "Directories created successfully"
}

# Check if VM already exists
check_existing_vm() {
    if limactl list | grep -q "^$LIMA_VM_NAME"; then
        log_warning "Lima VM '$LIMA_VM_NAME' already exists"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log_info "Stopping and deleting existing VM..."
            limactl stop "$LIMA_VM_NAME" 2>/dev/null || true
            limactl delete "$LIMA_VM_NAME" 2>/dev/null || true
            log_success "Existing VM removed"
        else
            log_info "Using existing VM"
            return 1
        fi
    fi
    return 0
}

# Start Lima VM
start_lima_vm() {
    log_info "Starting Lima VM with BuildKit configuration..."
    
    if ! check_existing_vm; then
        # VM exists and user chose not to recreate
        log_info "Ensuring existing VM is running..."
        limactl start "$LIMA_VM_NAME"
        return 0
    fi
    
    # Start new VM with our configuration
    log_info "This may take several minutes on first run..."
    if limactl start --name="$LIMA_VM_NAME" "$LIMA_CONFIG"; then
        log_success "Lima VM started successfully"
    else
        log_error "Failed to start Lima VM"
        log_info "Check the logs with: limactl shell $LIMA_VM_NAME sudo journalctl -u buildkit --no-pager"
        exit 1
    fi
}

# Verify BuildKit is running
verify_buildkit() {
    log_info "Verifying BuildKit daemon is running..."
    
    # Wait for VM to be fully ready
    sleep 5
    
    # Check if BuildKit service is active
    if limactl shell "$LIMA_VM_NAME" systemctl --user is-active --quiet buildkit.service; then
        log_success "BuildKit daemon is running"
        
        # Test BuildKit connectivity
        log_info "Testing BuildKit connectivity..."
        if limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock debug workers &> /dev/null; then
            log_success "BuildKit is responding to requests"
        else
            log_warning "BuildKit daemon is running but not responding to requests"
            log_info "This might resolve after a few seconds. You can test manually with:"
            log_info "  limactl shell $LIMA_VM_NAME ./buildctl.sh debug workers"
        fi
    else
        log_error "BuildKit daemon is not running"
        log_info "Check the service status with:"
        log_info "  limactl shell $LIMA_VM_NAME systemctl --user status buildkit.service"
        exit 1
    fi
}

# Create convenience scripts
create_convenience_scripts() {
    log_info "Creating convenience scripts..."
    
    # Create buildctl wrapper script
    cat > "$PROJECT_ROOT/scripts/buildctl.sh" << 'EOF'
#!/bin/bash
# Wrapper script for buildctl that connects to Lima VM
LIMA_VM_NAME="shmocker-buildkit"

if ! limactl list | grep -q "^$LIMA_VM_NAME.*Running"; then
    echo "Error: Lima VM '$LIMA_VM_NAME' is not running"
    echo "Start it with: limactl start $LIMA_VM_NAME"
    exit 1
fi

exec limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "$@"
EOF
    chmod +x "$PROJECT_ROOT/scripts/buildctl.sh"
    
    # Create VM management script
    cat > "$PROJECT_ROOT/scripts/lima-vm.sh" << 'EOF'
#!/bin/bash
# Lima VM management script for shmocker
LIMA_VM_NAME="shmocker-buildkit"

case "$1" in
    start)
        echo "Starting Lima VM..."
        limactl start "$LIMA_VM_NAME"
        ;;
    stop)
        echo "Stopping Lima VM..."
        limactl stop "$LIMA_VM_NAME"
        ;;
    restart)
        echo "Restarting Lima VM..."
        limactl stop "$LIMA_VM_NAME"
        limactl start "$LIMA_VM_NAME"
        ;;
    status)
        limactl list | grep "$LIMA_VM_NAME" || echo "VM not found"
        ;;
    shell)
        limactl shell "$LIMA_VM_NAME"
        ;;
    logs)
        limactl shell "$LIMA_VM_NAME" sudo journalctl -u buildkit -f
        ;;
    buildctl)
        shift
        limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock "$@"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|shell|logs|buildctl}"
        echo ""
        echo "Commands:"
        echo "  start    - Start the Lima VM"
        echo "  stop     - Stop the Lima VM"
        echo "  restart  - Restart the Lima VM"
        echo "  status   - Show VM status"
        echo "  shell    - Open shell in the VM"
        echo "  logs     - Show BuildKit daemon logs"
        echo "  buildctl - Run buildctl command in VM"
        exit 1
        ;;
esac
EOF
    chmod +x "$PROJECT_ROOT/scripts/lima-vm.sh"
    
    log_success "Convenience scripts created"
}

# Set up shell environment
setup_environment() {
    log_info "Setting up environment variables..."
    
    # Create environment file
    cat > "$PROJECT_ROOT/.env.lima" << EOF
# Lima BuildKit environment for shmocker
export BUILDKIT_HOST=tcp://127.0.0.1:1234
export LIMA_VM_NAME=shmocker-buildkit
export SHMOCKER_BACKEND=lima
EOF
    
    log_info "Environment file created at $PROJECT_ROOT/.env.lima"
    log_info "Source it in your shell with: source .env.lima"
}

# Display final instructions
show_final_instructions() {
    log_success "shmocker Lima setup completed successfully!"
    echo
    log_info "Next steps:"
    echo "1. Source the environment file: source .env.lima"
    echo "2. Test BuildKit: ./scripts/buildctl.sh debug workers"
    echo "3. Build an image: shmocker build ./examples"
    echo
    log_info "VM Management:"
    echo "• Start VM:    ./scripts/lima-vm.sh start"
    echo "• Stop VM:     ./scripts/lima-vm.sh stop"
    echo "• VM Status:   ./scripts/lima-vm.sh status"
    echo "• Open Shell:  ./scripts/lima-vm.sh shell"
    echo "• View Logs:   ./scripts/lima-vm.sh logs"
    echo
    log_info "BuildKit endpoint: tcp://127.0.0.1:1234"
    log_info "VM Name: $LIMA_VM_NAME"
    echo
    log_warning "Note: The Lima VM will consume ~8GB RAM and ~64GB disk space"
    log_warning "Stop the VM when not in use to free up resources: ./scripts/lima-vm.sh stop"
}

# Handle cleanup on script exit
cleanup() {
    local exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
        log_error "Setup failed with exit code $exit_code"
        log_info "You can re-run this script to retry the setup"
        log_info "For manual troubleshooting, check: limactl list"
    fi
}

# Main setup function
main() {
    echo
    log_info "🚀 Setting up shmocker with Lima BuildKit on macOS"
    echo
    
    trap cleanup EXIT
    
    check_macos
    check_requirements
    install_homebrew
    install_lima
    create_directories
    start_lima_vm
    verify_buildkit
    create_convenience_scripts
    setup_environment
    show_final_instructions
}

# Help function
show_help() {
    echo "shmocker macOS Setup Script"
    echo
    echo "This script sets up shmocker to use BuildKit running in a Lima VM on macOS."
    echo
    echo "Usage: $0 [OPTION]"
    echo
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --version  Show version information"
    echo
    echo "Requirements:"
    echo "  • macOS 10.15 or later"
    echo "  • At least 4GB RAM available for VM (8GB recommended)"
    echo "  • At least 80GB free disk space"
    echo "  • Internet connection for downloading VM image and packages"
    echo
    echo "What this script does:"
    echo "  1. Installs Homebrew (if not present)"
    echo "  2. Installs Lima VM runtime"
    echo "  3. Creates and starts a Ubuntu VM with BuildKit"
    echo "  4. Configures BuildKit for rootless container builds"
    echo "  5. Creates convenience scripts for VM management"
    echo "  6. Sets up environment for shmocker to use Lima BuildKit"
    echo
}

# Parse command line arguments
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -v|--version)
        echo "shmocker Lima setup script v1.0.0"
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
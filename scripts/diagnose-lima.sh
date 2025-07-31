#!/bin/bash
# Lima BuildKit diagnostic script for shmocker
# This script helps diagnose and troubleshoot Lima setup issues

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
LIMA_VM_NAME="shmocker-buildkit"
BUILDKIT_PORT=1234

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_section() {
    echo
    echo -e "${BLUE}=== $1 ===${NC}"
}

# Check if running on macOS
check_platform() {
    log_section "Platform Check"
    
    if [[ "$OSTYPE" == "darwin"* ]]; then
        macos_version=$(sw_vers -productVersion)
        log_success "Running on macOS $macos_version"
        
        # Check macOS version compatibility
        if [[ $(echo "$macos_version" | cut -d. -f1) -lt 11 ]]; then
            log_warning "macOS version may not be fully compatible with Lima"
        fi
    else
        log_error "This script is designed for macOS only"
        exit 1
    fi
    
    # Check architecture
    arch=$(uname -m)
    log_info "Architecture: $arch"
    
    if [[ "$arch" == "arm64" ]]; then
        log_info "Apple Silicon Mac detected"
    elif [[ "$arch" == "x86_64" ]]; then
        log_info "Intel Mac detected"
    fi
}

# Check system resources
check_resources() {
    log_section "System Resources"
    
    cpu_count=$(sysctl -n hw.ncpu)
    memory_gb=$(($(sysctl -n hw.memsize) / 1024 / 1024 / 1024))
    
    log_info "CPUs: $cpu_count"
    log_info "Memory: ${memory_gb}GB"
    
    if [[ $cpu_count -ge 4 ]]; then
        log_success "CPU count is adequate"
    else
        log_warning "Low CPU count may affect build performance"
    fi
    
    if [[ $memory_gb -ge 8 ]]; then
        log_success "Memory is adequate"
    else
        log_warning "Low memory may cause issues (8GB+ recommended)"
    fi
    
    # Check available disk space
    available_space=$(df -g / | awk 'NR==2 {print $4}')
    log_info "Available disk space: ${available_space}GB"
    
    if [[ $available_space -ge 80 ]]; then
        log_success "Disk space is adequate"
    else
        log_warning "Low disk space (80GB+ recommended)"
    fi
}

# Check Lima installation
check_lima_installation() {
    log_section "Lima Installation"
    
    if command -v limactl &> /dev/null; then
        lima_version=$(limactl --version 2>/dev/null || echo "unknown")
        log_success "Lima is installed: $lima_version"
        
        # Check if Lima is up to date
        if command -v brew &> /dev/null; then
            if brew outdated lima &> /dev/null; then
                log_warning "Lima update available: brew upgrade lima"
            else
                log_success "Lima is up to date"
            fi
        fi
    else
        log_error "Lima is not installed"
        log_info "Install with: brew install lima"
        return 1
    fi
    
    # Check Lima binary location
    lima_path=$(which limactl)
    log_info "Lima binary: $lima_path"
    
    # Check Lima data directory
    lima_data_dir="$HOME/.lima"
    if [[ -d "$lima_data_dir" ]]; then
        log_success "Lima data directory exists: $lima_data_dir"
        
        # Check disk usage
        lima_size=$(du -sh "$lima_data_dir" 2>/dev/null | cut -f1 || echo "unknown")
        log_info "Lima data directory size: $lima_size"
    else
        log_info "Lima data directory not found (will be created on first use)"
    fi
}

# Check Lima VM status
check_lima_vm() {
    log_section "Lima VM Status"
    
    # Check if VM exists
    if limactl list | grep -q "^$LIMA_VM_NAME"; then
        log_success "Lima VM '$LIMA_VM_NAME' exists"
        
        # Get VM details
        vm_status=$(limactl list --format "{{.Status}}" "$LIMA_VM_NAME" 2>/dev/null || echo "unknown")
        vm_arch=$(limactl list --format "{{.Arch}}" "$LIMA_VM_NAME" 2>/dev/null || echo "unknown")
        vm_cpus=$(limactl list --format "{{.CPUs}}" "$LIMA_VM_NAME" 2>/dev/null || echo "unknown")
        vm_memory=$(limactl list --format "{{.Memory}}" "$LIMA_VM_NAME" 2>/dev/null || echo "unknown")
        
        log_info "Status: $vm_status"
        log_info "Architecture: $vm_arch"
        log_info "CPUs: $vm_cpus"
        log_info "Memory: $vm_memory"
        
        if [[ "$vm_status" == "Running" ]]; then
            log_success "VM is running"
            
            # Test SSH connectivity
            if limactl shell "$LIMA_VM_NAME" echo "SSH test" &> /dev/null; then
                log_success "SSH connectivity working"
            else
                log_error "SSH connectivity failed"
            fi
        else
            log_warning "VM is not running (status: $vm_status)"
            log_info "Start with: limactl start $LIMA_VM_NAME"
        fi
    else
        log_error "Lima VM '$LIMA_VM_NAME' not found"
        log_info "Create with: ./scripts/setup-macos.sh"
        return 1
    fi
}

# Check BuildKit daemon
check_buildkit_daemon() {
    log_section "BuildKit Daemon"
    
    if ! limactl list | grep -q "^$LIMA_VM_NAME.*Running"; then
        log_error "Cannot check BuildKit - VM is not running"
        return 1
    fi
    
    # Check if BuildKit service is running
    if limactl shell "$LIMA_VM_NAME" systemctl --user is-active --quiet buildkit.service 2>/dev/null; then
        log_success "BuildKit service is active"
        
        # Get service status
        service_status=$(limactl shell "$LIMA_VM_NAME" systemctl --user show buildkit.service --property=ActiveState --value 2>/dev/null || echo "unknown")
        log_info "Service state: $service_status"
        
        # Check BuildKit socket
        if limactl shell "$LIMA_VM_NAME" test -S /run/user/1000/buildkit/buildkitd.sock 2>/dev/null; then
            log_success "BuildKit socket exists"
        else
            log_error "BuildKit socket not found"
        fi
        
        # Test BuildKit connectivity
        if limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock debug workers &> /dev/null; then
            log_success "BuildKit is responding"
            
            # Get worker information
            workers=$(limactl shell "$LIMA_VM_NAME" /usr/local/bin/buildctl --addr unix:///run/user/1000/buildkit/buildkitd.sock debug workers 2>/dev/null || echo "Failed to get workers")
            log_info "Workers: $workers"
        else
            log_error "BuildKit is not responding"
        fi
    else
        log_error "BuildKit service is not running"
        log_info "Check logs with: limactl shell $LIMA_VM_NAME systemctl --user status buildkit.service"
    fi
}

# Check network connectivity
check_network() {
    log_section "Network Connectivity"
    
    if ! limactl list | grep -q "^$LIMA_VM_NAME.*Running"; then
        log_error "Cannot check network - VM is not running"
        return 1
    fi
    
    # Check port forwarding
    if nc -z 127.0.0.1 $BUILDKIT_PORT 2>/dev/null; then
        log_success "BuildKit port $BUILDKIT_PORT is accessible"
    else
        log_error "BuildKit port $BUILDKIT_PORT is not accessible"
        log_info "This may indicate port forwarding issues"
    fi
    
    # Test internet connectivity from VM
    if limactl shell "$LIMA_VM_NAME" ping -c 1 8.8.8.8 &> /dev/null; then
        log_success "VM has internet connectivity"
    else
        log_warning "VM internet connectivity issues"
    fi
    
    # Test DNS resolution from VM
    if limactl shell "$LIMA_VM_NAME" nslookup docker.io &> /dev/null; then
        log_success "VM DNS resolution working"
    else
        log_warning "VM DNS resolution issues"
    fi
}

# Check mount points
check_mounts() {
    log_section "Mount Points"
    
    if ! limactl list | grep -q "^$LIMA_VM_NAME.*Running"; then
        log_error "Cannot check mounts - VM is not running"
        return 1
    fi
    
    # Check cache mount
    cache_dir="$HOME/.shmocker/cache"
    if [[ -d "$cache_dir" ]]; then
        log_success "Host cache directory exists: $cache_dir"
        
        # Check if mounted in VM
        if limactl shell "$LIMA_VM_NAME" mountpoint -q /var/lib/buildkit/cache 2>/dev/null; then
            log_success "Cache directory is mounted in VM"
        else
            log_warning "Cache directory mount not found in VM"
        fi
    else
        log_warning "Host cache directory not found: $cache_dir"
    fi
    
    # Check build context mount
    build_dir="/tmp/shmocker-builds"
    if [[ -d "$build_dir" ]]; then
        log_success "Host build directory exists: $build_dir"
    else
        log_info "Host build directory will be created as needed: $build_dir"
    fi
}

# Check dependencies
check_dependencies() {
    log_section "Dependencies"
    
    # Check Homebrew
    if command -v brew &> /dev/null; then
        brew_version=$(brew --version | head -n1)
        log_success "Homebrew is installed: $brew_version"
    else
        log_warning "Homebrew not found (recommended for Lima installation)"
    fi
    
    # Check netcat for port testing
    if command -v nc &> /dev/null; then
        log_success "netcat is available for port testing"
    else
        log_warning "netcat not found (used for connectivity testing)"
    fi
    
    # Check if in VM, check container runtime
    if limactl list | grep -q "^$LIMA_VM_NAME.*Running"; then
        if limactl shell "$LIMA_VM_NAME" which runc &> /dev/null; then
            log_success "runc is installed in VM"
        else
            log_error "runc not found in VM"
        fi
        
        if limactl shell "$LIMA_VM_NAME" which containerd &> /dev/null; then
            log_success "containerd is installed in VM"
        else
            log_error "containerd not found in VM"
        fi
    fi
}

# Generate summary report
generate_summary() {
    log_section "Summary"
    
    echo "Diagnostic completed. Key findings:"
    echo
    
    # Check overall health
    if limactl list | grep -q "^$LIMA_VM_NAME.*Running" && \
       limactl shell "$LIMA_VM_NAME" systemctl --user is-active --quiet buildkit.service 2>/dev/null && \
       nc -z 127.0.0.1 $BUILDKIT_PORT 2>/dev/null; then
        log_success "Lima BuildKit appears to be working correctly"
        echo
        echo "You can test with:"
        echo "  ./scripts/buildctl.sh debug workers"
        echo "  shmocker build ./examples"
    else
        log_warning "Lima BuildKit has issues that need attention"
        echo
        echo "Common troubleshooting steps:"
        echo "  1. Run setup script: ./scripts/setup-macos.sh"
        echo "  2. Start VM: limactl start $LIMA_VM_NAME"
        echo "  3. Check logs: ./scripts/lima-vm.sh logs"
        echo "  4. Restart VM: ./scripts/lima-vm.sh restart"
    fi
    
    echo
    echo "For more help:"
    echo "  • Lima documentation: https://lima-vm.io/"
    echo "  • BuildKit documentation: https://github.com/moby/buildkit"
    echo "  • shmocker issues: https://github.com/shmocker/shmocker/issues"
}

# Main diagnostic function
main() {
    echo
    log_info "🔍 Lima BuildKit Diagnostic Tool"
    echo
    
    check_platform
    check_resources
    check_dependencies
    check_lima_installation
    check_lima_vm
    check_buildkit_daemon
    check_network
    check_mounts
    generate_summary
    
    echo
    log_info "Diagnostic complete"
}

# Parse command line arguments
case "${1:-}" in
    -h|--help)
        echo "Lima BuildKit Diagnostic Script"
        echo
        echo "This script performs comprehensive diagnostics of the Lima BuildKit setup."
        echo
        echo "Usage: $0 [OPTION]"
        echo
        echo "Options:"
        echo "  -h, --help     Show this help message"
        echo
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
# macOS Setup Guide for shmocker

This guide helps you set up shmocker to build real container images on macOS using virtualization and BuildKit.

## Overview

On macOS, shmocker can use virtualization to run BuildKit in a Linux virtual machine. We provide two options:

### Option 1: Colima (Recommended)

[Colima](https://github.com/abiosoft/colima) is a simpler alternative specifically designed for container workloads:

- **Easier setup and management**
- **Optimized for container builds**  
- **Better Docker compatibility**
- **Automatic resource management**

**Setup:** See [macOS Setup with Colima](MACOS_SETUP_COLIMA.md)

### Option 2: Lima (Advanced)

[Lima](https://lima-vm.io/) provides maximum flexibility and customization:

- **Fine-grained VM control**
- **Custom configuration options**
- **General purpose VM usage**

**Setup:** Continue with this guide for Lima setup

---

## Lima Setup Guide

This section covers setting up shmocker with Lima for users who need advanced VM customization.

## Lima Quick Start

1. **Run the Lima setup script:**
   ```bash
   ./scripts/setup-macos.sh
   ```

2. **Build your first image:**
   ```bash
   shmocker build ./examples
   ```

That's it! The setup script handles everything automatically.

**Note:** For easier setup, consider using [Colima instead](MACOS_SETUP_COLIMA.md).

## What Gets Installed

The setup process installs and configures:

- **Lima VM runtime** (via Homebrew)
- **Ubuntu 22.04 VM** with 4 CPUs, 8GB RAM, 64GB disk
- **BuildKit daemon** running in rootless mode
- **Multi-platform build support** (amd64, arm64, arm/v7, etc.)
- **Persistent cache** mounted from `~/.shmocker/cache`
- **Management scripts** for VM lifecycle operations

## Requirements

### System Requirements

- **macOS 10.15+** (macOS 11+ recommended)
- **4+ CPU cores** (for good build performance)
- **8+ GB RAM** (4GB minimum, VM uses up to 8GB)
- **80+ GB free disk space** (VM image + build cache)
- **Internet connection** (for VM image download)

### Architecture Support

- **Apple Silicon (M1/M2/M3)**: Fully supported
- **Intel Macs**: Fully supported

## Detailed Setup

### 1. Prerequisites Check

Run the diagnostic script to check your system:

```bash
./scripts/diagnose-lima.sh
```

This will verify:
- macOS version compatibility
- Available system resources
- Required dependencies
- Lima installation status

### 2. Automatic Setup

The setup script handles everything:

```bash
./scripts/setup-macos.sh
```

**What it does:**
- Installs Homebrew (if needed)
- Installs Lima VM runtime
- Creates and configures the BuildKit VM
- Sets up BuildKit daemon with rootless containers
- Creates management scripts
- Configures environment variables

**First run takes 10-15 minutes** due to:
- VM image download (~2GB)
- Ubuntu package installation
- BuildKit compilation and setup

### 3. Manual Setup (Advanced)

If you prefer manual setup or need customization:

#### Install Lima
```bash
brew install lima
```

#### Create VM
```bash
limactl start --name=shmocker-buildkit lima/buildkit.yaml
```

#### Verify Setup
```bash
./scripts/buildctl.sh debug workers
```

## Usage

### Building Images

Standard Docker-compatible syntax:

```bash
# Build current directory
shmocker build .

# Build with tag
shmocker build -t myapp:latest .

# Multi-platform build
shmocker build --platform linux/amd64,linux/arm64 -t myapp:latest .

# Build with custom Dockerfile
shmocker build -f Dockerfile.custom .
```

### VM Management

Use the management script for VM operations:

```bash
# Check VM status
./scripts/lima-vm.sh status

# Start VM
./scripts/lima-vm.sh start

# Stop VM (saves resources)
./scripts/lima-vm.sh stop

# Restart VM
./scripts/lima-vm.sh restart

# Open shell in VM
./scripts/lima-vm.sh shell

# View BuildKit logs
./scripts/lima-vm.sh logs
```

### Direct BuildKit Access

Use buildctl directly through the wrapper:

```bash
# List workers
./scripts/buildctl.sh debug workers

# Prune build cache
./scripts/buildctl.sh prune

# Show disk usage
./scripts/buildctl.sh du
```

## Configuration

### Environment Variables

Source the Lima environment:

```bash
source .env.lima
```

Key variables:
- `BUILDKIT_HOST=tcp://127.0.0.1:1234`
- `LIMA_VM_NAME=shmocker-buildkit`
- `SHMOCKER_BACKEND=lima`

### Lima VM Configuration

Edit `lima/buildkit.yaml` to customize:

```yaml
# Hardware resources
cpus: 4
memory: "8GiB"
disk: "64GiB"

# Port forwarding
portForwards:
  - guestPort: 1234
    hostPort: 1234
    protocol: tcp
```

### BuildKit Configuration

BuildKit daemon config in VM at `~/.config/buildkit/buildkitd.toml`:

```toml
[grpc]
address = ["tcp://0.0.0.0:1234"]

[worker.oci]
enabled = true
rootless = true
platforms = ["linux/amd64", "linux/arm64", ...]
```

## Troubleshooting

### Common Issues

#### 1. VM Won't Start

**Symptoms:** `limactl start` fails or times out

**Solutions:**
```bash
# Check system resources
./scripts/diagnose-lima.sh

# Try recreating VM
limactl delete shmocker-buildkit
./scripts/setup-macos.sh
```

#### 2. BuildKit Not Responding

**Symptoms:** Build fails with connection errors

**Solutions:**
```bash
# Check BuildKit status
./scripts/lima-vm.sh logs

# Restart BuildKit service
./scripts/lima-vm.sh shell
systemctl --user restart buildkit.service

# Or restart entire VM
./scripts/lima-vm.sh restart
```

#### 3. Port Not Accessible

**Symptoms:** `tcp://127.0.0.1:1234` connection refused

**Solutions:**
```bash
# Check port forwarding
lsof -i :1234

# Restart VM (resets port forwarding)
./scripts/lima-vm.sh restart

# Check Lima configuration
limactl show-ssh shmocker-buildkit
```

#### 4. Build Performance Issues

**Symptoms:** Slow builds or timeouts

**Solutions:**
```bash
# Increase VM resources in lima/buildkit.yaml
cpus: 6
memory: "12GiB"

# Clear build cache
./scripts/buildctl.sh prune

# Check host resources
./scripts/diagnose-lima.sh
```

### Diagnostic Tools

#### Full System Check
```bash
./scripts/diagnose-lima.sh
```

#### VM Status Check
```bash
./scripts/lima-vm.sh status
limactl list
```

#### BuildKit Health Check
```bash
./scripts/buildctl.sh debug workers
./scripts/lima-vm.sh logs
```

#### Network Connectivity
```bash
# Test port access
nc -z 127.0.0.1 1234

# Test from VM
./scripts/lima-vm.sh shell
ping 8.8.8.8
```

### Log Locations

- **Lima logs:** `~/.lima/shmocker-buildkit/serial.log`
- **BuildKit logs:** `./scripts/lima-vm.sh logs`
- **VM system logs:** `./scripts/lima-vm.sh shell` → `sudo journalctl`

## Advanced Usage

### Custom VM Configuration

1. Edit `lima/buildkit.yaml`
2. Recreate VM:
   ```bash
   limactl delete shmocker-buildkit
   limactl start --name=shmocker-buildkit lima/buildkit.yaml
   ```

### Multiple VMs

Create additional VMs for different purposes:

```bash
# Copy and modify configuration
cp lima/buildkit.yaml lima/buildkit-experimental.yaml

# Start with different name
limactl start --name=shmocker-experimental lima/buildkit-experimental.yaml
```

### Cache Management

Control build cache location and size:

```bash
# Check cache usage
./scripts/buildctl.sh du

# Prune old cache
./scripts/buildctl.sh prune --keep-storage 5gb

# Manual cache location
export SHMOCKER_CACHE_DIR=/path/to/cache
```

### Registry Integration

Configure registry authentication:

```bash
# Docker Hub
export DOCKER_USERNAME=your-username
export DOCKER_PASSWORD=your-password

# Custom registry
export SHMOCKER_REGISTRY_URL=your-registry.com
export SHMOCKER_REGISTRY_USERNAME=username
export SHMOCKER_REGISTRY_PASSWORD=password
```

## Performance Tips

### 1. Resource Allocation

Optimal settings for different Mac configurations:

**8GB Mac:**
```yaml
cpus: 2
memory: "4GiB"
```

**16GB Mac:**
```yaml
cpus: 4
memory: "8GiB"
```

**32GB+ Mac:**
```yaml
cpus: 6
memory: "12GiB"
```

### 2. Cache Strategy

- **Keep VM running** during active development
- **Use registry cache** for CI/CD builds
- **Prune cache** regularly to free disk space

### 3. Network Optimization

- **Use local registry** for private images
- **Enable registry mirrors** in BuildKit config
- **Configure HTTP proxy** if behind corporate firewall

## Security Considerations

### VM Isolation

- VM runs in user space (no root required)
- Network access controlled by Lima
- File system access limited to mounted directories

### BuildKit Security

- Rootless container runtime
- No privileged operations
- Secure by default configuration

### Host Protection

- VM cannot access host system beyond mounts
- Port forwarding limited to localhost
- SSH keys isolated per-VM

## Migration and Backup

### Backup VM State

```bash
# Stop VM
./scripts/lima-vm.sh stop

# Backup Lima directory
tar -czf shmocker-buildkit-backup.tar.gz ~/.lima/shmocker-buildkit/
```

### Restore VM State

```bash
# Delete existing VM
limactl delete shmocker-buildkit

# Restore backup
tar -xzf shmocker-buildkit-backup.tar.gz -C ~/

# Start VM
./scripts/lima-vm.sh start
```

### Migration to New Mac

1. Backup cache directory: `~/.shmocker/cache`
2. Run setup on new Mac: `./scripts/setup-macos.sh`
3. Restore cache directory
4. Import container images if needed

## Getting Help

### Community Resources

- **Lima Project:** https://lima-vm.io/
- **BuildKit Docs:** https://github.com/moby/buildkit
- **shmocker Issues:** https://github.com/shmocker/shmocker/issues

### Diagnostic Information

When reporting issues, include:

```bash
# System info
./scripts/diagnose-lima.sh > diagnostic-output.txt

# Lima version
limactl --version

# VM configuration
limactl show-ssh shmocker-buildkit

# Recent logs
./scripts/lima-vm.sh logs --since "1 hour ago"
```

### Common Support Commands

```bash
# Full diagnostic
./scripts/diagnose-lima.sh

# Reset everything
limactl delete shmocker-buildkit
rm -rf ~/.shmocker
./scripts/setup-macos.sh

# Emergency stop
limactl stop --force shmocker-buildkit
```

## FAQ

### Q: Why Lima instead of Docker Desktop?

**A:** Lima provides:
- Open source and free
- Better performance on Apple Silicon
- True rootless operation
- No licensing restrictions
- Full control over VM configuration

### Q: Can I use both Lima and Docker Desktop?

**A:** Yes, they don't conflict. Use different ports:
- Lima BuildKit: `tcp://127.0.0.1:1234`
- Docker Desktop: `/var/run/docker.sock`

### Q: How do I update Lima or BuildKit?

**A:** Update process:
```bash
# Update Lima
brew upgrade lima

# Update BuildKit (recreate VM)
limactl delete shmocker-buildkit
./scripts/setup-macos.sh
```

### Q: Can I run multiple build jobs in parallel?

**A:** Yes, BuildKit supports concurrent builds automatically. Monitor with:
```bash
./scripts/buildctl.sh debug workers
```

### Q: How do I clean up completely?

**A:** Full cleanup:
```bash
# Stop and delete VM
limactl stop shmocker-buildkit
limactl delete shmocker-buildkit

# Remove data
rm -rf ~/.lima/shmocker-buildkit
rm -rf ~/.shmocker

# Remove Lima (optional)
brew uninstall lima
```

---

For more help, run `./scripts/diagnose-lima.sh` or check the [troubleshooting guide](troubleshooting.md).
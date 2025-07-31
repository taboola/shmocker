# macOS Setup Guide for shmocker with Colima

This guide helps you set up shmocker to build real container images on macOS using Colima and BuildKit.

## Overview

On macOS, shmocker can use [Colima](https://github.com/abiosoft/colima) to run BuildKit in a Linux virtual machine. Colima is a simpler alternative to Lima, specifically designed for container workloads, providing easier setup and better performance.

## Quick Start

1. **Run the setup script:**
   ```bash
   ./scripts/setup-macos-colima.sh
   ```

2. **Build your first image:**
   ```bash
   shmocker build ./examples
   ```

That's it! The setup script handles everything automatically.

## What Gets Installed

The setup process installs and configures:

- **Colima VM runtime** (via Homebrew)
- **Docker CLI and BuildKit client** (for container operations)
- **Linux VM** with optimized resource allocation
- **BuildKit daemon** running in rootless mode
- **Multi-platform build support** (amd64, arm64, arm/v7, etc.)
- **Persistent cache** mounted from `~/.shmocker/cache`
- **Management scripts** for profile lifecycle operations

## Requirements

### System Requirements

- **macOS 11.0+** (macOS 12+ recommended for Apple Silicon)
- **4+ CPU cores** (for good build performance)
- **8+ GB RAM** (4GB minimum, VM uses up to 50% of available RAM)
- **50+ GB free disk space** (VM image + build cache)
- **Internet connection** (for VM image download)

### Architecture Support

- **Apple Silicon (M1/M2/M3)**: Fully supported with native performance
- **Intel Macs**: Fully supported

## Detailed Setup

### 1. Prerequisites Check

Run the diagnostic to check your system:

```bash
# Check system compatibility
./scripts/diagnose-colima.sh  # (if available)

# Or check manually
sw_vers -productVersion  # Should be 11.0+
sysctl -n hw.ncpu        # CPU count
sysctl -n hw.memsize | awk '{print $1/1024/1024/1024 "GB"}'  # RAM
df -h /                  # Disk space
```

### 2. Automatic Setup

The setup script handles everything:

```bash
./scripts/setup-macos-colima.sh
```

**What it does:**
- Installs Homebrew (if needed)
- Installs Colima, Docker CLI, and BuildKit client
- Creates and configures the Colima profile
- Sets up BuildKit daemon with rootless containers
- Creates management scripts
- Configures environment variables

**First run takes 5-10 minutes** due to:
- VM image download (~1GB)
- Package installation and configuration
- BuildKit daemon setup and verification

### 3. Manual Setup (Advanced)

If you prefer manual setup or need customization:

#### Install Colima
```bash
brew install colima docker buildkit
```

#### Create Profile
```bash
colima start --profile shmocker --config colima/colima.yaml
```

#### Verify Setup
```bash
./scripts/colima-buildctl.sh debug workers
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

### Profile Management

Use the management script for profile operations:

```bash
# Check profile status
./scripts/colima-vm.sh status

# Start profile
./scripts/colima-vm.sh start

# Stop profile (saves resources)
./scripts/colima-vm.sh stop

# Restart profile
./scripts/colima-vm.sh restart

# List all profiles
./scripts/colima-vm.sh list

# Open shell in VM
./scripts/colima-vm.sh shell

# View BuildKit logs
./scripts/colima-vm.sh logs

# Delete profile (destructive)
./scripts/colima-vm.sh delete
```

### Direct BuildKit Access

Use buildctl directly through the wrapper:

```bash
# List workers
./scripts/colima-buildctl.sh debug workers

# Prune build cache
./scripts/colima-buildctl.sh prune

# Show disk usage
./scripts/colima-buildctl.sh du

# Check BuildKit version
./scripts/colima-buildctl.sh version
```

## Configuration

### Environment Variables

Source the Colima environment:

```bash
source .env.colima
```

Key variables:
- `COLIMA_PROFILE=shmocker`
- `BUILDKIT_HOST=unix:///run/user/1000/buildkit/buildkitd.sock`
- `SHMOCKER_BACKEND=colima`
- `DOCKER_HOST=unix://~/.colima/shmocker/docker.sock`

### Colima Profile Configuration

Edit `colima/colima.yaml` to customize:

```yaml
# Hardware resources (auto-configured by setup script)
cpu: 4
memory: 8
disk: 64

# VM type
vmType: qemu

# Container runtime
runtime: containerd

# Network configuration
network:
  address: true

# Mount points
mounts:
  - location: ~/.shmocker/cache
    mountPoint: /var/lib/buildkit/cache
    writable: true
```

### BuildKit Configuration

BuildKit daemon config in VM at `~/.config/buildkit/buildkitd.toml`:

```toml
[grpc]
address = ["unix:///run/user/1000/buildkit/buildkitd.sock"]

[worker.oci]
enabled = true
rootless = true
platforms = ["linux/amd64", "linux/arm64", ...]
```

## Troubleshooting

### Common Issues

#### 1. Profile Won't Start

**Symptoms:** `colima start` fails or times out

**Solutions:**
```bash
# Check system resources
./scripts/diagnose-colima.sh  # (if available)

# Try recreating profile
colima delete --profile shmocker
./scripts/setup-macos-colima.sh

# Check Colima logs
colima logs --profile shmocker
```

#### 2. BuildKit Not Responding

**Symptoms:** Build fails with connection errors

**Solutions:**
```bash
# Check BuildKit status
./scripts/colima-vm.sh logs

# Restart BuildKit service
colima ssh --profile shmocker -- systemctl --user restart buildkit.service

# Or restart entire profile
./scripts/colima-vm.sh restart
```

#### 3. Profile Not Found

**Symptoms:** `profile 'shmocker' not found`

**Solutions:**
```bash
# List existing profiles
colima list

# Recreate profile
./scripts/setup-macos-colima.sh

# Check profile status
colima status --profile shmocker
```

#### 4. Build Performance Issues

**Symptoms:** Slow builds or timeouts

**Solutions:**
```bash
# Increase profile resources in colima.yaml
cpu: 6
memory: 12

# Recreate profile with new resources
colima delete --profile shmocker
colima start --profile shmocker --config colima/colima.yaml

# Clear build cache
./scripts/colima-buildctl.sh prune

# Check host resources
./scripts/diagnose-colima.sh  # (if available)
```

### Diagnostic Tools

#### Full System Check
```bash
# Check Colima installation
colima version

# Check profile status
colima status --profile shmocker

# List all profiles
colima list
```

#### BuildKit Health Check
```bash
./scripts/colima-buildctl.sh debug workers
./scripts/colima-vm.sh logs
```

#### Network Connectivity
```bash
# Test from VM
colima ssh --profile shmocker -- ping 8.8.8.8

# Check BuildKit socket
colima ssh --profile shmocker -- ls -la /run/user/1000/buildkit/
```

### Log Locations

- **Colima logs:** `colima logs --profile shmocker`
- **BuildKit logs:** `./scripts/colima-vm.sh logs`
- **VM system logs:** `colima ssh --profile shmocker -- sudo journalctl`

## Advanced Usage

### Custom Profile Configuration

1. Edit `colima/colima.yaml`
2. Recreate profile:
   ```bash
   colima delete --profile shmocker
   colima start --profile shmocker --config colima/colima.yaml
   ```

### Multiple Profiles

Create additional profiles for different purposes:

```bash
# Copy and modify configuration
cp colima/colima.yaml colima/colima-experimental.yaml

# Start with different profile name
colima start --profile shmocker-experimental --config colima/colima-experimental.yaml
```

### Cache Management

Control build cache location and size:

```bash
# Check cache usage
./scripts/colima-buildctl.sh du

# Prune old cache
./scripts/colima-buildctl.sh prune --keep-storage 5gb

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
cpu: 2
memory: 4
```

**16GB Mac:**
```yaml
cpu: 4
memory: 8
```

**32GB+ Mac:**
```yaml
cpu: 6
memory: 12
```

### 2. Cache Strategy

- **Keep profile running** during active development
- **Use registry cache** for CI/CD builds
- **Prune cache** regularly to free disk space

### 3. Network Optimization

- **Use local registry** for private images
- **Enable registry mirrors** in BuildKit config
- **Configure HTTP proxy** if behind corporate firewall

## Security Considerations

### VM Isolation

- Profile runs in user space (no root required)
- Network access controlled by Colima
- File system access limited to mounted directories

### BuildKit Security

- Rootless container runtime
- No privileged operations
- Secure by default configuration

### Host Protection

- VM cannot access host system beyond mounts
- SSH keys isolated per-profile
- Network isolation from host system

## Migration and Backup

### Backup Profile State

```bash
# Stop profile
colima stop --profile shmocker

# Backup profile directory
tar -czf shmocker-profile-backup.tar.gz ~/.colima/shmocker/
```

### Restore Profile State

```bash
# Delete existing profile
colima delete --profile shmocker

# Restore backup
tar -xzf shmocker-profile-backup.tar.gz -C ~/

# Start profile
colima start --profile shmocker
```

### Migration to New Mac

1. Backup cache directory: `~/.shmocker/cache`
2. Run setup on new Mac: `./scripts/setup-macos-colima.sh`
3. Restore cache directory
4. Import container images if needed

## Colima vs Lima Comparison

### Advantages of Colima

| Feature | Colima | Lima |
|---------|--------|------|
| **Setup Complexity** | Simple, one command | Complex configuration |
| **Container Focus** | Optimized for containers | General purpose VM |
| **Docker Compatibility** | Built-in Docker support | Manual configuration |
| **Resource Management** | Automatic optimization | Manual tuning |
| **Profile Management** | Built-in profiles | Custom VM management |
| **BuildKit Integration** | Native support | Custom setup required |
| **Maintenance** | Minimal | Regular maintenance |

### When to Use Each

**Use Colima when:**
- You want simple setup and maintenance
- Container builds are your primary use case
- You prefer automatic resource management
- You need good Docker compatibility

**Use Lima when:**
- You need maximum customization
- You require specific VM configurations
- You want fine-grained control
- You use the VM for non-container workloads

## Getting Help

### Community Resources

- **Colima Project:** https://github.com/abiosoft/colima
- **BuildKit Docs:** https://github.com/moby/buildkit
- **shmocker Issues:** https://github.com/shmocker/shmocker/issues

### Diagnostic Information

When reporting issues, include:

```bash
# System info
sw_vers
sysctl -n hw.ncpu hw.memsize

# Colima version and status
colima version
colima status --profile shmocker

# BuildKit status
./scripts/colima-buildctl.sh debug workers

# Recent logs
./scripts/colima-vm.sh logs
```

### Common Support Commands

```bash
# Full diagnostic
colima version
colima list
./scripts/colima-vm.sh status

# Reset everything
colima delete --profile shmocker
rm -rf ~/.shmocker
./scripts/setup-macos-colima.sh

# Emergency stop
colima stop --profile shmocker --force
```

## FAQ

### Q: Why Colima instead of Docker Desktop?

**A:** Colima provides:
- Open source and free
- Better performance on Apple Silicon
- True rootless operation
- No licensing restrictions
- Full control over VM configuration
- Simpler than raw Lima

### Q: Can I use both Colima and Docker Desktop?

**A:** Yes, they don't conflict. Use different sockets:
- Colima: `unix://~/.colima/shmocker/docker.sock`
- Docker Desktop: `/var/run/docker.sock`

### Q: How do I update Colima or BuildKit?

**A:** Update process:
```bash
# Update Colima
brew upgrade colima

# Update BuildKit (recreate profile)
colima delete --profile shmocker
./scripts/setup-macos-colima.sh
```

### Q: Can I run multiple build jobs in parallel?

**A:** Yes, BuildKit supports concurrent builds automatically. Monitor with:
```bash
./scripts/colima-buildctl.sh debug workers
```

### Q: How do I clean up completely?

**A:** Full cleanup:
```bash
# Stop and delete profile
colima stop --profile shmocker
colima delete --profile shmocker

# Remove data
rm -rf ~/.colima/shmocker
rm -rf ~/.shmocker

# Remove Colima (optional)
brew uninstall colima
```

---

For more help, run `./scripts/colima-vm.sh status` or check the [troubleshooting guide](TROUBLESHOOTING.md).
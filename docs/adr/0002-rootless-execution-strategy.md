# ADR-0002: Rootless Execution Strategy

## Status
Accepted

## Context

One of the core requirements for shmocker is rootless operation. This capability is essential for:

1. **Security**: Running without root privileges reduces attack surface
2. **CI/CD Integration**: Many CI systems restrict root access
3. **Kubernetes Compatibility**: Pod Security Standards discourage privileged containers
4. **Developer Experience**: Eliminates need for sudo/root access on development machines

Several approaches exist for rootless container image building:

### Option 1: User Namespaces + overlayfs
- **Pros**: Native kernel support, good performance, compatible with most filesystems
- **Cons**: Requires kernel support (Linux 3.8+), may need sysctl configuration

### Option 2: FUSE-based filesystems (fuse-overlayfs)
- **Pros**: Works without special kernel configuration, broader compatibility
- **Cons**: FUSE overhead, potential performance impact, requires FUSE support

### Option 3: VFS snapshotter
- **Pros**: No special requirements, always works
- **Cons**: Poor performance, large storage overhead, slow layer operations

### Option 4: Hybrid approach with fallback
- **Pros**: Best performance when possible, fallback for compatibility
- **Cons**: More complex implementation, multiple code paths

## Decision

We will implement a **hybrid approach with automatic fallback**:

1. **Primary**: User namespaces + overlayfs snapshotter
2. **Fallback**: FUSE-overlayfs snapshotter  
3. **Last resort**: VFS snapshotter (development/testing only)

## Rationale

### Primary Strategy: User Namespaces + overlayfs

User namespaces provide the best performance and are well-supported in modern Linux kernels:

```go
// Rootless worker configuration
type RootlessWorkerConfig struct {
    // Use overlayfs snapshotter with user namespaces
    Snapshotter:     "overlayfs"
    UserNamespace:   true
    NetworkMode:     "host"  // or "none" for security
    
    // Security constraints
    NoNewPrivileges: true
    SeccompProfile:  "default"
    AppArmorProfile: "default"
}
```

### Fallback Strategy: FUSE-overlayfs

When user namespaces are unavailable or overlayfs fails:

```go
type FUSEWorkerConfig struct {
    Snapshotter:     "fuse-overlayfs" 
    UserNamespace:   false
    MountOptions:    []string{"userxattr"}
    
    // FUSE-specific optimizations
    FUSEOptions: map[string]string{
        "squash_to_uid": "0",
        "squash_to_gid": "0",
    }
}
```

### Detection and Selection Logic

```go
type SnapshotterSelector interface {
    SelectBest(ctx context.Context) (SnapshotterType, error)
    Validate(snapshotterType SnapshotterType) error
    GetFallbacks() []SnapshotterType
}

// Selection priority
const (
    SnapshotterOverlayFS SnapshotterType = "overlayfs"
    SnapshotterFUSE     SnapshotterType = "fuse-overlayfs"  
    SnapshotterVFS      SnapshotterType = "vfs"
)
```

## Implementation Details

### Worker Initialization

```go
func NewRootlessWorker(ctx context.Context) (Worker, error) {
    selector := NewSnapshotterSelector()
    
    // Try snapshotter options in priority order
    for _, snapshotter := range selector.GetFallbacks() {
        worker, err := createWorker(ctx, snapshotter)
        if err == nil {
            return worker, nil
        }
        log.Warnf("Failed to create %s worker: %v", snapshotter, err)
    }
    
    return nil, errors.New("no suitable snapshotter available")
}
```

### Runtime Environment Detection

```go
type RuntimeDetector interface {
    IsUserNamespaceSupported() bool
    IsOverlayFSSupported() bool  
    IsFUSESupported() bool
    GetKernelVersion() (*KernelVersion, error)
    CheckRequirements() (*RequirementCheck, error)
}

type RequirementCheck struct {
    UserNS      bool   `json:"user_ns"`
    OverlayFS   bool   `json:"overlay_fs"`
    FUSE        bool   `json:"fuse"`
    Warnings    []string `json:"warnings"`
    Suggestions []string `json:"suggestions"`
}
```

### Security Considerations

#### User Namespace Configuration
```go
type UserNamespaceConfig struct {
    // UID/GID mapping for the build process
    UIDMap: []IDMap{
        {ContainerID: 0, HostID: 1000, Size: 65536},
    }
    GIDMap: []IDMap{
        {ContainerID: 0, HostID: 1000, Size: 65536},
    }
    
    // Security constraints
    NoNewPrivileges: true
    DropCapabilities: []string{
        "CAP_SYS_ADMIN",
        "CAP_NET_ADMIN", 
        "CAP_SYS_MODULE",
    }
}
```

#### Network Isolation
```go
type NetworkConfig struct {
    Mode: NetworkMode // "host", "none", "bridge"
    
    // For security-sensitive environments
    DisableNetworking: bool
    AllowedDomains:   []string  // DNS allowlist
    ProxySettings:    *ProxyConfig
}
```

## Consequences

### Positive

1. **Broad Compatibility**: Works across different environments
2. **Performance**: Optimal performance when system supports it
3. **Security**: Maintains security posture without privileges
4. **Graceful Degradation**: Automatic fallback ensures functionality

### Negative

1. **Complexity**: Multiple execution paths to maintain
2. **Testing**: Need to test all snapshotter combinations
3. **Debugging**: Different behavior across environments
4. **Feature Parity**: Some features may not work in all modes

### Mitigation Strategies

1. **Environment Detection**: Clear error messages about capabilities
2. **Testing Matrix**: Comprehensive testing across environments
3. **Documentation**: Clear setup instructions for different environments
4. **Monitoring**: Telemetry to understand real-world usage patterns

## Environment Compatibility Matrix

| Environment | overlayfs | fuse-overlayfs | VFS | Notes |
|-------------|-----------|----------------|-----|-------|
| Ubuntu 20.04+ | ✅ | ✅ | ✅ | Full support |
| RHEL 8+ | ✅ | ✅ | ✅ | May need user_namespace.enable=1 |
| Alpine Linux | ✅ | ✅ | ✅ | Good performance |
| GitHub Actions | ⚠️ | ✅ | ✅ | Ubuntu runners support overlayfs |
| GitLab CI | ⚠️ | ✅ | ✅ | Depends on runner configuration |
| CircleCI | ❌ | ✅ | ✅ | Limited user namespace support |
| Kubernetes | ⚠️ | ✅ | ✅ | Depends on PSP/PSS configuration |

## Performance Characteristics

| Snapshotter | Build Performance | Storage Efficiency | Memory Usage |
|-------------|------------------|-------------------|--------------|
| overlayfs | Excellent (1.0x) | Excellent | Low |
| fuse-overlayfs | Good (1.2-1.5x) | Good | Medium |
| VFS | Poor (2-5x) | Poor | High |

## Implementation Phases

### Phase 1: Core Rootless Support
- User namespace detection and configuration
- overlayfs snapshotter integration
- Basic rootless worker setup

### Phase 2: Fallback Implementation  
- FUSE-overlayfs integration
- Automatic detection and fallback logic
- Environment compatibility testing

### Phase 3: Optimization
- Performance tuning for each snapshotter
- Memory usage optimization
- Advanced security hardening

### Phase 4: Monitoring and Telemetry
- Runtime snapshotter reporting
- Performance metrics collection
- Error pattern analysis

## Testing Strategy

```go
func TestRootlessExecution(t *testing.T) {
    testCases := []struct {
        name         string
        snapshotter  SnapshotterType
        environment  string
        dockerfile   string
        expectError  bool
    }{
        {"overlayfs-simple", SnapshotterOverlayFS, "ubuntu", simpleDockerfile, false},
        {"fuse-multistage", SnapshotterFUSE, "alpine", multistageDockerfile, false},
        {"vfs-fallback", SnapshotterVFS, "minimal", complexDockerfile, false},
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## References

- [User Namespaces Documentation](https://man7.org/linux/man-pages/man7/user_namespaces.7.html)
- [BuildKit Rootless Mode](https://github.com/moby/buildkit/blob/master/docs/rootless.md)
- [fuse-overlayfs Project](https://github.com/containers/fuse-overlayfs)
- [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)

## Related ADRs

- [ADR-0001: Embed BuildKit as Library](./0001-embed-buildkit-as-library.md)
- [ADR-0003: Cache Architecture](./0003-cache-architecture.md)
- [ADR-0005: Security Model](./0005-security-model.md)
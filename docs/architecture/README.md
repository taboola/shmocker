# Shmocker Architecture Overview

This document provides a comprehensive overview of the shmocker architecture, designed as a rootless Docker image builder that embeds BuildKit as a library for in-process execution.

## Architecture Principles

### Core Design Principles

1. **Rootless by Default**: No elevated privileges required for operation
2. **Single Binary Distribution**: Self-contained executable with no external dependencies
3. **BuildKit Integration**: Leverage proven BuildKit technology as an embedded library
4. **Security First**: Comprehensive security model with supply chain transparency
5. **Performance Focused**: Match or exceed Docker Buildx performance
6. **Clean Interfaces**: Testable, mockable interfaces throughout the system

### System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Shmocker Application                       │
├─────────────────────────────────────────────────────────────────┤
│  CLI Layer (cmd/shmocker)                                      │
│  ├─ Command Processing & Flag Parsing                          │
│  ├─ Configuration Management                                    │
│  └─ User Interface & Progress Reporting                        │
├─────────────────────────────────────────────────────────────────┤
│  Workflow Orchestration (internal/workflow)                    │
│  ├─ Build Workflow Management                                  │
│  ├─ Stage Execution & Dependencies                             │
│  ├─ Error Handling & Recovery                                  │
│  └─ Progress Tracking & Reporting                              │
├─────────────────────────────────────────────────────────────────┤
│  Core Services Layer                                           │
│  ├─ Builder Service (pkg/builder)        │                     │
│  ├─ Dockerfile Parser (pkg/dockerfile)   │ Business Logic     │
│  ├─ Registry Client (pkg/registry)       │                     │
│  ├─ SBOM Generator (pkg/sbom)           │                     │
│  ├─ Image Signing (pkg/signing)         │                     │
│  └─ Cache Manager (pkg/cache)           │                     │
├─────────────────────────────────────────────────────────────────┤
│  Embedded BuildKit Layer                                       │
│  ├─ BuildKit Controller (Control Client)     │                 │
│  ├─ Dockerfile Frontend (LLB Generation)     │ In-Process      │
│  ├─ Build Solver (DAG Resolution)           │ Execution       │
│  ├─ OCI Worker (Rootless)                   │                 │
│  ├─ Snapshotter (overlayfs/fuse-overlayfs)  │                 │
│  └─ Content Store (CAS)                     │                 │
├─────────────────────────────────────────────────────────────────┤
│  Infrastructure Layer                                          │
│  ├─ Error Management (internal/errors)                         │
│  ├─ Configuration (internal/config)                            │
│  ├─ Security Framework                                         │
│  └─ Logging & Metrics                                          │
└─────────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Builder Interface (`pkg/builder/interfaces.go`)

The central orchestration interface that coordinates the entire build process:

```go
type Builder interface {
    Build(ctx context.Context, req *BuildRequest) (*BuildResult, error)
    BuildWithProgress(ctx context.Context, req *BuildRequest, progress chan<- *ProgressEvent) (*BuildResult, error)
    Close() error
}
```

**Key Features:**
- BuildKit integration for high-performance builds
- Multi-platform support
- Comprehensive build result metadata
- Progress reporting for real-time feedback

### 2. Dockerfile Parser (`pkg/dockerfile/interfaces.go`)

Advanced Dockerfile parsing with full AST representation:

```go
type Parser interface {
    Parse(reader io.Reader) (*AST, error)
    ParseFile(path string) (*AST, error)
    Validate(ast *AST) error
}
```

**Key Features:**
- Complete Dockerfile grammar support (Docker 24+ syntax)
- Rich AST with source location information
- LLB conversion for BuildKit integration
- Validation and error reporting

### 3. Registry Client (`pkg/registry/interfaces.go`)

OCI-compliant registry operations with multi-tier support:

```go
type Client interface {
    Push(ctx context.Context, req *PushRequest) (*PushResult, error)
    Pull(ctx context.Context, req *PullRequest) (*PullResult, error)
    GetManifest(ctx context.Context, ref string) (*Manifest, error)
    // ... additional operations
}
```

**Key Features:**
- OCI Distribution API v1.1 compliance
- Multi-arch manifest support
- Authentication provider abstraction
- Progress reporting and retry logic

### 4. SBOM Generator (`pkg/sbom/interfaces.go`)

Software Bill of Materials generation for supply chain transparency:

```go
type Generator interface {
    Generate(ctx context.Context, req *GenerateRequest) (*SBOM, error)
    GenerateFromFilesystem(ctx context.Context, path string, opts *GenerateOptions) (*SBOM, error)
    Merge(ctx context.Context, sboms []*SBOM) (*SBOM, error)
}
```

**Key Features:**
- Multiple SBOM formats (SPDX, CycloneDX, Syft)
- Package scanner abstraction
- Vulnerability integration
- Attestation generation

### 5. Image Signing (`pkg/signing/interfaces.go`)

Cryptographic signing and verification using Sigstore/Cosign:

```go
type Signer interface {
    Sign(ctx context.Context, req *SignRequest) (*SignResult, error)
    GenerateKeyPair(ctx context.Context, opts *KeyGenOptions) (*KeyPair, error)
    GetPublicKey(ctx context.Context, keyRef string) (crypto.PublicKey, error)
}
```

**Key Features:**
- Cosign-compatible signing
- Multiple key providers (file, HSM, KMS)
- In-toto attestation support
- Policy-based verification

### 6. Cache Management (`pkg/cache/interfaces.go`)

Multi-tier caching system for optimal performance:

```go
type Manager interface {
    Get(ctx context.Context, key string) (*CacheEntry, error)
    Put(ctx context.Context, key string, data io.Reader, metadata *CacheMetadata) error
    Export(ctx context.Context, req *ExportRequest) (*ExportResult, error)
    Import(ctx context.Context, req *ImportRequest) (*ImportResult, error)
}
```

**Key Features:**
- Content-addressable storage
- Multi-tier architecture (local, registry, external)
- Import/export capabilities
- Intelligent pruning strategies

## Workflow Orchestration

### Workflow Engine (`internal/workflow/interfaces.go`)

The workflow engine coordinates the complete build process through discrete stages:

```go
type Orchestrator interface {
    Execute(ctx context.Context, req *WorkflowRequest) (*WorkflowResult, error)
    ExecuteStage(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error)
    GetProgress(ctx context.Context, workflowID string) (*WorkflowProgress, error)
}
```

### Build Workflow Stages

1. **Validation** - Input validation and configuration checks
2. **Context Preparation** - Build context processing and .dockerignore handling
3. **Dockerfile Parsing** - AST generation and validation
4. **Cache Resolution** - Cache key generation and lookup
5. **Build Execution** - BuildKit solver execution
6. **Image Assembly** - OCI manifest generation
7. **SBOM Generation** - Software bill of materials creation
8. **Image Signing** - Cryptographic signature generation
9. **Registry Push** - Image upload to registry
10. **Cache Export** - Cache data export for sharing
11. **Cleanup** - Resource cleanup and finalization

### Data Flow Architecture

```
Dockerfile + Context → Parser → AST → LLB → BuildKit Solver
                                                    ↓
Cache ← Content Store ← Snapshotter ← OCI Worker ← Build
  ↓                                                 ↓
Export → Registry ← Manifest ← Image ← Assembly ← Layers
                      ↓
              SBOM + Signatures + Attestations
```

## Security Architecture

### Security Framework (`docs/adr/0005-security-model.md`)

Comprehensive security model implementing:

1. **Rootless Execution**: User namespaces and capability restrictions
2. **Input Validation**: Comprehensive sanitization and validation
3. **Supply Chain Security**: SLSA-compliant provenance and SBOMs
4. **Cryptographic Integrity**: Image signing and verification
5. **Policy Enforcement**: OPA-based security policies

### Key Security Interfaces

```go
type SecurityManager interface {
    AuthorizeOperation(ctx context.Context, operation string, resource string, user *User) error
    SignArtifact(ctx context.Context, artifact []byte, keyRef string) (*Signature, error)
    GenerateProvenance(ctx context.Context, buildInfo *BuildInfo) (*Provenance, error)
    EnforcePolicy(ctx context.Context, policy *SecurityPolicy, context *SecurityContext) error
}
```

## Error Handling

### Error Management (`internal/errors/interfaces.go`)

Sophisticated error handling with:

1. **Automatic Classification**: Error type, severity, and recoverability detection
2. **Recovery Strategies**: Automated recovery for transient failures
3. **Rich Context**: Detailed error context for debugging
4. **User-Friendly Messages**: Clear, actionable error reporting

```go
type ErrorManager interface {
    HandleError(ctx context.Context, err error, context *ErrorContext) (*ErrorResult, error)
    RecordError(ctx context.Context, err error, context *ErrorContext) error
    GetErrorStats(ctx context.Context, timeRange TimeRange) (*ErrorStats, error)
}
```

## Architectural Decision Records

The architecture is documented through comprehensive ADRs:

- [ADR-0001: Embed BuildKit as Library](./adr/0001-embed-buildkit-as-library.md)
- [ADR-0002: Rootless Execution Strategy](./adr/0002-rootless-execution-strategy.md)
- [ADR-0003: Cache Architecture](./adr/0003-cache-architecture.md)
- [ADR-0004: Error Handling Strategy](./adr/0004-error-handling-strategy.md)
- [ADR-0005: Security Model](./adr/0005-security-model.md)

## Implementation Guidelines

### Interface Design Principles

1. **Context-Aware**: All operations accept `context.Context` for cancellation and timeout
2. **Error-Rich**: Comprehensive error types with recovery information
3. **Testable**: Interfaces designed for easy mocking and testing
4. **Extensible**: Plugin architecture for custom implementations
5. **Observable**: Built-in metrics and tracing support

### Testing Strategy

1. **Unit Tests**: Individual interface implementations
2. **Integration Tests**: Component interaction testing
3. **End-to-End Tests**: Complete workflow validation
4. **Security Tests**: Vulnerability and penetration testing
5. **Performance Tests**: Benchmarking against Docker Buildx

### Development Workflow

1. **Interface-First**: Design interfaces before implementations
2. **Mock-Driven**: Use mocks for parallel development
3. **Test-Driven**: Write tests alongside interface definitions
4. **Documentation**: Comprehensive documentation for all interfaces
5. **Review Process**: Architecture review for all interface changes

## Performance Characteristics

### Performance Targets

- **Cold Cache**: ≤110% of Docker Buildx build time
- **Warm Cache**: ≤105% of Docker Buildx build time
- **Memory Usage**: Comparable to BuildKit daemon
- **Disk Usage**: Efficient layer caching and deduplication

### Optimization Strategies

1. **Concurrent Operations**: Parallel layer builds and cache operations
2. **Smart Caching**: Content-addressable caching with intelligent pruning
3. **Network Optimization**: Connection pooling and multiplexing
4. **Memory Management**: Streaming operations and resource limits
5. **Build Optimization**: Multi-stage build parallelization

## Deployment Considerations

### Distribution

- **Single Binary**: Self-contained executable with embedded dependencies
- **Multi-Architecture**: Support for linux/amd64 and linux/arm64
- **Static Linking**: No external runtime dependencies
- **Small Footprint**: Optimized binary size for distribution

### Runtime Requirements

- **Linux Kernel**: ≥5.4 with user namespace support
- **Filesystem**: overlayfs or fuse-overlayfs support
- **Network**: Outbound HTTPS connectivity for registries
- **Storage**: Configurable cache directory with size limits

### Configuration

- **File-Based**: YAML/JSON configuration files
- **Environment**: Environment variable override support
- **CLI Flags**: Command-line flag precedence
- **Defaults**: Sensible defaults requiring minimal configuration

## Future Extensions

### Planned Enhancements

1. **Multi-Architecture Builds**: Enhanced cross-platform support
2. **Build Farms**: Distributed build execution
3. **Advanced Caching**: ML-driven cache optimization
4. **Plugin System**: Third-party integration support
5. **Web Interface**: Optional web UI for build management

### Integration Points

1. **CI/CD Systems**: GitHub Actions, GitLab CI, Jenkins integration
2. **Kubernetes**: Pod-based builds with resource limits
3. **Developer Tools**: IDE extensions and local development
4. **Monitoring**: Prometheus metrics and distributed tracing
5. **Security Tools**: SAST/DAST integration and policy enforcement

This architecture provides a solid foundation for building a production-ready, rootless container image builder that meets the performance, security, and functionality requirements outlined in the PRD.
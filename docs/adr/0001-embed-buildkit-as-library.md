# ADR-0001: Embed BuildKit as a Library for In-Process Execution

## Status
Accepted

## Context

Shmocker needs a build engine to execute Dockerfile instructions and create container images. The key requirements are:

1. **Rootless operation** - Must run without root privileges
2. **Single binary distribution** - No external daemon dependencies  
3. **Performance** - Match or exceed Docker Buildx performance
4. **Compatibility** - Support full Dockerfile grammar
5. **Build cache** - Efficient layer caching and sharing

Several approaches were considered:

### Option 1: External BuildKit Daemon
- **Pros**: Well-tested, full feature parity, separate process isolation
- **Cons**: Requires daemon management, IPC overhead, violates single-binary requirement

### Option 2: Shell-based Dockerfile Execution
- **Pros**: Simple implementation, full control
- **Cons**: Poor performance, limited cache sharing, complex multi-stage builds

### Option 3: Embed BuildKit as Library
- **Pros**: Single binary, in-process execution, full BuildKit feature set, excellent performance
- **Cons**: More complex initialization, shared process space

### Option 4: Alternative Build Engine (e.g., kaniko, img)
- **Pros**: Designed for rootless operation
- **Cons**: Feature gaps, different behavior, less ecosystem support

## Decision

We will **embed BuildKit as a library** for in-process execution.

## Rationale

1. **Architectural Alignment**: BuildKit's modular design supports library embedding through its `control` package
2. **Performance**: Eliminates IPC overhead between client and daemon
3. **Single Binary**: Satisfies distribution requirement without external dependencies
4. **Feature Completeness**: Inherits BuildKit's full Dockerfile grammar support and optimization capabilities
5. **Cache System**: Leverages BuildKit's advanced caching with content-addressable storage
6. **Rootless Support**: BuildKit's rootless mode is production-ready and well-tested
7. **Ecosystem**: Maintains compatibility with existing BuildKit cache backends and frontends

## Implementation Approach

### Core Integration Points

1. **BuildKit Controller**: Embed `github.com/moby/buildkit/control` package
2. **Worker Configuration**: Use rootless OCI worker with overlayfs snapshotter
3. **Frontend**: Utilize BuildKit's Dockerfile frontend for parsing and LLB generation
4. **Solver**: Direct access to BuildKit's solver for build execution
5. **Content Store**: Embedded content-addressable storage for layer management

### Key Components

```go
// Main integration interface
type BuildKitController interface {
    Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error)
    ImportCache(ctx context.Context, imports []*CacheImport) error
    ExportCache(ctx context.Context, exports []*CacheExport) error
    GetSession(ctx context.Context) (Session, error)
    Close() error
}

// Worker abstraction for rootless execution
type Worker interface {
    GetWorkerController() WorkerController
    Platforms() []Platform
    Executor() Executor
    CacheManager() cache.Manager
}
```

### Integration Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Shmocker Process                       │
├─────────────────────────────────────────────────────────────┤
│  CLI Layer                                                  │
│  ├─ Command Parsing                                         │
│  └─ Progress Reporting                                      │
├─────────────────────────────────────────────────────────────┤
│  Workflow Orchestration                                     │
│  ├─ Stage Management                                        │
│  ├─ Error Handling                                          │
│  └─ Resource Cleanup                                        │
├─────────────────────────────────────────────────────────────┤
│  Builder Interface Layer                                    │
│  ├─ Build Request Processing                                │
│  ├─ Context Preparation                                     │
│  └─ Result Assembly                                         │
├─────────────────────────────────────────────────────────────┤
│  Embedded BuildKit                                          │
│  ├─ Control Client ──────────────┐                         │
│  ├─ Dockerfile Frontend          │                         │
│  ├─ LLB Solver ──────────────────┼─ In-Process            │
│  ├─ OCI Worker (Rootless)        │                         │
│  ├─ Snapshotter (overlayfs)      │                         │
│  ├─ Executor (runc)              │                         │
│  └─ Content Store ───────────────┘                         │
└─────────────────────────────────────────────────────────────┘
```

## Consequences

### Positive

1. **Performance**: Eliminates daemon communication overhead
2. **Simplicity**: Single process to manage and distribute
3. **Resource Efficiency**: Shared memory space, better resource utilization
4. **Debugging**: Easier to debug with single process
5. **Deployment**: Simplified deployment model

### Negative

1. **Memory Usage**: BuildKit components share process memory
2. **Isolation**: Less isolation between build operations
3. **Initialization**: More complex startup sequence
4. **Resource Conflicts**: Potential for resource conflicts in shared process space

### Mitigation Strategies

1. **Memory Management**: Implement proper cleanup and resource management
2. **Concurrent Builds**: Use BuildKit's built-in concurrency controls
3. **Error Isolation**: Robust error handling to prevent cascade failures
4. **Resource Limits**: Implement resource limiting at the workflow level

## Implementation Phases

### Phase 1: Core Integration
- Embed BuildKit control client
- Configure rootless OCI worker
- Basic Dockerfile building

### Phase 2: Cache Integration
- Content-addressable cache
- Import/export capabilities
- Multi-stage optimization

### Phase 3: Advanced Features
- Multi-platform builds
- Secret handling
- SSH forwarding

### Phase 4: Production Hardening
- Resource management
- Error recovery
- Performance optimization

## References

- [BuildKit Architecture](https://github.com/moby/buildkit/blob/master/docs/architecture.md)
- [BuildKit Client API](https://pkg.go.dev/github.com/moby/buildkit/client)
- [Rootless Mode Documentation](https://github.com/moby/buildkit/blob/master/docs/rootless.md)
- [PRD Section 7: Key Decisions & Rationale](../prd/rootless_docker_image_builder_replacement_prd.md#7-key-decisions--rationale)

## Related ADRs

- [ADR-0002: Rootless Execution Strategy](./0002-rootless-execution-strategy.md)
- [ADR-0003: Cache Architecture](./0003-cache-architecture.md)
- [ADR-0004: Error Handling Strategy](./0004-error-handling-strategy.md)
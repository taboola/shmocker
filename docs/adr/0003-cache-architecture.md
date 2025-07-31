# ADR-0003: Cache Architecture

## Status
Accepted

## Context

Build caching is critical for achieving the performance goals outlined in the PRD. The requirements are:

1. **Performance**: Cold cache builds ≤110% of Docker Buildx, warm cache ≤105%
2. **Content-addressable**: Layer caching based on content hashes
3. **Multi-stage**: Efficient caching across multi-stage builds
4. **Import/Export**: Cache sharing across environments
5. **Persistence**: Durable cache across build sessions
6. **Size Management**: Automatic cache pruning and size limits

Several cache architectures were evaluated:

### Option 1: Simple File-based Cache
- **Pros**: Simple implementation, good for local development
- **Cons**: Limited sharing, no content-addressability, poor performance at scale

### Option 2: Content-Addressable Store (CAS) 
- **Pros**: Deduplication, content-addressable, efficient sharing
- **Cons**: More complex implementation, storage overhead for metadata

### Option 3: Registry-based Cache
- **Pros**: Remote sharing, OCI-compatible, standardized
- **Cons**: Network dependency, authentication complexity, limited local performance

### Option 4: Hybrid Multi-tier Cache
- **Pros**: Best performance characteristics, flexible, scalable
- **Cons**: Most complex implementation, multiple failure modes

## Decision

We will implement a **hybrid multi-tier cache architecture** with:

1. **L1 Cache**: Local content-addressable store (primary)
2. **L2 Cache**: Remote registry cache (optional)  
3. **L3 Cache**: External cache backends (S3, GCS, etc.)

## Rationale

### Performance Requirements

The hybrid approach provides optimal performance by:

- **L1 (Local CAS)**: Sub-millisecond layer lookup, no network latency
- **L2 (Registry)**: Fast layer sharing across environments
- **L3 (External)**: Massive scale cache storage for CI/CD pipelines

### Content-Addressable Architecture

All cache layers use content-addressable storage:

```go
type CacheKey struct {
    // Content hash of the layer
    Digest string `json:"digest"`
    
    // Build context that produced this layer  
    ContextHash string `json:"context_hash"`
    
    // Platform specification
    Platform string `json:"platform"`
    
    // Additional discriminators
    BuildArgs map[string]string `json:"build_args,omitempty"`
    Target    string            `json:"target,omitempty"`
}

func (k *CacheKey) String() string {
    h := sha256.New()
    h.Write([]byte(k.Digest))
    h.Write([]byte(k.ContextHash)) 
    h.Write([]byte(k.Platform))
    
    // Include build args in hash
    for key, value := range k.BuildArgs {
        h.Write([]byte(key + "=" + value))
    }
    
    return fmt.Sprintf("sha256:%x", h.Sum(nil))
}
```

## Architecture Design

### Cache Manager Interface

```go
type Manager interface {
    // Layer operations
    Get(ctx context.Context, key string) (*CacheEntry, error)
    Put(ctx context.Context, key string, data io.Reader, metadata *CacheMetadata) error
    Delete(ctx context.Context, key string) error
    
    // Bulk operations
    List(ctx context.Context, prefix string) ([]string, error)
    Clear(ctx context.Context) error
    
    // Management operations
    Size(ctx context.Context) (int64, error)
    Stats(ctx context.Context) (*CacheStats, error)
    Prune(ctx context.Context, opts *PruneOptions) (*PruneResult, error)
    
    // Import/Export
    Export(ctx context.Context, req *ExportRequest) (*ExportResult, error)
    Import(ctx context.Context, req *ImportRequest) (*ImportResult, error)
}
```

### Multi-tier Implementation

```go
type MultiTierCache struct {
    tiers []CacheTier
    
    // Configuration
    config *CacheConfig
    
    // Metrics and monitoring
    metrics CacheMetrics
}

type CacheTier struct {
    name     string
    store    Store
    priority int
    readonly bool
    
    // Tier-specific configuration
    config TierConfig
}

func (c *MultiTierCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
    // Try each tier in priority order
    for _, tier := range c.tiers {
        entry, err := tier.store.Get(ctx, key)
        if err == nil {
            // Promote to higher tiers (write-through)
            c.promoteEntry(ctx, key, entry)
            return entry, nil
        }
        if !isNotFoundError(err) {
            log.Warnf("Cache tier %s error: %v", tier.name, err)
        }
    }
    return nil, ErrCacheNotFound
}
```

### Local Content-Addressable Store (L1)

Primary cache layer optimized for local performance:

```go
type LocalCAS struct {
    // Root directory for cache storage
    root string
    
    // Content-addressable blob storage
    blobStore BlobStore
    
    // Metadata index
    metadata MetadataStore
    
    // Configuration
    config *LocalCASConfig
}

type LocalCASConfig struct {
    // Storage limits
    MaxSize    int64         `json:"max_size"`
    MaxEntries int64         `json:"max_entries"`
    
    // Cleanup policy
    TTL             time.Duration `json:"ttl"`
    CleanupInterval time.Duration `json:"cleanup_interval"`
    
    // Performance tuning
    CompressionLevel int    `json:"compression_level"`
    SyncWrites      bool   `json:"sync_writes"`
    UseDirectIO     bool   `json:"use_direct_io"`
}
```

### Directory Structure

```
~/.shmocker/cache/
├── config.json                    # Cache configuration
├── blobs/                         # Content-addressable blobs
│   ├── sha256/
│   │   ├── ab/
│   │   │   └── ab123...def        # Blob data (first 2 chars as subdirs)
│   │   └── cd/
│   │       └── cd456...789
├── metadata/                      # Cache metadata
│   ├── index.db                   # SQLite index
│   └── locks/                     # File locks for concurrent access
└── temp/                          # Temporary files during writes
    └── .tmp.123456
```

### Registry Cache Integration (L2)

```go
type RegistryCache struct {
    client registry.Client
    config *RegistryCacheConfig
}

type RegistryCacheConfig struct {
    // Registry configuration
    Registry string            `json:"registry"`
    Repo     string            `json:"repo"`
    Auth     *AuthConfig       `json:"auth,omitempty"`
    
    // Cache behavior
    PushOnWrite bool           `json:"push_on_write"`
    Compression CompressionType `json:"compression"`
    
    // Performance
    ConcurrentUploads   int `json:"concurrent_uploads"`
    ConcurrentDownloads int `json:"concurrent_downloads"`
}

// Registry cache uses OCI artifacts for cache data
func (r *RegistryCache) Put(ctx context.Context, key string, data io.Reader, metadata *CacheMetadata) error {
    // Create OCI artifact for cache entry
    artifact := &oci.Artifact{
        MediaType: "application/vnd.shmocker.cache.layer.v1+gzip",
        Digest:    metadata.Digest,
        Size:      metadata.Size,
        Annotations: map[string]string{
            "org.shmocker.cache.key":         key,
            "org.shmocker.cache.created":     metadata.CreatedAt.Format(time.RFC3339),
            "org.shmocker.cache.platform":    metadata.Platform,
        },
    }
    
    return r.client.PushArtifact(ctx, r.getRef(key), artifact, data)
}
```

## Cache Key Generation

### Dockerfile-based Caching

```go
type DockerfileCacheKeyGenerator struct{}

func (g *DockerfileCacheKeyGenerator) GenerateKey(input *KeyInput) (string, error) {
    // Hash build context
    contextHash, err := g.hashBuildContext(input.Context)
    if err != nil {
        return "", err
    }
    
    // Hash Dockerfile content up to this instruction
    dockerfileHash, err := g.hashDockerfilePrefix(input.Dockerfile, input.LayerIndex)
    if err != nil {
        return "", err
    }
    
    // Create composite key
    key := CacheKey{
        Digest:      input.LayerDigest,
        ContextHash: contextHash,
        Platform:    input.Platform,
        BuildArgs:   input.BuildArgs,
        Target:      input.Target,
    }
    
    return key.String(), nil
}

func (g *DockerfileCacheKeyGenerator) hashDockerfilePrefix(dockerfile string, upToIndex int) (string, error) {
    lines := strings.Split(dockerfile, "\n")
    if upToIndex >= len(lines) {
        upToIndex = len(lines) - 1
    }
    
    // Include only instructions up to the specified index
    prefix := strings.Join(lines[:upToIndex+1], "\n")
    
    h := sha256.New()
    h.Write([]byte(prefix))
    return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}
```

### Build Context Hashing

```go
func (g *DockerfileCacheKeyGenerator) hashBuildContext(contextPath string) (string, error) {
    hasher := sha256.New()
    
    err := filepath.Walk(contextPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        // Skip files based on .dockerignore
        if g.shouldIgnore(path) {
            return nil
        }
        
        // Include file path and metadata in hash
        relPath, _ := filepath.Rel(contextPath, path)
        hasher.Write([]byte(relPath))
        hasher.Write([]byte(fmt.Sprintf("%d", info.Size())))
        hasher.Write([]byte(fmt.Sprintf("%d", info.ModTime().Unix())))
        
        // Include file content for small files
        if info.Size() < 1024*1024 { // 1MB threshold
            content, err := os.ReadFile(path)
            if err != nil {
                return err
            }
            hasher.Write(content)
        }
        
        return nil
    })
    
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf("sha256:%x", hasher.Sum(nil)), nil
}
```

## Cache Import/Export

### Export Configuration

```go
type ExportRequest struct {
    Type        ExportType        `json:"type"`
    Destination string            `json:"destination"`
    Keys        []string          `json:"keys,omitempty"`
    
    // Filtering
    Filter      func(*CacheEntry) bool `json:"-"`
    MaxAge      time.Duration     `json:"max_age,omitempty"`
    MinSize     int64             `json:"min_size,omitempty"`
    
    // Compression and optimization
    Compression CompressionType   `json:"compression,omitempty"`
    Deduplicate bool             `json:"deduplicate,omitempty"`
}

// Supported export types
const (
    ExportTypeRegistry ExportType = "registry"   // OCI registry
    ExportTypeLocal    ExportType = "local"      // Local directory/tar
    ExportTypeS3       ExportType = "s3"         // AWS S3
    ExportTypeGCS      ExportType = "gcs"        // Google Cloud Storage
    ExportTypeGitHub   ExportType = "gha"        // GitHub Actions cache
)
```

### Import Strategies

```go
type ImportStrategy interface {
    Import(ctx context.Context, req *ImportRequest) (*ImportResult, error)
    Validate(req *ImportRequest) error
    EstimateSize(req *ImportRequest) (int64, error)
}

// Registry import strategy
type RegistryImportStrategy struct {
    client registry.Client
}

func (s *RegistryImportStrategy) Import(ctx context.Context, req *ImportRequest) (*ImportResult, error) {
    manifest, err := s.client.GetManifest(ctx, req.Source)
    if err != nil {
        return nil, err
    }
    
    result := &ImportResult{
        Source: req.Source,
        StartTime: time.Now(),
    }
    
    // Import each layer
    for _, layer := range manifest.Layers {
        blob, err := s.client.GetBlob(ctx, req.Source, layer.Digest)
        if err != nil {
            continue // Skip missing layers
        }
        
        // Store in local cache
        key := generateCacheKey(layer)
        err = s.cache.Put(ctx, key, blob, &CacheMetadata{
            Digest:    layer.Digest,
            Size:      layer.Size,
            MediaType: layer.MediaType,
            CreatedAt: time.Now(),
        })
        
        if err == nil {
            result.ImportedEntries++
            result.ImportedSize += layer.Size
        }
    }
    
    result.Duration = time.Since(result.StartTime)
    return result, nil
}
```

## Performance Optimizations

### Concurrent Operations

```go
type ConcurrentCache struct {
    cache Manager
    
    // Concurrency controls
    readSemaphore  chan struct{}
    writeSemaphore chan struct{}
    
    // Connection pooling for remote caches
    connPool *ConnectionPool
}

func (c *ConcurrentCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
    // Acquire read permit
    select {
    case c.readSemaphore <- struct{}{}:
        defer func() { <-c.readSemaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    
    return c.cache.Get(ctx, key)
}
```

### Compression and Deduplication

```go
type CompressedCache struct {
    underlying Manager
    compressor Compressor
    
    // Compression configuration
    algorithm CompressionType
    level     int
    threshold int64  // Minimum size to compress
}

func (c *CompressedCache) Put(ctx context.Context, key string, data io.Reader, metadata *CacheMetadata) error {
    // Compress large entries
    if metadata.Size > c.threshold {
        compressed, err := c.compressor.Compress(data)
        if err != nil {
            return err
        }
        
        // Update metadata
        metadata.CompressionType = c.algorithm
        data = compressed
    }
    
    return c.underlying.Put(ctx, key, data, metadata)
}
```

## Consequences

### Positive

1. **Performance**: Multi-tier approach optimizes for different use cases
2. **Flexibility**: Supports various cache backends and configurations  
3. **Scalability**: Can scale from local development to large CI/CD systems
4. **Efficiency**: Content-addressable storage eliminates duplication
5. **Reliability**: Graceful degradation when cache tiers are unavailable

### Negative

1. **Complexity**: Multiple cache implementations to maintain
2. **Storage**: Increased storage requirements for metadata
3. **Consistency**: Potential consistency issues across cache tiers
4. **Debugging**: More complex troubleshooting across multiple systems

### Mitigation Strategies

1. **Testing**: Comprehensive test suite covering all cache scenarios
2. **Monitoring**: Detailed metrics and alerting for cache performance
3. **Documentation**: Clear configuration guidelines and troubleshooting guides
4. **Fallback**: Graceful operation when cache layers fail

## Implementation Roadmap

### Phase 1: Local CAS Implementation
- Basic content-addressable store
- File-based blob storage
- Metadata indexing
- Cache size management

### Phase 2: Multi-tier Architecture
- Cache tier abstraction
- Priority-based lookup
- Write-through promotion
- Fallback handling

### Phase 3: Registry Integration
- OCI registry cache backend
- Push/pull optimization
- Authentication handling
- Manifest-based cache organization

### Phase 4: Advanced Features
- External cache backends (S3, GCS)
- Advanced compression algorithms
- Cache sharing and collaboration
- Performance analytics

## References

- [BuildKit Cache Documentation](https://github.com/moby/buildkit/blob/master/docs/cache.md)
- [OCI Artifacts Specification](https://github.com/opencontainers/artifacts)
- [Content-Addressable Storage Design](https://en.wikipedia.org/wiki/Content-addressable_storage)

## Related ADRs

- [ADR-0001: Embed BuildKit as Library](./0001-embed-buildkit-as-library.md)
- [ADR-0002: Rootless Execution Strategy](./0002-rootless-execution-strategy.md)
- [ADR-0004: Error Handling Strategy](./0004-error-handling-strategy.md)
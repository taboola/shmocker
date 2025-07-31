// Package cache defines interfaces for build cache management.
package cache

import (
	"context"
	"io"
	"time"
)

// Manager provides the main interface for build cache operations.
type Manager interface {
	// Get retrieves cached data by key
	Get(ctx context.Context, key string) (*CacheEntry, error)
	
	// Put stores data in the cache
	Put(ctx context.Context, key string, data io.Reader, metadata *CacheMetadata) error
	
	// Delete removes an entry from the cache
	Delete(ctx context.Context, key string) error
	
	// List returns all cache keys matching the given prefix
	List(ctx context.Context, prefix string) ([]string, error)
	
	// Clear removes all cache entries
	Clear(ctx context.Context) error
	
	// Size returns the total cache size
	Size(ctx context.Context) (int64, error)
	
	// Stats returns cache statistics
	Stats(ctx context.Context) (*CacheStats, error)
	
	// Prune removes expired or least recently used entries
	Prune(ctx context.Context, opts *PruneOptions) (*PruneResult, error)
	
	// Close closes the cache manager
	Close() error
}

// Store provides low-level cache storage operations.
type Store interface {
	// Read reads data from the cache store
	Read(ctx context.Context, key string) (io.ReadCloser, error)
	
	// Write writes data to the cache store
	Write(ctx context.Context, key string, data io.Reader) error
	
	// Exists checks if a key exists in the store
	Exists(ctx context.Context, key string) (bool, error)
	
	// Remove removes data from the store
	Remove(ctx context.Context, key string) error
	
	// Walk iterates over all keys in the store
	Walk(ctx context.Context, fn WalkFunc) error
	
	// Size returns the size of data for a key
	Size(ctx context.Context, key string) (int64, error)
	
	// LastModified returns the last modified time for a key
	LastModified(ctx context.Context, key string) (time.Time, error)
}

// KeyGenerator provides cache key generation strategies.
type KeyGenerator interface {
	// GenerateKey generates a cache key for the given input
	GenerateKey(input *KeyInput) (string, error)
	
	// ValidateKey validates a cache key
	ValidateKey(key string) error
	
	// ParseKey parses a cache key into components
	ParseKey(key string) (*KeyComponents, error)
}

// Exporter provides cache export functionality.
type Exporter interface {
	// Export exports cache data to an external destination
	Export(ctx context.Context, req *ExportRequest) (*ExportResult, error)
	
	// GetSupportedTypes returns supported export types
	GetSupportedTypes() []ExportType
	
	// ValidateExportConfig validates export configuration
	ValidateExportConfig(config *ExportConfig) error
}

// Importer provides cache import functionality.
type Importer interface {
	// Import imports cache data from an external source
	Import(ctx context.Context, req *ImportRequest) (*ImportResult, error)
	
	// GetSupportedTypes returns supported import types
	GetSupportedTypes() []ImportType
	
	// ValidateImportConfig validates import configuration
	ValidateImportConfig(config *ImportConfig) error
}

// Compressor provides cache data compression.
type Compressor interface {
	// Compress compresses data
	Compress(data io.Reader) (io.Reader, error)
	
	// Decompress decompresses data
	Decompress(data io.Reader) (io.Reader, error)
	
	// GetType returns the compression type
	GetType() CompressionType
	
	// GetRatio estimates the compression ratio
	GetRatio() float64
}

// Encryptor provides cache data encryption.
type Encryptor interface {
	// Encrypt encrypts data
	Encrypt(data io.Reader) (io.Reader, error)
	
	// Decrypt decrypts data
	Decrypt(data io.Reader) (io.Reader, error)
	
	// GetType returns the encryption type
	GetType() EncryptionType
}

// CacheEntry represents a cached entry.
type CacheEntry struct {
	// Key is the cache key
	Key string `json:"key"`
	
	// Data provides access to the cached data
	Data io.ReadCloser `json:"-"`
	
	// Metadata contains cache metadata
	Metadata *CacheMetadata `json:"metadata"`
	
	// Size is the data size
	Size int64 `json:"size"`
}

// CacheMetadata contains metadata for cache entries.
type CacheMetadata struct {
	// ContentType is the content type
	ContentType string `json:"content_type,omitempty"`
	
	// CreatedAt is when the entry was created
	CreatedAt time.Time `json:"created_at"`
	
	// LastAccessed is when the entry was last accessed
	LastAccessed time.Time `json:"last_accessed"`
	
	// ExpiresAt is when the entry expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	
	// Tags contains arbitrary tags
	Tags map[string]string `json:"tags,omitempty"`
	
	// Checksum is the data checksum
	Checksum string `json:"checksum,omitempty"`
	
	// CompressionType indicates if data is compressed
	CompressionType CompressionType `json:"compression_type,omitempty"`
	
	// EncryptionType indicates if data is encrypted
	EncryptionType EncryptionType `json:"encryption_type,omitempty"`
	
	// BuildArgs contains build arguments that affect this cache entry
	BuildArgs map[string]string `json:"build_args,omitempty"`
	
	// Platform is the target platform
	Platform string `json:"platform,omitempty"`
	
	// LayerDigest is the layer digest this cache entry corresponds to
	LayerDigest string `json:"layer_digest,omitempty"`
}

// CacheStats provides cache statistics.
type CacheStats struct {
	// TotalEntries is the total number of cache entries
	TotalEntries int64 `json:"total_entries"`
	
	// TotalSize is the total cache size in bytes
	TotalSize int64 `json:"total_size"`
	
	// HitCount is the number of cache hits
	HitCount int64 `json:"hit_count"`
	
	// MissCount is the number of cache misses
	MissCount int64 `json:"miss_count"`
	
	// HitRatio is the cache hit ratio
	HitRatio float64 `json:"hit_ratio"`
	
	// AverageEntrySize is the average entry size
	AverageEntrySize int64 `json:"average_entry_size"`
	
	// OldestEntry is the timestamp of the oldest entry
	OldestEntry *time.Time `json:"oldest_entry,omitempty"`
	
	// NewestEntry is the timestamp of the newest entry
	NewestEntry *time.Time `json:"newest_entry,omitempty"`
	
	// ExpiredEntries is the number of expired entries
	ExpiredEntries int64 `json:"expired_entries"`
}

// PruneOptions contains options for cache pruning.
type PruneOptions struct {
	// MaxAge removes entries older than this duration
	MaxAge time.Duration `json:"max_age,omitempty"`
	
	// MaxSize removes entries until cache is below this size
	MaxSize int64 `json:"max_size,omitempty"`
	
	// MaxEntries removes entries until cache has fewer than this many entries
	MaxEntries int64 `json:"max_entries,omitempty"`
	
	// Strategy specifies the pruning strategy
	Strategy PruneStrategy `json:"strategy,omitempty"`
	
	// DryRun only reports what would be pruned without actually removing
	DryRun bool `json:"dry_run,omitempty"`
	
	// Filter allows custom filtering of entries to prune
	Filter func(*CacheEntry) bool `json:"-"`
}

// PruneStrategy represents a cache pruning strategy.
type PruneStrategy string

const (
	PruneStrategyLRU       PruneStrategy = "lru"       // Least Recently Used
	PruneStrategyLFU       PruneStrategy = "lfu"       // Least Frequently Used
	PruneStrategyFIFO      PruneStrategy = "fifo"      // First In, First Out
	PruneStrategySize      PruneStrategy = "size"      // Largest entries first
	PruneStrategyRandom    PruneStrategy = "random"    // Random selection
	PruneStrategyExpired   PruneStrategy = "expired"   // Expired entries only
)

// PruneResult contains the results of a cache pruning operation.
type PruneResult struct {
	// RemovedEntries is the number of entries removed
	RemovedEntries int64 `json:"removed_entries"`
	
	// RemovedSize is the total size of removed data
	RemovedSize int64 `json:"removed_size"`
	
	// RemainingEntries is the number of entries remaining
	RemainingEntries int64 `json:"remaining_entries"`
	
	// RemainingSize is the total size of remaining data
	RemainingSize int64 `json:"remaining_size"`
	
	// Duration is how long the pruning took
	Duration time.Duration `json:"duration"`
	
	// Errors contains any errors encountered during pruning
	Errors []string `json:"errors,omitempty"`
}

// KeyInput contains input for cache key generation.
type KeyInput struct {
	// Context is the build context path or reference
	Context string `json:"context"`
	
	// Dockerfile is the Dockerfile content or path
	Dockerfile string `json:"dockerfile"`
	
	// BuildArgs contains build arguments
	BuildArgs map[string]string `json:"build_args,omitempty"`
	
	// Platform is the target platform
	Platform string `json:"platform,omitempty"`
	
	// Target is the build target
	Target string `json:"target,omitempty"`
	
	// BaseImage is the base image reference
	BaseImage string `json:"base_image,omitempty"`
	
	// LayerIndex is the layer index within the build
	LayerIndex int `json:"layer_index,omitempty"`
	
	// Instruction is the Dockerfile instruction
	Instruction string `json:"instruction,omitempty"`
	
	// Additional contains additional key components
	Additional map[string]string `json:"additional,omitempty"`
}

// KeyComponents represents the components of a parsed cache key.
type KeyComponents struct {
	// Namespace is the cache namespace
	Namespace string `json:"namespace"`
	
	// Type is the cache entry type
	Type CacheType `json:"type"`
	
	// Version is the cache format version
	Version string `json:"version"`
	
	// Hash is the content hash
	Hash string `json:"hash"`
	
	// Platform is the target platform
	Platform string `json:"platform,omitempty"`
	
	// Additional contains additional components
	Additional map[string]string `json:"additional,omitempty"`
}

// CacheType represents the type of cached data.
type CacheType string

const (
	CacheTypeLayer       CacheType = "layer"       // Layer cache
	CacheTypeManifest    CacheType = "manifest"    // Manifest cache
	CacheTypeConfig      CacheType = "config"      // Config cache
	CacheTypeSource      CacheType = "source"      // Source cache
	CacheTypeDependency  CacheType = "dependency"  // Dependency cache
	CacheTypeExecution   CacheType = "execution"   // Execution result cache
	CacheTypeAttestation CacheType = "attestation" // Attestation cache
)

// ExportRequest represents a cache export request.
type ExportRequest struct {
	// Type is the export type
	Type ExportType `json:"type"`
	
	// Config contains export configuration
	Config *ExportConfig `json:"config"`
	
	// Keys contains specific keys to export (empty = all)
	Keys []string `json:"keys,omitempty"`
	
	// Filter allows custom filtering of entries
	Filter func(*CacheEntry) bool `json:"-"`
	
	// Compression enables compression
	Compression CompressionType `json:"compression,omitempty"`
	
	// Encryption enables encryption
	Encryption EncryptionType `json:"encryption,omitempty"`
}

// ExportConfig contains configuration for cache export.
type ExportConfig struct {
	// Destination is the export destination
	Destination string `json:"destination"`
	
	// Credentials contains authentication credentials
	Credentials map[string]string `json:"credentials,omitempty"`
	
	// Options contains type-specific options
	Options map[string]interface{} `json:"options,omitempty"`
	
	// Metadata contains export metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ExportType represents a cache export type.
type ExportType string

const (
	ExportTypeRegistry   ExportType = "registry"   // Export to OCI registry
	ExportTypeLocal      ExportType = "local"      // Export to local directory
	ExportTypeS3         ExportType = "s3"         // Export to S3
	ExportTypeGCS        ExportType = "gcs"        // Export to Google Cloud Storage
	ExportTypeAzure      ExportType = "azure"      // Export to Azure Blob Storage
	ExportTypeInline     ExportType = "inline"     // Inline cache export
	ExportTypeGitHub     ExportType = "gha"        // GitHub Actions cache
)

// ExportResult contains the result of a cache export operation.
type ExportResult struct {
	// ExportedEntries is the number of exported entries
	ExportedEntries int64 `json:"exported_entries"`
	
	// ExportedSize is the total size of exported data
	ExportedSize int64 `json:"exported_size"`
	
	// Destination is the export destination
	Destination string `json:"destination"`
	
	// Duration is how long the export took
	Duration time.Duration `json:"duration"`
	
	// Manifest contains the export manifest
	Manifest *ExportManifest `json:"manifest,omitempty"`
	
	// Errors contains any errors encountered during export
	Errors []string `json:"errors,omitempty"`
}

// ImportRequest represents a cache import request.
type ImportRequest struct {
	// Type is the import type
	Type ImportType `json:"type"`
	
	// Config contains import configuration
	Config *ImportConfig `json:"config"`
	
	// Keys contains specific keys to import (empty = all)
	Keys []string `json:"keys,omitempty"`
	
	// Filter allows custom filtering of entries
	Filter func(*CacheEntry) bool `json:"-"`
	
	// Overwrite overwrites existing entries
	Overwrite bool `json:"overwrite,omitempty"`
}

// ImportConfig contains configuration for cache import.
type ImportConfig struct {
	// Source is the import source
	Source string `json:"source"`
	
	// Credentials contains authentication credentials
	Credentials map[string]string `json:"credentials,omitempty"`
	
	// Options contains type-specific options
	Options map[string]interface{} `json:"options,omitempty"`
	
	// Metadata contains import metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ImportType represents a cache import type.
type ImportType string

const (
	ImportTypeRegistry   ImportType = "registry"   // Import from OCI registry
	ImportTypeLocal      ImportType = "local"      // Import from local directory
	ImportTypeS3         ImportType = "s3"         // Import from S3
	ImportTypeGCS        ImportType = "gcs"        // Import from Google Cloud Storage
	ImportTypeAzure      ImportType = "azure"      // Import from Azure Blob Storage
	ImportTypeInline     ImportType = "inline"     // Inline cache import
	ImportTypeGitHub     ImportType = "gha"        // GitHub Actions cache
)

// ImportResult contains the result of a cache import operation.
type ImportResult struct {
	// ImportedEntries is the number of imported entries
	ImportedEntries int64 `json:"imported_entries"`
	
	// ImportedSize is the total size of imported data
	ImportedSize int64 `json:"imported_size"`
	
	// SkippedEntries is the number of skipped entries
	SkippedEntries int64 `json:"skipped_entries"`
	
	// Source is the import source
	Source string `json:"source"`
	
	// Duration is how long the import took
	Duration time.Duration `json:"duration"`
	
	// Manifest contains the import manifest
	Manifest *ImportManifest `json:"manifest,omitempty"`
	
	// Errors contains any errors encountered during import
	Errors []string `json:"errors,omitempty"`
}

// CompressionType represents a compression algorithm.
type CompressionType string

const (
	CompressionTypeNone   CompressionType = "none"
	CompressionTypeGzip   CompressionType = "gzip"
	CompressionTypeZstd   CompressionType = "zstd"
	CompressionTypeLZ4    CompressionType = "lz4"
	CompressionTypeBzip2  CompressionType = "bzip2"
)

// EncryptionType represents an encryption algorithm.
type EncryptionType string

const (
	EncryptionTypeNone   EncryptionType = "none"
	EncryptionTypeAES256 EncryptionType = "aes256"
	EncryptionTypeChaCha EncryptionType = "chacha20"
)

// ExportManifest contains metadata about exported cache data.
type ExportManifest struct {
	// Version is the manifest version
	Version string `json:"version"`
	
	// CreatedAt is when the export was created
	CreatedAt time.Time `json:"created_at"`
	
	// Source is the export source
	Source string `json:"source"`
	
	// Entries contains information about exported entries
	Entries []*ManifestEntry `json:"entries"`
	
	// Metadata contains additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ImportManifest contains metadata about imported cache data.
type ImportManifest struct {
	// Version is the manifest version
	Version string `json:"version"`
	
	// CreatedAt is when the import was created
	CreatedAt time.Time `json:"created_at"`
	
	// Source is the import source
	Source string `json:"source"`
	
	// Entries contains information about imported entries
	Entries []*ManifestEntry `json:"entries"`
	
	// Metadata contains additional metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ManifestEntry represents an entry in an export/import manifest.
type ManifestEntry struct {
	// Key is the cache key
	Key string `json:"key"`
	
	// Size is the entry size
	Size int64 `json:"size"`
	
	// Checksum is the entry checksum
	Checksum string `json:"checksum"`
	
	// CreatedAt is when the entry was created
	CreatedAt time.Time `json:"created_at"`
	
	// Metadata contains entry metadata
	Metadata *CacheMetadata `json:"metadata,omitempty"`
}

// WalkFunc is a function type for walking cache entries.
type WalkFunc func(key string, entry *CacheEntry) error

// Config contains cache manager configuration.
type Config struct {
	// Type is the cache type
	Type string `json:"type"`
	
	// Directory is the cache directory (for file-based caches)
	Directory string `json:"directory,omitempty"`
	
	// MaxSize is the maximum cache size
	MaxSize int64 `json:"max_size,omitempty"`
	
	// MaxEntries is the maximum number of entries
	MaxEntries int64 `json:"max_entries,omitempty"`
	
	// TTL is the default time-to-live for entries
	TTL time.Duration `json:"ttl,omitempty"`
	
	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration `json:"cleanup_interval,omitempty"`
	
	// CompressionType is the default compression type
	CompressionType CompressionType `json:"compression_type,omitempty"`
	
	// EncryptionType is the default encryption type
	EncryptionType EncryptionType `json:"encryption_type,omitempty"`
	
	// EncryptionKey is the encryption key
	EncryptionKey string `json:"encryption_key,omitempty"`
	
	// Options contains type-specific options
	Options map[string]interface{} `json:"options,omitempty"`
}
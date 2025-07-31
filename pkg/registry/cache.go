// Package registry provides cache import/export functionality using OCI registries.
package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"

	"github.com/shmocker/shmocker/pkg/cache"
)

// RegistryCacheExporter implements cache export to OCI registries.
type RegistryCacheExporter struct {
	client Client
	config *RegistryCacheConfig
}

// RegistryCacheImporter implements cache import from OCI registries.
type RegistryCacheImporter struct {
	client Client
	config *RegistryCacheConfig
}

// RegistryCacheConfig contains configuration for registry-based cache operations.
type RegistryCacheConfig struct {
	// Registry is the registry hostname
	Registry string `json:"registry"`
	
	// Repository is the repository name for cache artifacts
	Repository string `json:"repository"`
	
	// Namespace is the namespace prefix for cache artifacts
	Namespace string `json:"namespace,omitempty"`
	
	// Auth contains authentication configuration
	Auth *Credentials `json:"auth,omitempty"`
	
	// Compression enables compression of cache artifacts
	Compression cache.CompressionType `json:"compression,omitempty"`
	
	// MaxConcurrentUploads limits concurrent upload operations
	MaxConcurrentUploads int `json:"max_concurrent_uploads,omitempty"`
	
	// MaxConcurrentDownloads limits concurrent download operations
	MaxConcurrentDownloads int `json:"max_concurrent_downloads,omitempty"`
	
	// CacheVersion is the cache format version
	CacheVersion string `json:"cache_version,omitempty"`
}

// CacheArtifact represents a cache artifact in OCI format.
type CacheArtifact struct {
	// MediaType identifies the artifact type
	MediaType string `json:"mediaType"`
	
	// Digest is the content digest
	Digest string `json:"digest"`
	
	// Size is the content size
	Size int64 `json:"size"`
	
	// Annotations contain metadata about the cache entry
	Annotations map[string]string `json:"annotations,omitempty"`
	
	// Data contains the cache entry data
	Data io.ReadCloser `json:"-"`
}

// CacheManifest represents an OCI manifest for cache artifacts.
type CacheManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	MediaType     string                 `json:"mediaType"`
	Config        *Descriptor            `json:"config"`
	Layers        []*Descriptor          `json:"layers"`
	Annotations   map[string]string      `json:"annotations,omitempty"`
	Subject       *Descriptor            `json:"subject,omitempty"`
}

// Cache artifact media types
const (
	CacheArtifactMediaType        = "application/vnd.shmocker.cache.artifact.v1+json"
	CacheConfigMediaType          = "application/vnd.shmocker.cache.config.v1+json"
	CacheLayerMediaType           = "application/vnd.shmocker.cache.layer.v1+tar"
	CacheLayerGzipMediaType       = "application/vnd.shmocker.cache.layer.v1+tar+gzip"
	CacheManifestMediaType        = "application/vnd.shmocker.cache.manifest.v1+json"
	CacheIndexMediaType           = "application/vnd.shmocker.cache.index.v1+json"
)

// NewRegistryCacheExporter creates a new registry cache exporter.
func NewRegistryCacheExporter(client Client, config *RegistryCacheConfig) cache.Exporter {
	if config == nil {
		config = &RegistryCacheConfig{}
	}
	if config.MaxConcurrentUploads == 0 {
		config.MaxConcurrentUploads = 5
	}
	if config.CacheVersion == "" {
		config.CacheVersion = "v1"
	}
	
	return &RegistryCacheExporter{
		client: client,
		config: config,
	}
}

// Export exports cache data to a registry.
func (e *RegistryCacheExporter) Export(ctx context.Context, req *cache.ExportRequest) (*cache.ExportResult, error) {
	if req == nil {
		return nil, errors.New("export request cannot be nil")
	}

	startTime := time.Now()
	result := &cache.ExportResult{
		Destination: e.getRepositoryRef(req.Config.Destination),
	}

	// Create cache manifest
	manifest := &CacheManifest{
		SchemaVersion: 2,
		MediaType:     CacheManifestMediaType,
		Annotations: map[string]string{
			"org.shmocker.cache.version":    e.config.CacheVersion,
			"org.shmocker.cache.created":    time.Now().Format(time.RFC3339),
			"org.shmocker.cache.source":     "shmocker",
		},
	}

	// Add metadata annotations
	if req.Config.Metadata != nil {
		for k, v := range req.Config.Metadata {
			manifest.Annotations["org.shmocker.cache.metadata."+k] = v
		}
	}

	// Create cache config
	configData := map[string]interface{}{
		"version":     e.config.CacheVersion,
		"created":     time.Now().Format(time.RFC3339),
		"compression": string(req.Compression),
		"metadata":    req.Config.Metadata,
	}

	configBytes, err := json.Marshal(configData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cache config")
	}

	// Upload config blob
	configDigest := digest.FromBytes(configBytes).String()
	configBlob := &BlobData{
		MediaType: CacheConfigMediaType,
		Digest:    configDigest,
		Size:      int64(len(configBytes)),
		Content:   io.NopCloser(bytes.NewReader(configBytes)),
	}

	if err := e.uploadBlob(ctx, configBlob); err != nil {
		return nil, errors.Wrap(err, "failed to upload config")
	}

	manifest.Config = &Descriptor{
		MediaType: CacheConfigMediaType,
		Digest:    configDigest,
		Size:      int64(len(configBytes)),
	}

	// TODO: Implement actual cache entry processing
	// This would iterate through cache entries and upload them as layers
	// For now, we'll create a placeholder implementation
	
	// Upload manifest
	manifestRef := e.getRepositoryRef(req.Config.Destination)
	_, err = json.Marshal(manifest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cache manifest")
	}

	if err := e.client.PutManifest(ctx, manifestRef, &Manifest{
		SchemaVersion: manifest.SchemaVersion,
		MediaType:     manifest.MediaType,
		Config:        manifest.Config,
		Layers:        manifest.Layers,
		Annotations:   manifest.Annotations,
		Subject:       manifest.Subject,
	}); err != nil {
		return nil, errors.Wrap(err, "failed to upload cache manifest")
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// GetSupportedTypes returns supported export types.
func (e *RegistryCacheExporter) GetSupportedTypes() []cache.ExportType {
	return []cache.ExportType{cache.ExportTypeRegistry}
}

// ValidateExportConfig validates export configuration.
func (e *RegistryCacheExporter) ValidateExportConfig(config *cache.ExportConfig) error {
	if config.Destination == "" {
		return errors.New("destination is required")
	}
	
	// Validate destination format (should be a valid registry reference)
	if !strings.Contains(config.Destination, "/") {
		return errors.New("destination must be in format registry/repository:tag")
	}
	
	return nil
}

// NewRegistryCacheImporter creates a new registry cache importer.
func NewRegistryCacheImporter(client Client, config *RegistryCacheConfig) cache.Importer {
	if config == nil {
		config = &RegistryCacheConfig{}
	}
	if config.MaxConcurrentDownloads == 0 {
		config.MaxConcurrentDownloads = 5
	}
	if config.CacheVersion == "" {
		config.CacheVersion = "v1"
	}
	
	return &RegistryCacheImporter{
		client: client,
		config: config,
	}
}

// Import imports cache data from a registry.
func (i *RegistryCacheImporter) Import(ctx context.Context, req *cache.ImportRequest) (*cache.ImportResult, error) {
	if req == nil {
		return nil, errors.New("import request cannot be nil")
	}

	startTime := time.Now()
	result := &cache.ImportResult{
		Source: req.Config.Source,
	}

	// Get cache manifest
	manifestRef := i.getRepositoryRef(req.Config.Source)
	manifest, err := i.client.GetManifest(ctx, manifestRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cache manifest")
	}

	// Validate manifest is a cache manifest
	if manifest.MediaType != CacheManifestMediaType {
		return nil, fmt.Errorf("invalid cache manifest media type: %s", manifest.MediaType)
	}

	// Get cache config
	if manifest.Config != nil {
		configData, err := i.client.GetBlob(ctx, manifestRef, manifest.Config.Digest)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get cache config")
		}
		defer configData.Close()

		configBytes, err := io.ReadAll(configData)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read cache config")
		}

		var config map[string]interface{}
		if err := json.Unmarshal(configBytes, &config); err != nil {
			return nil, errors.Wrap(err, "failed to parse cache config")
		}

		// Validate cache version compatibility
		if version, ok := config["version"].(string); ok && version != i.config.CacheVersion {
			return nil, fmt.Errorf("incompatible cache version: %s (expected %s)", version, i.config.CacheVersion)
		}
	}

	// Process cache layers
	for _, layer := range manifest.Layers {
		// Download layer data
		layerData, err := i.client.GetBlob(ctx, manifestRef, layer.Digest)
		if err != nil {
			continue // Skip missing layers
		}
		layerData.Close() // We're not actually processing the data yet

		result.ImportedEntries++
		result.ImportedSize += layer.Size
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// GetSupportedTypes returns supported import types.
func (i *RegistryCacheImporter) GetSupportedTypes() []cache.ImportType {
	return []cache.ImportType{cache.ImportTypeRegistry}
}

// ValidateImportConfig validates import configuration.
func (i *RegistryCacheImporter) ValidateImportConfig(config *cache.ImportConfig) error {
	if config.Source == "" {
		return errors.New("source is required")
	}
	
	// Validate source format (should be a valid registry reference)
	if !strings.Contains(config.Source, "/") {
		return errors.New("source must be in format registry/repository:tag")
	}
	
	return nil
}

// Helper methods

// uploadBlob uploads a blob to the registry.
func (e *RegistryCacheExporter) uploadBlob(ctx context.Context, blob *BlobData) error {
	ref := e.getRepositoryRef("")
	_, err := e.client.PutBlob(ctx, ref, blob.Content)
	return err
}

// getRepositoryRef constructs a full repository reference.
func (e *RegistryCacheExporter) getRepositoryRef(tag string) string {
	repo := e.config.Repository
	if e.config.Namespace != "" {
		repo = e.config.Namespace + "/" + repo
	}
	
	ref := e.config.Registry + "/" + repo
	if tag != "" && !strings.Contains(tag, ":") {
		ref += ":" + tag
	} else if tag != "" {
		// Tag already contains registry/repo
		ref = tag
	} else {
		ref += ":latest"
	}
	
	return ref
}

// getRepositoryRef constructs a full repository reference.
func (i *RegistryCacheImporter) getRepositoryRef(tag string) string {
	repo := i.config.Repository
	if i.config.Namespace != "" {
		repo = i.config.Namespace + "/" + repo
	}
	
	ref := i.config.Registry + "/" + repo
	if tag != "" && !strings.Contains(tag, ":") {
		ref += ":" + tag
	} else if tag != "" {
		// Tag already contains registry/repo
		ref = tag
	} else {
		ref += ":latest"
	}
	
	return ref
}

// CacheKeyToArtifactTag converts a cache key to an artifact tag.
func CacheKeyToArtifactTag(key string) string {
	// Replace invalid characters in cache key to make it a valid tag
	tag := strings.ReplaceAll(key, ":", "-")
	tag = strings.ReplaceAll(tag, "/", "-")
	tag = strings.ReplaceAll(tag, "+", "-")
	tag = strings.ToLower(tag)
	
	// Ensure tag starts with alphanumeric
	if len(tag) > 0 && !isAlphaNumeric(tag[0]) {
		tag = "cache-" + tag
	}
	
	// Truncate if too long (OCI spec limits tag length)
	if len(tag) > 128 {
		tag = tag[:128]
	}
	
	return tag
}

// ArtifactTagToCacheKey converts an artifact tag back to a cache key.
func ArtifactTagToCacheKey(tag string) string {
	// This is a best-effort conversion since the transformation is lossy
	key := strings.ReplaceAll(tag, "-", ":")
	
	// Remove cache- prefix if present
	if strings.HasPrefix(key, "cache:") {
		key = strings.TrimPrefix(key, "cache:")
	}
	
	return key
}

// isAlphaNumeric checks if a character is alphanumeric.
func isAlphaNumeric(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// CreateCacheReference creates a registry reference for cache storage.
func CreateCacheReference(registry, namespace, repository, key string) string {
	repo := repository
	if namespace != "" {
		repo = namespace + "/" + repo
	}
	
	tag := CacheKeyToArtifactTag(key)
	return fmt.Sprintf("%s/%s:%s", registry, repo, tag)
}

// ParseCacheReference parses a cache reference into components.
func ParseCacheReference(ref string) (registry, namespace, repository, key string, err error) {
	// Split registry and repo:tag
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", "", "", errors.New("invalid cache reference format")
	}
	
	registry = parts[0]
	repoTag := parts[1]
	
	// Split repo and tag
	repoParts := strings.SplitN(repoTag, ":", 2)
	if len(repoParts) != 2 {
		return "", "", "", "", errors.New("invalid cache reference format: missing tag")
	}
	
	repoPath := repoParts[0]
	tag := repoParts[1]
	
	// Extract namespace and repository
	pathParts := strings.SplitN(repoPath, "/", 2)
	if len(pathParts) == 2 {
		namespace = pathParts[0]
		repository = pathParts[1]
	} else {
		repository = pathParts[0]
	}
	
	// Convert tag back to cache key
	key = ArtifactTagToCacheKey(tag)
	
	return registry, namespace, repository, key, nil
}
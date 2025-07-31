package registry

import (
	"context"
	"strings"
	"testing"

	"github.com/shmocker/shmocker/pkg/cache"
)

func TestCacheKeyToArtifactTag(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "simple key",
			key:      "mykey",
			expected: "mykey",
		},
		{
			name:     "key with colons",
			key:      "sha256:abc123def456",
			expected: "sha256-abc123def456",
		},
		{
			name:     "key with slashes",
			key:      "build/cache/layer",
			expected: "build-cache-layer",
		},
		{
			name:     "key with plus signs",
			key:      "layer+gzip",
			expected: "layer-gzip",
		},
		{
			name:     "mixed case key",
			key:      "MyCache-Key",
			expected: "mycache-key",
		},
		{
			name:     "key starting with number",
			key:      "123key",
			expected: "123key",
		},
		{
			name:     "key starting with special char",
			key:      "_specialkey",
			expected: "cache-_specialkey",
		},
		{
			name:     "very long key",
			key:      strings.Repeat("a", 150),
			expected: strings.Repeat("a", 128), // Should be truncated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CacheKeyToArtifactTag(tt.key)
			if result != tt.expected {
				t.Errorf("CacheKeyToArtifactTag() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestArtifactTagToCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "simple tag",
			tag:      "mytag",
			expected: "mytag",
		},
		{
			name:     "tag with dashes",
			tag:      "sha256-abc123def456",
			expected: "sha256:abc123def456",
		},
		{
			name:     "tag with cache prefix",
			tag:      "cache-mykey",
			expected: "mykey",
		},
		{
			name:     "complex tag",
			tag:      "build-cache-layer",
			expected: "build:cache:layer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ArtifactTagToCacheKey(tt.tag)
			if result != tt.expected {
				t.Errorf("ArtifactTagToCacheKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCreateCacheReference(t *testing.T) {
	tests := []struct {
		name       string
		registry   string
		namespace  string
		repository string
		key        string
		expected   string
	}{
		{
			name:       "simple reference",
			registry:   "registry.example.com",
			namespace:  "",
			repository: "myapp",
			key:        "cache-key",
			expected:   "registry.example.com/myapp:cache-key",
		},
		{
			name:       "reference with namespace",
			registry:   "ghcr.io",
			namespace:  "myorg",
			repository: "myapp",
			key:        "build-cache",
			expected:   "ghcr.io/myorg/myapp:build-cache",
		},
		{
			name:       "reference with special characters",
			registry:   "localhost:5000",
			namespace:  "test",
			repository: "app",
			key:        "sha256:abc123",
			expected:   "localhost:5000/test/app:sha256-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateCacheReference(tt.registry, tt.namespace, tt.repository, tt.key)
			if result != tt.expected {
				t.Errorf("CreateCacheReference() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseCacheReference(t *testing.T) {
	tests := []struct {
		name               string
		ref                string
		expectedRegistry   string
		expectedNamespace  string
		expectedRepository string
		expectedKey        string
		expectError        bool
	}{
		{
			name:               "simple reference",
			ref:                "registry.example.com/myapp:my-cache-key",
			expectedRegistry:   "registry.example.com",
			expectedNamespace:  "",
			expectedRepository: "myapp",  
			expectedKey:        "my:cache:key",
			expectError:        false,
		},
		{
			name:               "reference with namespace",
			ref:                "ghcr.io/myorg/myapp:build-cache",
			expectedRegistry:   "ghcr.io",
			expectedNamespace:  "myorg",
			expectedRepository: "myapp",
			expectedKey:        "build:cache",
			expectError:        false,
		},
		{
			name:        "invalid reference - no tag",
			ref:         "registry.example.com/myapp",
			expectError: true,
		},
		{
			name:        "invalid reference - no repository",
			ref:         "registry.example.com",
			expectError: true,
		},
		{
			name:        "empty reference",
			ref:         "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, namespace, repository, key, err := ParseCacheReference(tt.ref)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseCacheReference() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseCacheReference() error = %v, want nil", err)
				return
			}

			if registry != tt.expectedRegistry {
				t.Errorf("ParseCacheReference() registry = %v, want %v", registry, tt.expectedRegistry)
			}
			if namespace != tt.expectedNamespace {
				t.Errorf("ParseCacheReference() namespace = %v, want %v", namespace, tt.expectedNamespace)
			}
			if repository != tt.expectedRepository {
				t.Errorf("ParseCacheReference() repository = %v, want %v", repository, tt.expectedRepository)
			}
			if key != tt.expectedKey {
				t.Errorf("ParseCacheReference() key = %v, want %v", key, tt.expectedKey)
			}
		})
	}
}

func TestNewRegistryCacheExporter(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{
		Registry:             "registry.example.com",
		Repository:           "myapp",
		MaxConcurrentUploads: 10,
	}

	exporter := NewRegistryCacheExporter(client, config)
	if exporter == nil {
		t.Fatal("NewRegistryCacheExporter() returned nil")
	}

	// Test with nil config (should use defaults)
	exporter = NewRegistryCacheExporter(client, nil)
	if exporter == nil {
		t.Fatal("NewRegistryCacheExporter() with nil config returned nil")
	}
}

func TestNewRegistryCacheImporter(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{
		Registry:               "registry.example.com",
		Repository:             "myapp",
		MaxConcurrentDownloads: 10,
	}

	importer := NewRegistryCacheImporter(client, config)
	if importer == nil {
		t.Fatal("NewRegistryCacheImporter() returned nil")
	}

	// Test with nil config (should use defaults)
	importer = NewRegistryCacheImporter(client, nil)
	if importer == nil {
		t.Fatal("NewRegistryCacheImporter() with nil config returned nil")
	}
}

func TestRegistryCacheExporter_GetSupportedTypes(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{}
	exporter := NewRegistryCacheExporter(client, config)

	types := exporter.GetSupportedTypes()
	if len(types) == 0 {
		t.Error("GetSupportedTypes() returned empty slice")
	}

	// Should support registry type
	found := false
	for _, t := range types {
		if t == cache.ExportTypeRegistry {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetSupportedTypes() does not include registry type")
	}
}

func TestRegistryCacheImporter_GetSupportedTypes(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{}
	importer := NewRegistryCacheImporter(client, config)

	types := importer.GetSupportedTypes()
	if len(types) == 0 {
		t.Error("GetSupportedTypes() returned empty slice")
	}

	// Should support registry type
	found := false
	for _, t := range types {
		if t == cache.ImportTypeRegistry {
			found = true
			break
		}
	}
	if !found {
		t.Error("GetSupportedTypes() does not include registry type")
	}
}

func TestRegistryCacheExporter_ValidateExportConfig(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{}
	exporter := NewRegistryCacheExporter(client, config)

	tests := []struct {
		name        string
		config      *cache.ExportConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &cache.ExportConfig{
				Destination: "registry.example.com/myapp:cache",
			},
			expectError: false,
		},
		{
			name: "empty destination",
			config: &cache.ExportConfig{
				Destination: "",
			},
			expectError: true,
		},
		{
			name: "invalid destination format",
			config: &cache.ExportConfig{
				Destination: "invalid-destination",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := exporter.ValidateExportConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("ValidateExportConfig() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateExportConfig() error = %v, want nil", err)
			}
		})
	}
}

func TestRegistryCacheImporter_ValidateImportConfig(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{}
	importer := NewRegistryCacheImporter(client, config)

	tests := []struct {
		name        string
		config      *cache.ImportConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: &cache.ImportConfig{
				Source: "registry.example.com/myapp:cache",
			},
			expectError: false,
		},
		{
			name: "empty source",
			config: &cache.ImportConfig{
				Source: "",
			},
			expectError: true,
		},
		{
			name: "invalid source format",
			config: &cache.ImportConfig{
				Source: "invalid-source",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := importer.ValidateImportConfig(tt.config)
			if tt.expectError && err == nil {
				t.Error("ValidateImportConfig() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("ValidateImportConfig() error = %v, want nil", err)
			}
		})
	}
}

func TestRegistryCacheExporter_Export(t *testing.T) {
	client := &mockRegistryClient{}
	config := &RegistryCacheConfig{
		Registry:   "registry.example.com",
		Repository: "myapp",
	}
	exporter := NewRegistryCacheExporter(client, config).(*RegistryCacheExporter)

	ctx := context.Background()
	req := &cache.ExportRequest{
		Type: cache.ExportTypeRegistry,
		Config: &cache.ExportConfig{
			Destination: "cache-tag",
			Metadata: map[string]string{
				"build-id": "12345",
			},
		},
		Compression: cache.CompressionTypeGzip,
	}

	result, err := exporter.Export(ctx, req)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if result == nil {
		t.Fatal("Export() returned nil result")
	}

	if result.Destination == "" {
		t.Error("Export() result missing destination")
	}

	// Test with nil request
	_, err = exporter.Export(ctx, nil)
	if err == nil {
		t.Error("Export() with nil request should return error")
	}
}

func TestRegistryCacheImporter_Import(t *testing.T) {
	// Setup mock client to return cache manifest
	manifest := &Manifest{
		SchemaVersion: 2,
		MediaType:     CacheManifestMediaType,
		Config: &Descriptor{
			MediaType: CacheConfigMediaType,
			Digest:    "sha256:config123",
			Size:      1024,
		},
		Layers: []*Descriptor{
			{
				MediaType: CacheLayerGzipMediaType,
				Digest:    "sha256:layer123",
				Size:      2048,
			},
		},
	}
	
	client := &mockCacheRegistryClient{
		mockRegistryClient: &mockRegistryClient{},
		manifest:          manifest,
	}

	config := &RegistryCacheConfig{
		Registry:   "registry.example.com",
		Repository: "myapp",
	}
	importer := NewRegistryCacheImporter(client, config).(*RegistryCacheImporter)

	ctx := context.Background()
	req := &cache.ImportRequest{
		Type: cache.ImportTypeRegistry,
		Config: &cache.ImportConfig{
			Source: "cache-tag",
		},
	}

	result, err := importer.Import(ctx, req)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result == nil {
		t.Fatal("Import() returned nil result")
	}

	if result.Source != req.Config.Source {
		t.Errorf("Import() result source = %v, want %v", result.Source, req.Config.Source)
	}

	if result.ImportedEntries == 0 {
		t.Error("Import() result should have imported entries")
	}

	// Test with nil request
	_, err = importer.Import(ctx, nil)
	if err == nil {
		t.Error("Import() with nil request should return error")
	}
}

func TestRegistryCacheExporter_getRepositoryRef(t *testing.T) {
	exporter := &RegistryCacheExporter{
		config: &RegistryCacheConfig{
			Registry:   "registry.example.com",
			Repository: "myapp",
			Namespace:  "myorg",
		},
	}

	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "simple tag",
			tag:      "latest",
			expected: "registry.example.com/myorg/myapp:latest",
		},
		{
			name:     "empty tag",
			tag:      "",
			expected: "registry.example.com/myorg/myapp:latest",
		},
		{
			name:     "full reference",
			tag:      "registry.example.com/myorg/myapp:v1.0.0",
			expected: "registry.example.com/myorg/myapp:v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getRepositoryRef(tt.tag)
			if result != tt.expected {
				t.Errorf("getRepositoryRef() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRegistryCacheImporter_getRepositoryRef(t *testing.T) {
	importer := &RegistryCacheImporter{
		config: &RegistryCacheConfig{
			Registry:   "registry.example.com",
			Repository: "myapp",
		},
	}

	tests := []struct {
		name     string
		tag      string
		expected string
	}{
		{
			name:     "simple tag",
			tag:      "latest",
			expected: "registry.example.com/myapp:latest",
		},
		{
			name:     "empty tag",
			tag:      "",
			expected: "registry.example.com/myapp:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := importer.getRepositoryRef(tt.tag)
			if result != tt.expected {
				t.Errorf("getRepositoryRef() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Enhanced mock registry client for cache testing
type mockCacheRegistryClient struct {
	*mockRegistryClient
	manifest *Manifest
}

func (m *mockCacheRegistryClient) GetManifest(ctx context.Context, ref string) (*Manifest, error) {
	if m.manifest != nil {
		return m.manifest, nil
	}
	return m.mockRegistryClient.GetManifest(ctx, ref)
}
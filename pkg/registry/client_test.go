package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
)

func TestClientImpl_parseReference(t *testing.T) {
	client := &ClientImpl{
		config: &Config{},
	}

	tests := []struct {
		name         string
		ref          string
		wantRegistry string
		wantRepo     string
		wantTag      string
		wantErr      bool
	}{
		{
			name:         "simple reference",
			ref:          "nginx",
			wantRegistry: "https://registry-1.docker.io",
			wantRepo:     "nginx",
			wantTag:      "latest",
		},
		{
			name:         "reference with tag",
			ref:          "nginx:1.21",
			wantRegistry: "https://registry-1.docker.io",
			wantRepo:     "nginx",
			wantTag:      "1.21",
		},
		{
			name:         "registry with repo and tag",
			ref:          "ghcr.io/myorg/myapp:v1.0.0",
			wantRegistry: "https://ghcr.io",
			wantRepo:     "myorg/myapp",
			wantTag:      "v1.0.0",
		},
		{
			name:         "localhost registry",
			ref:          "localhost:5000/myapp:latest",
			wantRegistry: "https://localhost:5000",
			wantRepo:     "myapp",
			wantTag:      "latest",
		},
		{
			name:    "invalid reference",
			ref:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, repo, tag, err := client.parseReference(tt.ref)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseReference() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("parseReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if registry != tt.wantRegistry {
				t.Errorf("parseReference() registry = %v, want %v", registry, tt.wantRegistry)
			}
			if repo != tt.wantRepo {
				t.Errorf("parseReference() repo = %v, want %v", repo, tt.wantRepo)
			}
			if tag != tt.wantTag {
				t.Errorf("parseReference() tag = %v, want %v", tag, tt.wantTag)
			}
		})
	}
}

func TestClientImpl_Push(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "HEAD" && strings.Contains(r.URL.Path, "/blobs/"):
			// Blob doesn't exist, return 404
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/blobs/uploads/"):
			// Start blob upload
			w.Header().Set("Location", "/v2/test/repo/blobs/uploads/uuid123")
			w.WriteHeader(http.StatusAccepted)
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/blobs/uploads/"):
			// Complete blob upload
			w.Header().Set("Location", "/v2/test/repo/blobs/sha256:abc123")
			w.WriteHeader(http.StatusCreated)
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/manifests/"):
			// Upload manifest
			w.Header().Set("Docker-Content-Digest", "sha256:manifest123")
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create auth provider
	authProvider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		tokens:      make(map[string]*Token),
		config:      &AuthConfig{},
		httpClient:  server.Client(),
	}

	// Create client with test server URL
	client := &ClientImpl{
		config: &Config{
			Insecure: true, // Allow HTTP for testing
		},
		httpClient: server.Client(),
		auth:       authProvider,
	}

	// Create test manifest and blobs
	manifest := &Manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypes.OCIManifest,
		Config: &Descriptor{
			MediaType: MediaTypes.OCIConfig,
			Digest:    "sha256:config123",
			Size:      1024,
		},
		Layers: []*Descriptor{
			{
				MediaType: MediaTypes.OCILayerGzip,
				Digest:    "sha256:layer123",
				Size:      2048,
			},
		},
	}

	blobData := []byte("test blob content")
	blob := &BlobData{
		MediaType: "application/octet-stream",
		Digest:    digest.FromBytes(blobData).String(),
		Size:      int64(len(blobData)),
		Content:   io.NopCloser(bytes.NewReader(blobData)),
	}

	pushReq := &PushRequest{
		Reference: strings.TrimPrefix(server.URL, "http://") + "/test/repo:latest",
		Manifest:  manifest,
		Blobs:     []*BlobData{blob},
	}

	ctx := context.Background()
	result, err := client.Push(ctx, pushReq)
	
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if result == nil {
		t.Fatal("Push() result is nil")
	}

	if result.Digest == "" {
		t.Error("Push() result missing digest")
	}

	if len(result.PushedBlobs) == 0 {
		t.Error("Push() result missing pushed blobs")
	}
}

func TestClientImpl_Pull(t *testing.T) {
	// Create test manifest
	manifest := &Manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypes.OCIManifest,
		Config: &Descriptor{
			MediaType: MediaTypes.OCIConfig,
			Digest:    "sha256:config123",
			Size:      1024,
		},
		Layers: []*Descriptor{
			{
				MediaType: MediaTypes.OCILayerGzip,
				Digest:    "sha256:layer123",
				Size:      2048,
			},
		},
	}

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/manifests/"):
			// Return manifest
			w.Header().Set("Content-Type", MediaTypes.OCIManifest)
			json.NewEncoder(w).Encode(manifest)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/blobs/sha256:config123"):
			// Return config blob
			config := ImageConfig{
				Architecture: "amd64",
				OS:           "linux",
			}
			json.NewEncoder(w).Encode(config)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/blobs/"):
			// Return layer blob
			w.Write([]byte("layer content"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client := &ClientImpl{
		config: &Config{
			Insecure: true,
		},
		httpClient: server.Client(),
		auth: &RegistryAuthProvider{
			credentials: make(map[string]*Credentials),
			tokens:      make(map[string]*Token),
			config:      &AuthConfig{},
			httpClient:  server.Client(),
		},
	}

	pullReq := &PullRequest{
		Reference: strings.TrimPrefix(server.URL, "http://") + "/test/repo:latest",
	}

	ctx := context.Background()
	result, err := client.Pull(ctx, pullReq)
	
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}

	if result == nil {
		t.Fatal("Pull() result is nil")
	}

	if result.Manifest == nil {
		t.Error("Pull() result missing manifest")
	}

	if result.Config == nil {
		t.Error("Pull() result missing config")
	}

	if len(result.PulledBlobs) == 0 {
		t.Error("Pull() result missing pulled blobs")
	}
}

func TestClientImpl_GetManifest(t *testing.T) {
	manifest := &Manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypes.OCIManifest,
		Layers:        []*Descriptor{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", MediaTypes.OCIManifest)
			json.NewEncoder(w).Encode(manifest)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &ClientImpl{
		config: &Config{
			Insecure: true,
		},
		httpClient: server.Client(),
		auth: &RegistryAuthProvider{
			credentials: make(map[string]*Credentials),
			tokens:      make(map[string]*Token),
			config:      &AuthConfig{},
			httpClient:  server.Client(),
		},
	}

	ctx := context.Background()
	ref := strings.TrimPrefix(server.URL, "http://") + "/test/repo:latest"
	
	result, err := client.GetManifest(ctx, ref)
	
	if err != nil {
		t.Fatalf("GetManifest() error = %v", err)
	}

	if result == nil {
		t.Fatal("GetManifest() result is nil")
	}

	if result.SchemaVersion != manifest.SchemaVersion {
		t.Error("GetManifest() schema version mismatch")
	}

	if result.MediaType != manifest.MediaType {
		t.Error("GetManifest() media type mismatch")
	}
}

func TestClientImpl_ListTags(t *testing.T) {
	tags := []string{"latest", "v1.0.0", "v1.1.0"}
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/tags/list") {
			response := struct {
				Name string   `json:"name"`
				Tags []string `json:"tags"`
			}{
				Name: "test/repo",
				Tags: tags,
			}
			json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &ClientImpl{
		config: &Config{
			Insecure: true,
		},
		httpClient: server.Client(),
		auth: &RegistryAuthProvider{
			credentials: make(map[string]*Credentials),
			tokens:      make(map[string]*Token),
			config:      &AuthConfig{},
			httpClient:  server.Client(),
		},
	}

	ctx := context.Background()
	repo := strings.TrimPrefix(server.URL, "http://") + "/test/repo"
	
	result, err := client.ListTags(ctx, repo)
	
	if err != nil {
		t.Fatalf("ListTags() error = %v", err)
	}

	if len(result) != len(tags) {
		t.Errorf("ListTags() returned %d tags, expected %d", len(result), len(tags))
	}

	for i, tag := range tags {
		if result[i] != tag {
			t.Errorf("ListTags() tag[%d] = %s, expected %s", i, result[i], tag)
		}
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config",
			config: &Config{
				URL:       "https://registry.example.com",
				Timeout:   60 * time.Second,
				UserAgent: "test-client",
			},
			wantErr: false,
		},
		{
			name: "config with certificates",
			config: &Config{
				CertFile: "/nonexistent/cert.pem",
				KeyFile:  "/nonexistent/key.pem",
			},
			wantErr: true, // Should fail with nonexistent files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config)
			
			if tt.wantErr {
				if err == nil {
					t.Error("New() expected error, got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("New() error = %v, want nil", err)
				return
			}
			
			if client == nil {
				t.Error("New() returned nil client")
			}
			
			// Clean up
			if client != nil {
				client.Close()
			}
		})
	}
}

func TestBlobStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "HEAD" && strings.Contains(r.URL.Path, "/blobs/"):
			w.Header().Set("Content-Length", "1024")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/blobs/"):
			w.Write([]byte("blob content"))
		case r.Method == "DELETE" && strings.Contains(r.URL.Path, "/blobs/"):
			w.WriteHeader(http.StatusAccepted)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &ClientImpl{
		config: &Config{
			Insecure: true,
		},
		httpClient: server.Client(),
		auth: &RegistryAuthProvider{
			credentials: make(map[string]*Credentials),
			tokens:      make(map[string]*Token),
			config:      &AuthConfig{},
			httpClient:  server.Client(),
		},
	}

	registry := server.URL
	blobStore := NewBlobStore(client, registry, "test/repo")

	ctx := context.Background()
	digest := "sha256:abc123"

	// Test Stat
	info, err := blobStore.Stat(ctx, digest)
	if err != nil {
		t.Fatalf("BlobStore.Stat() error = %v", err)
	}
	if info.Size != 1024 {
		t.Errorf("BlobStore.Stat() size = %d, want 1024", info.Size)
	}

	// Test Get
	blob, err := blobStore.Get(ctx, digest)
	if err != nil {
		t.Fatalf("BlobStore.Get() error = %v", err)
	}
	defer blob.Close()

	content, err := io.ReadAll(blob)
	if err != nil {
		t.Fatalf("Failed to read blob content: %v", err)
	}
	if string(content) != "blob content" {
		t.Errorf("BlobStore.Get() content = %s, want 'blob content'", string(content))
	}

	// Test Delete
	err = blobStore.Delete(ctx, digest)
	if err != nil {
		t.Fatalf("BlobStore.Delete() error = %v", err)
	}
}

func TestManifestStore(t *testing.T) {
	manifest := &Manifest{
		SchemaVersion: 2,
		MediaType:     MediaTypes.OCIManifest,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/manifests/"):
			w.Header().Set("Content-Type", MediaTypes.OCIManifest)
			json.NewEncoder(w).Encode(manifest)
		case r.Method == "PUT" && strings.Contains(r.URL.Path, "/manifests/"):
			w.WriteHeader(http.StatusCreated)
		case r.Method == "DELETE" && strings.Contains(r.URL.Path, "/manifests/"):
			w.WriteHeader(http.StatusAccepted)
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/tags/list"):
			response := struct {
				Name string   `json:"name"`
				Tags []string `json:"tags"`
			}{
				Name: "test/repo",
				Tags: []string{"latest", "v1.0.0"},
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := &ClientImpl{
		config: &Config{
			Insecure: true,
		},
		httpClient: server.Client(),
		auth: &RegistryAuthProvider{
			credentials: make(map[string]*Credentials),
			tokens:      make(map[string]*Token),
			config:      &AuthConfig{},
			httpClient:  server.Client(),
		},
	}

	registry := server.URL
	manifestStore := NewManifestStore(client, registry, "test/repo")

	ctx := context.Background()
	ref := "latest"

	// Test Get
	result, err := manifestStore.Get(ctx, ref)
	if err != nil {
		t.Fatalf("ManifestStore.Get() error = %v", err)
	}
	if result.SchemaVersion != manifest.SchemaVersion {
		t.Errorf("ManifestStore.Get() schema version = %d, want %d", result.SchemaVersion, manifest.SchemaVersion)
	}

	// Test Put
	err = manifestStore.Put(ctx, ref, manifest)
	if err != nil {
		t.Fatalf("ManifestStore.Put() error = %v", err)
	}

	// Test Delete
	err = manifestStore.Delete(ctx, ref)
	if err != nil {
		t.Fatalf("ManifestStore.Delete() error = %v", err)
	}

	// Test List
	tags, err := manifestStore.List(ctx)
	if err != nil {
		t.Fatalf("ManifestStore.List() error = %v", err)
	}
	expectedTags := []string{"latest", "v1.0.0"}
	if len(tags) != len(expectedTags) {
		t.Errorf("ManifestStore.List() returned %d tags, expected %d", len(tags), len(expectedTags))
	}
}
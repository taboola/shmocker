// Package registry provides OCI registry client functionality.
package registry

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// ClientImpl represents an OCI registry client implementation.
type ClientImpl struct {
	config     *Config
	httpClient *http.Client
	auth       AuthProvider
}


// BlobStoreImpl implements the BlobStore interface.
type BlobStoreImpl struct {
	client   *ClientImpl
	registry string
	repo     string
}

// ManifestStoreImpl implements the ManifestStore interface.
type ManifestStoreImpl struct {
	client   *ClientImpl
	registry string
	repo     string
}

// New creates a new registry client.
func New(config *Config) (Client, error) {
	if config == nil {
		config = &Config{
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			UserAgent:  "shmocker/1.0",
		}
	}

	// Create HTTP client with timeout and TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.Insecure,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Load client certificates if provided
	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load client certificate")
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	// Create enhanced auth provider
	authConfig := &AuthConfig{
		InsecureRegistries: []string{},
	}
	if config.Insecure {
		// Add common insecure registries if insecure mode is enabled
		authConfig.InsecureRegistries = []string{"localhost", "127.0.0.1", "0.0.0.0"}
	}
	
	authProvider, err := NewRegistryAuthProvider(authConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create auth provider")
	}

	return &ClientImpl{
		config:     config,
		httpClient: httpClient,
		auth:       authProvider,
	}, nil
}

// Push pushes an image to the registry.
func (c *ClientImpl) Push(ctx context.Context, req *PushRequest) (*PushResult, error) {
	if req == nil {
		return nil, errors.New("push request cannot be nil")
	}

	startTime := time.Now()
	result := &PushResult{
		PushedBlobs: make([]*BlobResult, 0),
	}

	// Parse registry from reference
	registryURL, repo, tag, err := c.parseReference(req.Reference)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reference")
	}

	// Upload blobs first
	for _, blob := range req.Blobs {
		blobResult, err := c.pushBlob(ctx, registryURL, repo, blob, req.ProgressCallback)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to push blob %s", blob.Digest)
		}
		result.PushedBlobs = append(result.PushedBlobs, blobResult)
		result.Size += blobResult.Size
	}

	// Upload image config
	if req.Config != nil {
		configData, err := json.Marshal(req.Config)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal config")
		}

		configBlob := &BlobData{
			MediaType: MediaTypes.OCIConfig,
			Content:   io.NopCloser(bytes.NewReader(configData)),
			Size:      int64(len(configData)),
		}
		configBlob.Digest = digest.FromBytes(configData).String()

		blobResult, err := c.pushBlob(ctx, registryURL, repo, configBlob, req.ProgressCallback)
		if err != nil {
			return nil, errors.Wrap(err, "failed to push config")
		}
		result.PushedBlobs = append(result.PushedBlobs, blobResult)
		result.Size += blobResult.Size
	}

	// Upload manifest
	manifestData, err := json.Marshal(req.Manifest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal manifest")
	}

	manifestDigest, err := c.putManifest(ctx, registryURL, repo, tag, manifestData, req.Manifest.MediaType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to push manifest")
	}

	result.Digest = manifestDigest
	result.Duration = time.Since(startTime)

	return result, nil
}

// Pull pulls an image from the registry.
func (c *ClientImpl) Pull(ctx context.Context, req *PullRequest) (*PullResult, error) {
	if req == nil {
		return nil, errors.New("pull request cannot be nil")
	}

	startTime := time.Now()
	result := &PullResult{
		PulledBlobs: make([]*BlobResult, 0),
	}

	// Parse registry from reference
	registryURL, repo, tag, err := c.parseReference(req.Reference)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reference")
	}

	// Get manifest
	manifest, err := c.getManifest(ctx, registryURL, repo, tag)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest")
	}
	result.Manifest = manifest

	// Pull config if present
	if manifest.Config != nil {
		configData, err := c.getBlob(ctx, registryURL, repo, manifest.Config.Digest)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get config")
		}
		defer configData.Close()

		configBytes, err := io.ReadAll(configData)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read config")
		}

		var config ImageConfig
		if err := json.Unmarshal(configBytes, &config); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal config")
		}
		result.Config = &config
	}

	// Pull layers
	for _, layer := range manifest.Layers {
		blobResult := &BlobResult{
			Digest:    layer.Digest,
			Size:      layer.Size,
			MediaType: layer.MediaType,
			Uploaded:  false, // This is a pull operation
		}
		result.PulledBlobs = append(result.PulledBlobs, blobResult)
		result.Size += layer.Size

		// Report progress
		if req.ProgressCallback != nil {
			req.ProgressCallback(&PullProgress{
				Action: "Pulling layer",
				ID:     layer.Digest,
				Progress: &ProgressDetail{
					Current: 0,
					Total:   layer.Size,
				},
			})
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// GetManifest retrieves an image manifest.
func (c *ClientImpl) GetManifest(ctx context.Context, ref string) (*Manifest, error) {
	registryURL, repo, tag, err := c.parseReference(ref)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reference")
	}

	return c.getManifest(ctx, registryURL, repo, tag)
}

// PutManifest uploads an image manifest.
func (c *ClientImpl) PutManifest(ctx context.Context, ref string, manifest *Manifest) error {
	registryURL, repo, tag, err := c.parseReference(ref)
	if err != nil {
		return errors.Wrap(err, "invalid reference")
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return errors.Wrap(err, "failed to marshal manifest")
	}

	_, err = c.putManifest(ctx, registryURL, repo, tag, manifestData, manifest.MediaType)
	return err
}

// GetBlob retrieves a blob from the registry.
func (c *ClientImpl) GetBlob(ctx context.Context, ref string, digest string) (io.ReadCloser, error) {
	registryURL, repo, _, err := c.parseReference(ref)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reference")
	}

	return c.getBlob(ctx, registryURL, repo, digest)
}

// PutBlob uploads a blob to the registry.
func (c *ClientImpl) PutBlob(ctx context.Context, ref string, content io.Reader) (*BlobResult, error) {
	registryURL, repo, _, err := c.parseReference(ref)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reference")
	}

	// Read content to calculate size and digest
	data, err := io.ReadAll(content)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read content")
	}

	blobData := &BlobData{
		Content: io.NopCloser(bytes.NewReader(data)),
		Size:    int64(len(data)),
		Digest:  digest.FromBytes(data).String(),
	}

	return c.pushBlob(ctx, registryURL, repo, blobData, nil)
}

// DeleteBlob deletes a blob from the registry.
func (c *ClientImpl) DeleteBlob(ctx context.Context, ref string, digest string) error {
	registryURL, repo, _, err := c.parseReference(ref)
	if err != nil {
		return errors.Wrap(err, "invalid reference")
	}

	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repo, digest)
	req, err := http.NewRequestWithContext(ctx, "DELETE", blobURL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	// Add authentication
	if err := c.addAuth(req, registryURL); err != nil {
		return errors.Wrap(err, "failed to add authentication")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete blob failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ListTags lists all tags for a repository.
func (c *ClientImpl) ListTags(ctx context.Context, repository string) ([]string, error) {
	registryURL, repo, _, err := c.parseReference(repository)
	if err != nil {
		return nil, errors.Wrap(err, "invalid reference")
	}

	tagsURL := fmt.Sprintf("%s/v2/%s/tags/list", registryURL, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", tagsURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Add authentication
	if err := c.addAuth(req, registryURL); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list tags failed with status: %d", resp.StatusCode)
	}

	var tagsResponse struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tagsResponse); err != nil {
		return nil, errors.Wrap(err, "failed to decode response")
	}

	return tagsResponse.Tags, nil
}

// GetImageConfig retrieves the image configuration.
func (c *ClientImpl) GetImageConfig(ctx context.Context, ref string) (*ImageConfig, error) {
	// Get manifest first
	manifest, err := c.GetManifest(ctx, ref)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest")
	}

	if manifest.Config == nil {
		return nil, errors.New("manifest has no config")
	}

	// Get config blob
	configData, err := c.GetBlob(ctx, ref, manifest.Config.Digest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config blob")
	}
	defer configData.Close()

	configBytes, err := io.ReadAll(configData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read config")
	}

	var config ImageConfig
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal config")
	}

	return &config, nil
}

// Close closes the registry client.
func (c *ClientImpl) Close() error {
	// Close HTTP client connections
	c.httpClient.CloseIdleConnections()
	return nil
}

// Helper methods

// parseReference parses a registry reference into components.
func (c *ClientImpl) parseReference(ref string) (registry, repo, tag string, err error) {
	if ref == "" {
		return "", "", "", errors.New("invalid reference format")
	}

	// Handle simple references like "nginx" or "nginx:latest"
	if !strings.Contains(ref, "/") {
		// No registry specified, assume Docker Hub
		registry = "registry-1.docker.io"
		repo = ref
	} else {
		parts := strings.Split(ref, "/")
		if len(parts) < 2 {
			return "", "", "", errors.New("invalid reference format")
		}

		// Extract registry (first part)
		registry = parts[0]
		if !strings.Contains(registry, ".") && !strings.Contains(registry, ":") {
			// No registry specified, assume Docker Hub
			registry = "registry-1.docker.io"
			repo = ref
		} else {
			repo = strings.Join(parts[1:], "/")
		}
	}

	// Extract tag
	if strings.Contains(repo, ":") {
		repoParts := strings.Split(repo, ":")
		repo = repoParts[0]
		tag = repoParts[1]
	} else {
		tag = "latest"
	}

	// Ensure registry has protocol
	if !strings.HasPrefix(registry, "http://") && !strings.HasPrefix(registry, "https://") {
		if c.config.Insecure {
			registry = "http://" + registry
		} else {
			registry = "https://" + registry
		}
	}

	return registry, repo, tag, nil
}

// pushBlob uploads a blob to the registry.
func (c *ClientImpl) pushBlob(ctx context.Context, registryURL, repo string, blob *BlobData, progressCallback func(*PushProgress)) (*BlobResult, error) {
	// Check if blob already exists
	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repo, blob.Digest)
	req, err := http.NewRequestWithContext(ctx, "HEAD", blobURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HEAD request")
	}

	if err := c.addAuth(req, registryURL); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	resp, err := c.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			// Blob already exists
			return &BlobResult{
				Digest:    blob.Digest,
				Size:      blob.Size,
				MediaType: blob.MediaType,
				Uploaded:  false, // Already existed
			}, nil
		}
	}

	// Start upload
	uploadURL := fmt.Sprintf("%s/v2/%s/blobs/uploads/", registryURL, repo)
	req, err = http.NewRequestWithContext(ctx, "POST", uploadURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create upload request")
	}

	if err := c.addAuth(req, registryURL); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "upload initiation failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("upload initiation failed with status: %d", resp.StatusCode)
	}

	// Get upload URL from Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return nil, errors.New("missing Location header in upload response")
	}

	// Make location absolute if it's relative
	if strings.HasPrefix(location, "/") {
		u, _ := url.Parse(registryURL)
		u.Path = location
		location = u.String()
	}

	// Upload blob data
	req, err = http.NewRequestWithContext(ctx, "PUT", location+"&digest="+blob.Digest, blob.Content)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create PUT request")
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.FormatInt(blob.Size, 10))

	if err := c.addAuth(req, registryURL); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	// Report progress
	if progressCallback != nil {
		progressCallback(&PushProgress{
			Action: "Pushing",
			ID:     blob.Digest,
			Progress: &ProgressDetail{
				Current: 0,
				Total:   blob.Size,
			},
		})
	}

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "blob upload failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("blob upload failed with status: %d", resp.StatusCode)
	}

	// Report completion
	if progressCallback != nil {
		progressCallback(&PushProgress{
			Action: "Pushed",
			ID:     blob.Digest,
			Progress: &ProgressDetail{
				Current: blob.Size,
				Total:   blob.Size,
			},
		})
	}

	return &BlobResult{
		Digest:    blob.Digest,
		Size:      blob.Size,
		MediaType: blob.MediaType,
		Location:  resp.Header.Get("Location"),
		Uploaded:  true,
	}, nil
}

// getManifest retrieves a manifest from the registry.
func (c *ClientImpl) getManifest(ctx context.Context, registryURL, repo, reference string) (*Manifest, error) {
	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, repo, reference)
	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	// Accept both OCI and Docker manifest formats
	req.Header.Set("Accept", strings.Join([]string{
		MediaTypes.OCIManifest,
		MediaTypes.DockerManifest,
		MediaTypes.OCIManifestList,
		MediaTypes.DockerManifestList,
	}, ", "))

	if err := c.addAuth(req, registryURL); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get manifest failed with status: %d", resp.StatusCode)
	}

	var manifest Manifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, errors.Wrap(err, "failed to decode manifest")
	}

	return &manifest, nil
}

// putManifest uploads a manifest to the registry.
func (c *ClientImpl) putManifest(ctx context.Context, registryURL, repo, tag string, manifestData []byte, mediaType string) (string, error) {
	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", registryURL, repo, tag)
	req, err := http.NewRequestWithContext(ctx, "PUT", manifestURL, bytes.NewReader(manifestData))
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", mediaType)
	req.Header.Set("Content-Length", strconv.Itoa(len(manifestData)))

	if err := c.addAuth(req, registryURL); err != nil {
		return "", errors.Wrap(err, "failed to add authentication")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("put manifest failed with status: %d", resp.StatusCode)
	}

	// Return the manifest digest from Docker-Content-Digest header
	manifestDigest := resp.Header.Get("Docker-Content-Digest")
	if manifestDigest == "" {
		// Calculate digest if not provided
		manifestDigest = digest.FromBytes(manifestData).String()
	}

	return manifestDigest, nil
}

// getBlob retrieves a blob from the registry.
func (c *ClientImpl) getBlob(ctx context.Context, registryURL, repo, digest string) (io.ReadCloser, error) {
	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", registryURL, repo, digest)
	req, err := http.NewRequestWithContext(ctx, "GET", blobURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	if err := c.addAuth(req, registryURL); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("get blob failed with status: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// addAuth adds authentication to the request.
func (c *ClientImpl) addAuth(req *http.Request, registryURL string) error {
	// Extract hostname from registry URL
	u, err := url.Parse(registryURL)
	if err != nil {
		return errors.Wrap(err, "invalid registry URL")
	}

	hostname := u.Host
	if u.Port() == "" {
		if u.Scheme == "https" {
			hostname += ":443"
		} else {
			hostname += ":80"
		}
	}

	// Get credentials for this registry
	creds, err := c.auth.GetCredentials(req.Context(), hostname)
	if err != nil {
		return nil // No credentials available, continue without auth
	}

	if creds.Token != "" {
		// Bearer token authentication
		req.Header.Set("Authorization", "Bearer "+creds.Token)
	} else if creds.Username != "" && creds.Password != "" {
		// Basic authentication
		req.SetBasicAuth(creds.Username, creds.Password)
	}

	// Set User-Agent
	if c.config.UserAgent != "" {
		req.Header.Set("User-Agent", c.config.UserAgent)
	}

	return nil
}


// BlobStore implementation

// NewBlobStore creates a new blob store for a registry and repository.
func NewBlobStore(client Client, registry, repo string) BlobStore {
	return &BlobStoreImpl{
		client:   client.(*ClientImpl),
		registry: registry,
		repo:     repo,
	}
}

// Stat returns information about a blob.
func (b *BlobStoreImpl) Stat(ctx context.Context, digest string) (*BlobInfo, error) {
	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", b.registry, b.repo, digest)
	req, err := http.NewRequestWithContext(ctx, "HEAD", blobURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	if err := b.client.addAuth(req, b.registry); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	resp, err := b.client.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("blob stat failed with status: %d", resp.StatusCode)
	}

	size, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	lastModified, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))

	return &BlobInfo{
		Digest:       digest,
		Size:         size,
		MediaType:    resp.Header.Get("Content-Type"),
		CreatedAt:    lastModified,
		LastModified: lastModified,
	}, nil
}

// Get retrieves a blob by digest.
func (b *BlobStoreImpl) Get(ctx context.Context, digest string) (io.ReadCloser, error) {
	return b.client.getBlob(ctx, b.registry, b.repo, digest)
}

// Put stores a blob and returns its digest.
func (b *BlobStoreImpl) Put(ctx context.Context, content io.Reader) (string, error) {
	data, err := io.ReadAll(content)
	if err != nil {
		return "", errors.Wrap(err, "failed to read content")
	}

	blobDigest := digest.FromBytes(data).String()
	blobData := &BlobData{
		Content: io.NopCloser(bytes.NewReader(data)),
		Size:    int64(len(data)),
		Digest:  blobDigest,
	}

	result, err := b.client.pushBlob(ctx, b.registry, b.repo, blobData, nil)
	if err != nil {
		return "", err
	}

	return result.Digest, nil
}

// Delete removes a blob.
func (b *BlobStoreImpl) Delete(ctx context.Context, digest string) error {
	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s", b.registry, b.repo, digest)
	req, err := http.NewRequestWithContext(ctx, "DELETE", blobURL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	if err := b.client.addAuth(req, b.registry); err != nil {
		return errors.Wrap(err, "failed to add authentication")
	}

	resp, err := b.client.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete blob failed with status: %d", resp.StatusCode)
	}

	return nil
}

// List returns all blob digests.
func (b *BlobStoreImpl) List(ctx context.Context) ([]string, error) {
	// Note: This is not a standard registry API endpoint
	// Most registries don't support listing all blobs in a repository
	return nil, errors.New("blob listing not supported by registry API")
}

// ManifestStore implementation

// NewManifestStore creates a new manifest store for a registry and repository.
func NewManifestStore(client Client, registry, repo string) ManifestStore {
	return &ManifestStoreImpl{
		client:   client.(*ClientImpl),
		registry: registry,
		repo:     repo,
	}
}

// Get retrieves a manifest by reference.
func (m *ManifestStoreImpl) Get(ctx context.Context, ref string) (*Manifest, error) {
	return m.client.getManifest(ctx, m.registry, m.repo, ref)
}

// Put stores a manifest.
func (m *ManifestStoreImpl) Put(ctx context.Context, ref string, manifest *Manifest) error {
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return errors.Wrap(err, "failed to marshal manifest")
	}

	_, err = m.client.putManifest(ctx, m.registry, m.repo, ref, manifestData, manifest.MediaType)
	return err
}

// Delete removes a manifest.
func (m *ManifestStoreImpl) Delete(ctx context.Context, ref string) error {
	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s", m.registry, m.repo, ref)
	req, err := http.NewRequestWithContext(ctx, "DELETE", manifestURL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create request")
	}

	if err := m.client.addAuth(req, m.registry); err != nil {
		return errors.Wrap(err, "failed to add authentication")
	}

	resp, err := m.client.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete manifest failed with status: %d", resp.StatusCode)
	}

	return nil
}

// List returns all manifest references.
func (m *ManifestStoreImpl) List(ctx context.Context) ([]string, error) {
	// Construct full reference for ListTags
	// Strip protocol from registry URL if present
	registry := m.registry
	registry = strings.TrimPrefix(registry, "http://")
	registry = strings.TrimPrefix(registry, "https://")
	
	fullRef := registry + "/" + m.repo
	return m.client.ListTags(ctx, fullRef)
}
// Package registry defines interfaces for container registry operations.
package registry

import (
	"context"
	"io"
	"time"
)

// Client provides the main interface for interacting with container registries.
type Client interface {
	// Push pushes an image to the registry
	Push(ctx context.Context, req *PushRequest) (*PushResult, error)
	
	// Pull pulls an image from the registry
	Pull(ctx context.Context, req *PullRequest) (*PullResult, error)
	
	// GetManifest retrieves an image manifest
	GetManifest(ctx context.Context, ref string) (*Manifest, error)
	
	// PutManifest uploads an image manifest
	PutManifest(ctx context.Context, ref string, manifest *Manifest) error
	
	// GetBlob retrieves a blob from the registry
	GetBlob(ctx context.Context, ref string, digest string) (io.ReadCloser, error)
	
	// PutBlob uploads a blob to the registry
	PutBlob(ctx context.Context, ref string, content io.Reader) (*BlobResult, error)
	
	// DeleteBlob deletes a blob from the registry
	DeleteBlob(ctx context.Context, ref string, digest string) error
	
	// ListTags lists all tags for a repository
	ListTags(ctx context.Context, repository string) ([]string, error)
	
	// GetImageConfig retrieves the image configuration
	GetImageConfig(ctx context.Context, ref string) (*ImageConfig, error)
	
	// Close closes the registry client
	Close() error
}

// AuthProvider provides authentication for registry operations.
type AuthProvider interface {
	// GetCredentials returns credentials for the given registry
	GetCredentials(ctx context.Context, registry string) (*Credentials, error)
	
	// RefreshToken refreshes an access token if needed
	RefreshToken(ctx context.Context, registry string) (*Token, error)
	
	// ClearCredentials clears cached credentials for a registry
	ClearCredentials(registry string)
}

// BlobStore provides low-level blob storage operations.
type BlobStore interface {
	// Stat returns information about a blob
	Stat(ctx context.Context, digest string) (*BlobInfo, error)
	
	// Get retrieves a blob by digest
	Get(ctx context.Context, digest string) (io.ReadCloser, error)
	
	// Put stores a blob and returns its digest
	Put(ctx context.Context, content io.Reader) (string, error)
	
	// Delete removes a blob
	Delete(ctx context.Context, digest string) error
	
	// List returns all blob digests
	List(ctx context.Context) ([]string, error)
}

// ManifestStore provides manifest storage operations.
type ManifestStore interface {
	// Get retrieves a manifest by reference
	Get(ctx context.Context, ref string) (*Manifest, error)
	
	// Put stores a manifest
	Put(ctx context.Context, ref string, manifest *Manifest) error
	
	// Delete removes a manifest
	Delete(ctx context.Context, ref string) error
	
	// List returns all manifest references
	List(ctx context.Context) ([]string, error)
}

// Config represents registry configuration.
type Config struct {
	// URL is the registry URL
	URL string `json:"url"`
	
	// Username for authentication
	Username string `json:"username,omitempty"`
	
	// Password for authentication
	Password string `json:"password,omitempty"`
	
	// Token for bearer authentication
	Token string `json:"token,omitempty"`
	
	// Insecure allows insecure connections
	Insecure bool `json:"insecure,omitempty"`
	
	// Timeout for registry operations
	Timeout time.Duration `json:"timeout,omitempty"`
	
	// UserAgent for HTTP requests
	UserAgent string `json:"user_agent,omitempty"`
	
	// MaxRetries for failed operations
	MaxRetries int `json:"max_retries,omitempty"`
	
	// CertFile for client certificate authentication
	CertFile string `json:"cert_file,omitempty"`
	
	// KeyFile for client certificate authentication
	KeyFile string `json:"key_file,omitempty"`
	
	// CAFile for custom CA certificates
	CAFile string `json:"ca_file,omitempty"`
}

// PushRequest represents a request to push an image.
type PushRequest struct {
	// Reference is the image reference to push to
	Reference string `json:"reference"`
	
	// Manifest is the image manifest to push
	Manifest *Manifest `json:"manifest"`
	
	// Blobs contains the blobs to push
	Blobs []*BlobData `json:"blobs"`
	
	// Config contains the image configuration
	Config *ImageConfig `json:"config"`
	
	// Auth provides authentication information
	Auth *Credentials `json:"auth,omitempty"`
	
	// ProgressCallback receives progress updates
	ProgressCallback func(*PushProgress) `json:"-"`
}

// PushResult represents the result of a push operation.
type PushResult struct {
	// Digest is the digest of the pushed manifest
	Digest string `json:"digest"`
	
	// Size is the total size pushed
	Size int64 `json:"size"`
	
	// Duration is how long the push took
	Duration time.Duration `json:"duration"`
	
	// PushedBlobs contains information about pushed blobs
	PushedBlobs []*BlobResult `json:"pushed_blobs"`
}

// PullRequest represents a request to pull an image.
type PullRequest struct {
	// Reference is the image reference to pull
	Reference string `json:"reference"`
	
	// Platform specifies the target platform
	Platform string `json:"platform,omitempty"`
	
	// Auth provides authentication information
	Auth *Credentials `json:"auth,omitempty"`
	
	// ProgressCallback receives progress updates
	ProgressCallback func(*PullProgress) `json:"-"`
}

// PullResult represents the result of a pull operation.
type PullResult struct {
	// Manifest is the pulled image manifest
	Manifest *Manifest `json:"manifest"`
	
	// Config is the image configuration
	Config *ImageConfig `json:"config"`
	
	// Size is the total size pulled
	Size int64 `json:"size"`
	
	// Duration is how long the pull took
	Duration time.Duration `json:"duration"`
	
	// PulledBlobs contains information about pulled blobs
	PulledBlobs []*BlobResult `json:"pulled_blobs"`
}

// Manifest represents an OCI image manifest.
type Manifest struct {
	// SchemaVersion is the manifest schema version
	SchemaVersion int `json:"schemaVersion"`
	
	// MediaType is the manifest media type
	MediaType string `json:"mediaType"`
	
	// Config is the image configuration descriptor
	Config *Descriptor `json:"config"`
	
	// Layers contains the layer descriptors
	Layers []*Descriptor `json:"layers"`
	
	// Annotations contains arbitrary metadata
	Annotations map[string]string `json:"annotations,omitempty"`
	
	// Subject points to another manifest (for attestations)
	Subject *Descriptor `json:"subject,omitempty"`
}

// Descriptor represents an OCI descriptor.
type Descriptor struct {
	// MediaType is the media type of the content
	MediaType string `json:"mediaType"`
	
	// Digest is the content digest
	Digest string `json:"digest"`
	
	// Size is the content size in bytes
	Size int64 `json:"size"`
	
	// URLs contains alternate URLs for the content
	URLs []string `json:"urls,omitempty"`
	
	// Annotations contains arbitrary metadata
	Annotations map[string]string `json:"annotations,omitempty"`
	
	// Platform specifies the target platform
	Platform *Platform `json:"platform,omitempty"`
}

// Platform represents a target platform.
type Platform struct {
	// Architecture is the CPU architecture
	Architecture string `json:"architecture"`
	
	// OS is the operating system
	OS string `json:"os"`
	
	// OSVersion is the OS version
	OSVersion string `json:"os.version,omitempty"`
	
	// OSFeatures lists required OS features
	OSFeatures []string `json:"os.features,omitempty"`
	
	// Variant is the CPU variant
	Variant string `json:"variant,omitempty"`
}

// ImageConfig represents an OCI image configuration.
type ImageConfig struct {
	// Architecture is the CPU architecture
	Architecture string `json:"architecture"`
	
	// OS is the operating system
	OS string `json:"os"`
	
	// Config contains the execution configuration
	Config *ContainerConfig `json:"config"`
	
	// RootFS contains the root filesystem description
	RootFS *RootFS `json:"rootfs"`
	
	// History contains the layer history
	History []*HistoryEntry `json:"history,omitempty"`
	
	// Created is when the image was created
	Created *time.Time `json:"created,omitempty"`
	
	// Author is the image author
	Author string `json:"author,omitempty"`
}

// ContainerConfig represents container execution configuration.
type ContainerConfig struct {
	// User specifies the user to run as
	User string `json:"User,omitempty"`
	
	// ExposedPorts lists exposed ports
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	
	// Env contains environment variables
	Env []string `json:"Env,omitempty"`
	
	// Entrypoint is the container entrypoint
	Entrypoint []string `json:"Entrypoint,omitempty"`
	
	// Cmd is the default command
	Cmd []string `json:"Cmd,omitempty"`
	
	// Volumes lists volume mount points
	Volumes map[string]struct{} `json:"Volumes,omitempty"`
	
	// WorkingDir is the working directory
	WorkingDir string `json:"WorkingDir,omitempty"`
	
	// Labels contains arbitrary metadata
	Labels map[string]string `json:"Labels,omitempty"`
	
	// StopSignal is the signal to stop the container
	StopSignal string `json:"StopSignal,omitempty"`
}

// RootFS represents the root filesystem description.
type RootFS struct {
	// Type is the rootfs type (usually "layers")
	Type string `json:"type"`
	
	// DiffIDs contains the layer diff IDs
	DiffIDs []string `json:"diff_ids"`
}

// HistoryEntry represents a single layer in the image history.
type HistoryEntry struct {
	// Created is when this layer was created
	Created *time.Time `json:"created,omitempty"`
	
	// CreatedBy is the command that created this layer
	CreatedBy string `json:"created_by,omitempty"`
	
	// Author is who created this layer
	Author string `json:"author,omitempty"`
	
	// Comment is an arbitrary comment
	Comment string `json:"comment,omitempty"`
	
	// EmptyLayer indicates if this is an empty layer
	EmptyLayer bool `json:"empty_layer,omitempty"`
}

// BlobData represents blob content and metadata.
type BlobData struct {
	// Digest is the blob digest
	Digest string `json:"digest"`
	
	// Size is the blob size
	Size int64 `json:"size"`
	
	// MediaType is the blob media type
	MediaType string `json:"media_type"`
	
	// Content provides access to the blob data
	Content io.ReadCloser `json:"-"`
}

// BlobResult represents the result of a blob operation.
type BlobResult struct {
	// Digest is the blob digest
	Digest string `json:"digest"`
	
	// Size is the blob size
	Size int64 `json:"size"`
	
	// MediaType is the blob media type
	MediaType string `json:"media_type"`
	
	// Location is the blob location URL
	Location string `json:"location,omitempty"`
	
	// Uploaded indicates if the blob was uploaded
	Uploaded bool `json:"uploaded"`
}

// BlobInfo provides information about a blob.
type BlobInfo struct {
	// Digest is the blob digest
	Digest string `json:"digest"`
	
	// Size is the blob size
	Size int64 `json:"size"`
	
	// MediaType is the blob media type
	MediaType string `json:"media_type"`
	
	// CreatedAt is when the blob was created
	CreatedAt time.Time `json:"created_at"`
	
	// LastModified is when the blob was last modified
	LastModified time.Time `json:"last_modified"`
}

// Credentials represents authentication credentials.
type Credentials struct {
	// Username for basic authentication
	Username string `json:"username,omitempty"`
	
	// Password for basic authentication
	Password string `json:"password,omitempty"`
	
	// Token for bearer authentication
	Token string `json:"token,omitempty"`
	
	// RefreshToken for token refresh
	RefreshToken string `json:"refresh_token,omitempty"`
	
	// Registry is the registry hostname
	Registry string `json:"registry"`
	
	// ExpiresAt is when the token expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Token represents an authentication token.
type Token struct {
	// AccessToken is the access token
	AccessToken string `json:"access_token"`
	
	// RefreshToken is the refresh token
	RefreshToken string `json:"refresh_token,omitempty"`
	
	// TokenType is the token type (usually "Bearer")
	TokenType string `json:"token_type,omitempty"`
	
	// ExpiresIn is the token lifetime in seconds
	ExpiresIn int `json:"expires_in,omitempty"`
	
	// Scope is the token scope
	Scope string `json:"scope,omitempty"`
}

// PushProgress represents push operation progress.
type PushProgress struct {
	// Action describes the current action
	Action string `json:"action"`
	
	// ID identifies the layer or component
	ID string `json:"id,omitempty"`
	
	// Progress contains progress details
	Progress *ProgressDetail `json:"progress,omitempty"`
	
	// Status provides additional status information
	Status string `json:"status,omitempty"`
}

// PullProgress represents pull operation progress.
type PullProgress struct {
	// Action describes the current action
	Action string `json:"action"`
	
	// ID identifies the layer or component
	ID string `json:"id,omitempty"`
	
	// Progress contains progress details
	Progress *ProgressDetail `json:"progress,omitempty"`
	
	// Status provides additional status information
	Status string `json:"status,omitempty"`
}

// ProgressDetail provides detailed progress information.
type ProgressDetail struct {
	// Current is the current progress
	Current int64 `json:"current"`
	
	// Total is the total amount
	Total int64 `json:"total"`
	
	// StartedAt is when the operation started
	StartedAt *time.Time `json:"started_at,omitempty"`
}

// MediaTypes contains common OCI media types.
var MediaTypes = struct {
	// Manifest media types
	OCIManifest       string
	DockerManifest    string
	OCIManifestList   string
	DockerManifestList string
	
	// Config media types
	OCIConfig       string
	DockerConfig    string
	
	// Layer media types
	OCILayer          string
	DockerLayer       string
	OCILayerGzip      string
	DockerLayerGzip   string
	
	// Foreign layer media types
	DockerForeignLayer string
}{
	OCIManifest:       "application/vnd.oci.image.manifest.v1+json",
	DockerManifest:    "application/vnd.docker.distribution.manifest.v2+json",
	OCIManifestList:   "application/vnd.oci.image.index.v1+json",
	DockerManifestList: "application/vnd.docker.distribution.manifest.list.v2+json",
	
	OCIConfig:         "application/vnd.oci.image.config.v1+json",
	DockerConfig:      "application/vnd.docker.container.image.v1+json",
	
	OCILayer:          "application/vnd.oci.image.layer.v1.tar",
	DockerLayer:       "application/vnd.docker.image.rootfs.diff.tar",
	OCILayerGzip:      "application/vnd.oci.image.layer.v1.tar+gzip",
	DockerLayerGzip:   "application/vnd.docker.image.rootfs.diff.tar.gzip",
	
	DockerForeignLayer: "application/vnd.docker.image.rootfs.foreign.diff.tar.gzip",
}
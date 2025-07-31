// Package builder defines interfaces for container image building functionality.
package builder

import (
	"context"
	"time"

	"github.com/shmocker/shmocker/pkg/cache"
	"github.com/shmocker/shmocker/pkg/dockerfile"
	"github.com/shmocker/shmocker/pkg/registry"
)

// Builder provides the main interface for building container images using BuildKit.
// It orchestrates the entire build process from parsing to final image creation.
type Builder interface {
	// Build executes a complete image build workflow
	Build(ctx context.Context, req *BuildRequest) (*BuildResult, error)

	// BuildWithProgress executes a build with progress reporting
	BuildWithProgress(ctx context.Context, req *BuildRequest, progress chan<- *ProgressEvent) (*BuildResult, error)

	// Close cleans up resources used by the builder
	Close() error
}

// BuildKitController provides the interface for interacting with BuildKit as an embedded library.
// This abstracts BuildKit's solver and execution capabilities.
type BuildKitController interface {
	// Solve executes a BuildKit solve operation with the provided definition
	Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error)

	// ImportCache imports build cache from external sources
	ImportCache(ctx context.Context, imports []*CacheImport) error

	// ExportCache exports build cache to external destinations
	ExportCache(ctx context.Context, exports []*CacheExport) error

	// GetSession returns the current BuildKit session for advanced operations
	GetSession(ctx context.Context) (Session, error)

	// Close shuts down the BuildKit controller
	Close() error
}

// Worker represents the BuildKit worker interface for rootless execution.
type Worker interface {
	// GetWorkerController returns the underlying worker controller
	GetWorkerController() WorkerController

	// Platforms returns the platforms supported by this worker
	Platforms() []Platform

	// Executor returns the executor for running build steps
	Executor() Executor

	// CacheManager returns the cache manager for this worker
	CacheManager() cache.Manager
}

// WorkerController provides low-level worker control operations.
type WorkerController interface {
	// GetDefault returns the default worker
	GetDefault() (Worker, error)

	// List returns all available workers
	List() ([]Worker, error)
}

// Executor handles the execution of individual build steps within the rootless environment.
type Executor interface {
	// Run executes a single build step
	Run(ctx context.Context, step *ExecutionStep) (*ExecutionResult, error)

	// Prepare prepares the execution environment
	Prepare(ctx context.Context, spec *ExecutionSpec) error

	// Cleanup cleans up after execution
	Cleanup(ctx context.Context) error
}

// Platform represents a target platform for multi-arch builds.
type Platform struct {
	OS           string
	Architecture string
	Variant      string
}

// String returns the platform string in the format "os/arch[/variant]"
func (p Platform) String() string {
	if p.Variant != "" {
		return p.OS + "/" + p.Architecture + "/" + p.Variant
	}
	return p.OS + "/" + p.Architecture
}

// Session represents a BuildKit session for managing build state.
type Session interface {
	// ID returns the session identifier
	ID() string

	// Run starts the session
	Run(ctx context.Context) error

	// Close ends the session
	Close() error
}

// ProgressReporter defines the interface for build progress reporting.
type ProgressReporter interface {
	// ReportProgress sends a progress update
	ReportProgress(event *ProgressEvent)

	// SetTotal sets the total number of steps
	SetTotal(total int)

	// Close finalizes progress reporting
	Close()
}

// ProgressEvent represents a single progress update during the build process.
type ProgressEvent struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	Status    ProgressStatus         `json:"status"`
	Progress  *ProgressDetail        `json:"progress,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Error     string                 `json:"error,omitempty"`
	Stream    string                 `json:"stream,omitempty"`
	Aux       map[string]interface{} `json:"aux,omitempty"`
}

// ProgressStatus represents the status of a build step.
type ProgressStatus string

const (
	StatusStarted   ProgressStatus = "started"
	StatusRunning   ProgressStatus = "running"
	StatusCompleted ProgressStatus = "completed"
	StatusError     ProgressStatus = "error"
	StatusCanceled  ProgressStatus = "canceled"
)

// ProgressDetail provides detailed progress information.
type ProgressDetail struct {
	Current int64 `json:"current,omitempty"`
	Total   int64 `json:"total,omitempty"`
}

// BuildRequest represents a complete build request with all necessary parameters.
type BuildRequest struct {
	// Context configuration
	Context    BuildContext    `json:"context"`
	Dockerfile *dockerfile.AST `json:"dockerfile"`

	// Build parameters
	Tags      []string          `json:"tags,omitempty"`
	Target    string            `json:"target,omitempty"`
	Platforms []Platform        `json:"platforms,omitempty"`
	BuildArgs map[string]string `json:"build_args,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`

	// Cache configuration
	CacheFrom []*CacheImport `json:"cache_from,omitempty"`
	CacheTo   []*CacheExport `json:"cache_to,omitempty"`
	NoCache   bool           `json:"no_cache,omitempty"`

	// Output configuration
	Output *OutputConfig `json:"output,omitempty"`

	// Security and compliance
	GenerateSBOM bool `json:"generate_sbom,omitempty"`
	SignImage    bool `json:"sign_image,omitempty"`

	// Advanced options
	Pull        bool         `json:"pull,omitempty"`
	Secrets     []*Secret    `json:"secrets,omitempty"`
	SSH         []*SSHConfig `json:"ssh,omitempty"`
	NetworkMode string       `json:"network_mode,omitempty"`
}

// BuildResult contains the results of a successful build operation.
type BuildResult struct {
	// Image information
	ImageID     string           `json:"image_id"`
	ImageDigest string           `json:"image_digest"`
	Manifests   []*ImageManifest `json:"manifests"`

	// Build metadata
	BuildTime   time.Duration `json:"build_time"`
	CacheHits   int           `json:"cache_hits"`
	CacheMisses int           `json:"cache_misses"`

	// Security artifacts
	SBOM      *SBOMData      `json:"sbom,omitempty"`
	Signature *SignatureData `json:"signature,omitempty"`

	// Export results
	ExportedCache []*CacheExport `json:"exported_cache,omitempty"`
}

// BuildContext represents the build context for an image build.
type BuildContext struct {
	Type         ContextType `json:"type"`
	Source       string      `json:"source"`
	Include      []string    `json:"include,omitempty"`
	Exclude      []string    `json:"exclude,omitempty"`
	DockerIgnore bool        `json:"docker_ignore,omitempty"`
}

// ContextType defines the type of build context.
type ContextType string

const (
	ContextTypeLocal ContextType = "local"
	ContextTypeGit   ContextType = "git"
	ContextTypeTar   ContextType = "tar"
	ContextTypeStdin ContextType = "stdin"
	ContextTypeHTTP  ContextType = "http"
)

// OutputConfig defines where and how to output the built image.
type OutputConfig struct {
	Type        OutputType       `json:"type"`
	Destination string           `json:"destination,omitempty"`
	Push        bool             `json:"push,omitempty"`
	Registry    *registry.Config `json:"registry,omitempty"`
}

// OutputType defines the output format for built images.
type OutputType string

const (
	OutputTypeRegistry OutputType = "registry"
	OutputTypeLocal    OutputType = "local"
	OutputTypeTar      OutputType = "tar"
	OutputTypeOCI      OutputType = "oci"
)

// Secret represents build-time secrets.
type Secret struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target,omitempty"`
}

// SSHConfig represents SSH agent configuration for builds.
type SSHConfig struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths,omitempty"`
}

// SolveDefinition represents a BuildKit solve definition.
type SolveDefinition struct {
	Definition []byte            `json:"definition"`
	Frontend   string            `json:"frontend"`
	Metadata   map[string][]byte `json:"metadata,omitempty"`
}

// SolveResult represents the result of a BuildKit solve operation.
type SolveResult struct {
	Ref      string            `json:"ref"`
	Metadata map[string][]byte `json:"metadata,omitempty"`
}

// ExecutionStep represents a single step in the build execution.
type ExecutionStep struct {
	ID         string   `json:"id"`
	Command    []string `json:"command"`
	Env        []string `json:"env,omitempty"`
	WorkingDir string   `json:"working_dir,omitempty"`
	User       string   `json:"user,omitempty"`
	Mounts     []*Mount `json:"mounts,omitempty"`
}

// ExecutionResult represents the result of executing a build step.
type ExecutionResult struct {
	ExitCode int           `json:"exit_code"`
	Stdout   []byte        `json:"stdout,omitempty"`
	Stderr   []byte        `json:"stderr,omitempty"`
	Duration time.Duration `json:"duration"`
}

// ExecutionSpec defines the specification for build step execution.
type ExecutionSpec struct {
	Platform    Platform          `json:"platform"`
	Constraints map[string]string `json:"constraints,omitempty"`
}

// Mount represents a mount point in build execution.
type Mount struct {
	Type    MountType `json:"type"`
	Source  string    `json:"source,omitempty"`
	Target  string    `json:"target"`
	Options []string  `json:"options,omitempty"`
}

// MountType defines the type of mount.
type MountType string

const (
	MountTypeBind   MountType = "bind"
	MountTypeCache  MountType = "cache"
	MountTypeTmpfs  MountType = "tmpfs"
	MountTypeSecret MountType = "secret"
	MountTypeSSH    MountType = "ssh"
)

// CacheImport represents cache import configuration.
type CacheImport struct {
	Type  string            `json:"type"`
	Ref   string            `json:"ref"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// CacheExport represents cache export configuration.
type CacheExport struct {
	Type  string            `json:"type"`
	Ref   string            `json:"ref"`
	Attrs map[string]string `json:"attrs,omitempty"`
}

// ImageManifest represents an OCI image manifest.
type ImageManifest struct {
	MediaType     string            `json:"mediaType"`
	SchemaVersion int               `json:"schemaVersion"`
	Config        *Descriptor       `json:"config"`
	Layers        []*Descriptor     `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	Platform      *Platform         `json:"platform,omitempty"`
}

// Descriptor represents an OCI descriptor.
type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	URLs        []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// SBOMData represents Software Bill of Materials data.
type SBOMData struct {
	Format  string `json:"format"`
	Version string `json:"version"`
	Data    []byte `json:"data"`
	Digest  string `json:"digest"`
}

// SignatureData represents image signature data.
type SignatureData struct {
	Algorithm string `json:"algorithm"`
	Signature []byte `json:"signature"`
	KeyID     string `json:"key_id,omitempty"`
	Digest    string `json:"digest"`
}

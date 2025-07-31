//go:build !linux
// +build !linux

package builder

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// builder implements the Builder interface using a stub implementation
type builder struct {
	controller BuildKitController
	options    *BuilderOptions
}

// BuilderOptions contains configuration options for the builder
type BuilderOptions struct {
	Root     string
	DataRoot string
	Debug    bool
}

// BuildKitOptions contains configuration options for BuildKit controller
type BuildKitOptions struct {
	Root     string
	DataRoot string
	Debug    bool
}

// New creates a new Builder instance with stub BuildKit
func New(ctx context.Context, opts *BuilderOptions) (Builder, error) {
	if opts == nil {
		opts = &BuilderOptions{}
	}

	// Create BuildKit controller
	controller, err := NewBuildKitController(ctx, &BuildKitOptions{
		Root:     opts.Root,
		DataRoot: opts.DataRoot,
		Debug:    opts.Debug,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create BuildKit controller")
	}

	return &builder{
		controller: controller,
		options:    opts,
	}, nil
}

// Build executes a complete image build workflow (stub implementation)
func (b *builder) Build(ctx context.Context, req *BuildRequest) (*BuildResult, error) {
	if req == nil {
		return nil, errors.New("build request cannot be nil")
	}

	startTime := time.Now()

	// Validate build request
	if err := b.validateBuildRequest(req); err != nil {
		return nil, errors.Wrap(err, "invalid build request")
	}

	fmt.Printf("STUB: Building image from context: %s\n", req.Context.Source)
	fmt.Printf("STUB: Tags: %v\n", req.Tags)
	fmt.Printf("STUB: Target: %s\n", req.Target)
	fmt.Printf("STUB: Platforms: %v\n", req.Platforms)

	// Simulate some work
	time.Sleep(100 * time.Millisecond)

	// Create mock LLB definition
	def := &SolveDefinition{
		Definition: []byte(fmt.Sprintf("mock-dockerfile-content-for-%s", req.Context.Source)),
		Frontend:   "dockerfile.v0",
		Metadata:   make(map[string][]byte),
	}

	// Execute solve
	result, err := b.controller.Solve(ctx, def)
	if err != nil {
		return nil, errors.Wrap(err, "build failed")
	}

	// Process results
	buildResult := &BuildResult{
		ImageID:     result.Ref,
		BuildTime:   time.Since(startTime),
		CacheHits:   0,
		CacheMisses: 0,
	}

	// Handle cache export if specified
	if len(req.CacheTo) > 0 {
		if err := b.controller.ExportCache(ctx, req.CacheTo); err != nil {
			return nil, errors.Wrap(err, "failed to export cache")
		}
		buildResult.ExportedCache = req.CacheTo
	}

	return buildResult, nil
}

// BuildWithProgress executes a build with progress reporting (stub implementation)
func (b *builder) BuildWithProgress(ctx context.Context, req *BuildRequest, progress chan<- *ProgressEvent) (*BuildResult, error) {
	if progress == nil {
		return b.Build(ctx, req)
	}

	// Report build start
	progress <- &ProgressEvent{
		ID:        "build-start",
		Name:      "Starting build (stub)",
		Status:    StatusStarted,
		Timestamp: time.Now(),
	}

	// Simulate progress steps
	steps := []string{"parsing", "resolving", "building", "exporting"}
	for i, step := range steps {
		progress <- &ProgressEvent{
			ID:     fmt.Sprintf("step-%d", i+1),
			Name:   fmt.Sprintf("Step %d: %s", i+1, step),
			Status: StatusRunning,
			Progress: &ProgressDetail{
				Current: int64(i + 1),
				Total:   int64(len(steps)),
			},
			Timestamp: time.Now(),
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Execute build
	result, err := b.Build(ctx, req)

	// Report completion or error
	if err != nil {
		progress <- &ProgressEvent{
			ID:        "build-error",
			Name:      "Build failed",
			Status:    StatusError,
			Error:     err.Error(),
			Timestamp: time.Now(),
		}
		return nil, err
	}

	progress <- &ProgressEvent{
		ID:        "build-complete",
		Name:      "Build completed (stub)",
		Status:    StatusCompleted,
		Timestamp: time.Now(),
	}

	return result, nil
}

// Close cleans up resources used by the builder
func (b *builder) Close() error {
	if b.controller != nil {
		return b.controller.Close()
	}
	return nil
}

// validateBuildRequest validates the build request parameters
func (b *builder) validateBuildRequest(req *BuildRequest) error {
	if req.Dockerfile == nil {
		return errors.New("dockerfile AST is required")
	}

	if req.Context.Type == "" {
		return errors.New("build context type is required")
	}

	if req.Context.Source == "" {
		return errors.New("build context source is required")
	}

	// Validate platforms
	for _, platform := range req.Platforms {
		if platform.OS == "" || platform.Architecture == "" {
			return errors.New("platform OS and architecture are required")
		}
	}

	return nil
}

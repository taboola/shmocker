//go:build linux
// +build linux

// Package builder provides core image building functionality for shmocker.
package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
	"github.com/tonistiigi/fsutil"
	"golang.org/x/sync/errgroup"

	"github.com/shmocker/shmocker/pkg/dockerfile"
)

// builder implements the Builder interface using embedded BuildKit
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

// New creates a new Builder instance with embedded BuildKit
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

// Build executes a complete image build workflow
func (b *builder) Build(ctx context.Context, req *BuildRequest) (*BuildResult, error) {
	if req == nil {
		return nil, errors.New("build request cannot be nil")
	}

	startTime := time.Now()

	// Validate build request
	if err := b.validateBuildRequest(req); err != nil {
		return nil, errors.Wrap(err, "invalid build request")
	}

	// Prepare build context
	buildContext, err := b.prepareBuildContext(ctx, &req.Context)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare build context")
	}
	defer buildContext.Close()

	// Convert Dockerfile AST to LLB definition
	def, err := b.generateLLBDefinition(ctx, req, buildContext)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate LLB definition")
	}

	// Execute build
	result, err := b.controller.Solve(ctx, def)
	if err != nil {
		return nil, errors.Wrap(err, "build failed")
	}

	// Process results
	buildResult := &BuildResult{
		ImageID:     result.Ref,
		BuildTime:   time.Since(startTime),
		CacheHits:   0, // TODO: Extract from build metadata
		CacheMisses: 0, // TODO: Extract from build metadata
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

// BuildWithProgress executes a build with progress reporting
func (b *builder) BuildWithProgress(ctx context.Context, req *BuildRequest, progress chan<- *ProgressEvent) (*BuildResult, error) {
	if progress == nil {
		return b.Build(ctx, req)
	}

	// Create progress reporter
	reporter := &progressReporter{
		ch:    progress,
		total: 0, // Will be updated as we discover build steps
	}
	defer reporter.Close()

	// Report build start
	reporter.ReportProgress(&ProgressEvent{
		ID:        "build-start",
		Name:      "Starting build",
		Status:    StatusStarted,
		Timestamp: time.Now(),
	})

	// Execute build
	result, err := b.Build(ctx, req)

	// Report completion or error
	if err != nil {
		reporter.ReportProgress(&ProgressEvent{
			ID:        "build-error",
			Name:      "Build failed",
			Status:    StatusError,
			Error:     err.Error(),
			Timestamp: time.Now(),
		})
		return nil, err
	}

	reporter.ReportProgress(&ProgressEvent{
		ID:        "build-complete",
		Name:      "Build completed",
		Status:    StatusCompleted,
		Timestamp: time.Now(),
	})

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

// prepareBuildContext prepares the build context based on the context type
func (b *builder) prepareBuildContext(ctx context.Context, buildCtx *BuildContext) (*buildContextManager, error) {
	switch buildCtx.Type {
	case ContextTypeLocal:
		return b.prepareLocalContext(ctx, buildCtx)
	case ContextTypeGit:
		return nil, errors.New("git context not yet implemented")
	case ContextTypeTar:
		return nil, errors.New("tar context not yet implemented")
	case ContextTypeHTTP:
		return nil, errors.New("HTTP context not yet implemented")
	default:
		return nil, errors.Errorf("unsupported context type: %s", buildCtx.Type)
	}
}

// prepareLocalContext prepares a local directory build context
func (b *builder) prepareLocalContext(ctx context.Context, buildCtx *BuildContext) (*buildContextManager, error) {
	absPath, err := filepath.Abs(buildCtx.Source)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get absolute path")
	}

	// Verify directory exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, errors.Wrap(err, "build context directory does not exist")
	}

	// Create fsutil sync provider for the local directory
	// This will handle .dockerignore files and provide efficient file access
	var excludes []string
	if buildCtx.DockerIgnore {
		// TODO: Parse .dockerignore file if it exists
	}
	excludes = append(excludes, buildCtx.Exclude...)

	return &buildContextManager{
		contextType: ContextTypeLocal,
		source:      absPath,
		excludes:    excludes,
	}, nil
}

// generateLLBDefinition converts the Dockerfile AST to LLB definition
func (b *builder) generateLLBDefinition(ctx context.Context, req *BuildRequest, buildCtx *buildContextManager) (*SolveDefinition, error) {
	// Use BuildKit's Dockerfile frontend to convert to LLB
	frontendAttrs := map[string][]byte{
		"filename": []byte("Dockerfile"),
	}

	// Add build args
	for k, v := range req.BuildArgs {
		frontendAttrs["build-arg:"+k] = []byte(v)
	}

	// Add labels
	for k, v := range req.Labels {
		frontendAttrs["label:"+k] = []byte(v)
	}

	// Add target if specified
	if req.Target != "" {
		frontendAttrs["target"] = []byte(req.Target)
	}

	// Add platform information
	if len(req.Platforms) > 0 {
		platform := req.Platforms[0] // Use first platform for single-platform builds
		frontendAttrs["platform"] = []byte(platform.String())
	}

	// Convert Dockerfile AST to bytes (this would need to be implemented)
	dockerfileContent, err := b.dockerfileASTToBytes(req.Dockerfile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize Dockerfile AST")
	}

	return &SolveDefinition{
		Definition: dockerfileContent,
		Frontend:   "dockerfile.v0",
		Metadata:   frontendAttrs,
	}, nil
}

// dockerfileASTToBytes converts a Dockerfile AST back to byte representation
func (b *builder) dockerfileASTToBytes(ast *dockerfile.AST) ([]byte, error) {
	// TODO: Implement AST to bytes conversion
	// This would reconstruct the Dockerfile content from the AST
	return nil, errors.New("AST to bytes conversion not yet implemented")
}

// buildContextManager manages build context resources
type buildContextManager struct {
	contextType ContextType
	source      string
	excludes    []string
}

func (bcm *buildContextManager) Close() error {
	// Cleanup any temporary resources
	return nil
}

// progressReporter implements the ProgressReporter interface
type progressReporter struct {
	ch    chan<- *ProgressEvent
	total int
}

func (pr *progressReporter) ReportProgress(event *ProgressEvent) {
	if pr.ch != nil {
		select {
		case pr.ch <- event:
		default:
			// Channel is full or closed, skip this update
		}
	}
}

func (pr *progressReporter) SetTotal(total int) {
	pr.total = total
}

func (pr *progressReporter) Close() {
	// Progress channel is managed by caller
}

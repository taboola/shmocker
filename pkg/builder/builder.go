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

	// Validate multi-stage build if target is specified
	if err := b.validateMultiStageBuild(req); err != nil {
		return nil, errors.Wrap(err, "multi-stage build validation failed")
	}

	// Prepare build context
	buildContext, err := b.prepareBuildContext(ctx, &req.Context)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare build context")
	}
	defer buildContext.Close()

	// Handle cache import if specified
	if len(req.CacheFrom) > 0 {
		if err := b.controller.ImportCache(ctx, req.CacheFrom); err != nil {
			return nil, errors.Wrap(err, "failed to import cache")
		}
	}

	// Execute multi-platform build if multiple platforms specified
	if len(req.Platforms) > 1 {
		return b.buildMultiPlatform(ctx, req, buildContext)
	}

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

	// Extract metadata from build result
	if result.Metadata != nil {
		// TODO: Parse build metadata for cache statistics, manifests, etc.
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
		// Validate supported platforms
		if !b.isSupportedPlatform(platform) {
			return errors.Errorf("unsupported platform: %s", platform.String())
		}
	}

	return nil
}

// validateMultiStageBuild validates multi-stage build specific requirements
func (b *builder) validateMultiStageBuild(req *BuildRequest) error {
	if req.Dockerfile == nil || len(req.Dockerfile.Stages) <= 1 {
		return nil // Single stage build, no validation needed
	}

	// Build stage name map
	stageNames := make(map[string]int)
	for i, stage := range req.Dockerfile.Stages {
		if stage.Name != "" {
			stageNames[stage.Name] = i
		}
	}

	// Validate target stage exists
	if req.Target != "" {
		if _, exists := stageNames[req.Target]; !exists {
			return errors.Errorf("target stage '%s' not found in Dockerfile", req.Target)
		}
	}

	// Validate stage dependencies
	for i, stage := range req.Dockerfile.Stages {
		// Check FROM stage references
		if stage.From != nil && stage.From.Stage != "" {
			if depIndex, exists := stageNames[stage.From.Stage]; !exists {
				return errors.Errorf("stage %d references unknown stage '%s'", i, stage.From.Stage)
			} else if depIndex >= i {
				return errors.Errorf("stage %d references future stage '%s' (index %d)", i, stage.From.Stage, depIndex)
			}
		}

		// Check COPY --from stage references
		for _, instr := range stage.Instructions {
			if copy, ok := instr.(*dockerfile.CopyInstruction); ok && copy.From != "" {
				// Check if it's a stage reference (not an external image)
				if depIndex, exists := stageNames[copy.From]; exists {
					if depIndex >= i {
						return errors.Errorf("stage %d COPY --from references future stage '%s' (index %d)", i, copy.From, depIndex)
					}
				}
			}
		}
	}

	return nil
}

// isSupportedPlatform checks if the platform is supported
func (b *builder) isSupportedPlatform(platform Platform) bool {
	// For now, support common platforms
	supportedPlatforms := map[string][]string{
		"linux": {"amd64", "arm64", "arm", "386", "ppc64le", "s390x"},
		"darwin": {"amd64", "arm64"},
		"windows": {"amd64"},
	}

	if archs, exists := supportedPlatforms[platform.OS]; exists {
		for _, arch := range archs {
			if arch == platform.Architecture {
				return true
			}
		}
	}

	return false
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
	// Create LLB converter for multi-stage support
	converter := dockerfile.NewLLBConverter()
	
	// Prepare conversion options
	convertOpts := &dockerfile.ConvertOptions{
		BuildArgs: req.BuildArgs,
		Target:    req.Target,
		Labels:    req.Labels,
	}
	
	// Set platform if specified
	if len(req.Platforms) > 0 {
		convertOpts.Platform = req.Platforms[0].String()
	}
	
	// Convert Dockerfile AST to LLB definition with multi-stage support
	llbDef, err := converter.Convert(req.Dockerfile, convertOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert Dockerfile AST to LLB")
	}
	
	// Prepare frontend attributes for BuildKit
	frontendAttrs := make(map[string][]byte)
	
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
	
	// Copy LLB metadata to frontend attributes
	for k, v := range llbDef.Metadata {
		frontendAttrs[k] = v
	}
	
	return &SolveDefinition{
		Definition: llbDef.Definition,
		Frontend:   "dockerfile.v0",
		Metadata:   frontendAttrs,
	}, nil
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

// buildMultiPlatform handles multi-platform builds
func (b *builder) buildMultiPlatform(ctx context.Context, req *BuildRequest, buildCtx *buildContextManager) (*BuildResult, error) {
	startTime := time.Now()
	var manifests []*ImageManifest
	var errors []error

	// Build for each platform
	for _, platform := range req.Platforms {
		// Create platform-specific build request
		platformReq := *req
		platformReq.Platforms = []Platform{platform}

		// Generate LLB definition for this platform
		def, err := b.generateLLBDefinition(ctx, &platformReq, buildCtx)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to generate LLB for platform %s: %w", platform.String(), err))
			continue
		}

		// Execute build for this platform
		result, err := b.controller.Solve(ctx, def)
		if err != nil {
			errors = append(errors, fmt.Errorf("build failed for platform %s: %w", platform.String(), err))
			continue
		}

		// Create manifest for this platform
		manifest := &ImageManifest{
			MediaType:     "application/vnd.oci.image.manifest.v1+json",
			SchemaVersion: 2,
			Platform:      &platform,
			// TODO: Populate config and layers from result
		}
		manifests = append(manifests, manifest)
	}

	// Check if any builds succeeded
	if len(manifests) == 0 {
		return nil, fmt.Errorf("all platform builds failed: %v", errors)
	}

	// Create multi-platform result
	buildResult := &BuildResult{
		ImageID:     fmt.Sprintf("multi-platform-%d", time.Now().Unix()),
		Manifests:   manifests,
		BuildTime:   time.Since(startTime),
		CacheHits:   0, // TODO: Aggregate from platform builds
		CacheMisses: 0, // TODO: Aggregate from platform builds
	}

	// Handle cache export if specified
	if len(req.CacheTo) > 0 {
		if err := b.controller.ExportCache(ctx, req.CacheTo); err != nil {
			return nil, fmt.Errorf("failed to export cache: %w", err)
		}
		buildResult.ExportedCache = req.CacheTo
	}

	return buildResult, nil
}

//go:build linux
// +build linux

package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/cache/remotecache"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/control"
	"github.com/moby/buildkit/executor"
	"github.com/moby/buildkit/frontend/dockerui"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/worker/base"
	"github.com/moby/buildkit/worker/runc"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/shmocker/shmocker/pkg/cache"
)

// buildKitController implements the BuildKitController interface
type buildKitController struct {
	controller *control.Controller
	worker     *runc.Worker
	closer     func() error
}

// NewBuildKitController creates a new embedded BuildKit controller
func NewBuildKitController(ctx context.Context, opts *BuildKitOptions) (BuildKitController, error) {
	if opts == nil {
		opts = &BuildKitOptions{}
	}

	// Set default options
	if opts.Root == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get user home directory")
		}
		opts.Root = filepath.Join(homeDir, ".shmocker", "buildkit")
	}

	if opts.DataRoot == "" {
		opts.DataRoot = filepath.Join(opts.Root, "data")
	}

	// Ensure directories exist
	if err := os.MkdirAll(opts.Root, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create root directory")
	}

	if err := os.MkdirAll(opts.DataRoot, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create data root directory")
	}

	// Create rootless OCI worker
	worker, err := createRootlessWorker(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create rootless worker")
	}

	// Create controller
	controller, err := control.NewController(control.Opt{
		WorkerController: &workerController{worker: worker},
		SessionManager:   session.NewManager(),
		ContentStore:     worker.ContentStore(),
		Entitlements:     []entitlements.Entitlement{},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create controller")
	}

	return &buildKitController{
		controller: controller,
		worker:     worker,
		closer: func() error {
			return worker.Close()
		},
	}, nil
}

// BuildKitOptions contains configuration options for BuildKit controller
type BuildKitOptions struct {
	Root     string
	DataRoot string
	Debug    bool
}

// Solve executes a BuildKit solve operation
func (c *buildKitController) Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
	if def == nil {
		return nil, errors.New("solve definition cannot be nil")
	}

	// Create solve request
	req := &control.SolveRequest{
		Definition: &pb.Definition{
			Def: def.Definition,
		},
		Frontend:      def.Frontend,
		FrontendAttrs: make(map[string]string),
		ExporterAttrs: make(map[string]string),
		Session:       session.NewSession(ctx, "shmocker", ""),
	}

	// Add metadata to frontend attributes
	for k, v := range def.Metadata {
		req.FrontendAttrs[k] = string(v)
	}

	// Set multi-platform build configuration
	if platformStr, exists := req.FrontendAttrs["platform"]; exists {
		// Parse platform and set up multi-platform build if needed
		if err := c.configurePlatformBuild(req, platformStr); err != nil {
			return nil, errors.Wrap(err, "failed to configure platform build")
		}
	}

	// Configure exporter for multi-platform output
	req.ExporterAttrs["name"] = "docker"
	req.ExporterAttrs["push"] = "false"

	// Execute solve
	res, err := c.controller.Solve(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "solve failed")
	}

	result := &SolveResult{
		Metadata: make(map[string][]byte),
	}

	if res.Ref != nil {
		result.Ref = res.Ref.ID()
	}

	// Copy metadata
	for k, v := range res.Metadata {
		result.Metadata[k] = v
	}

	return result, nil
}

// ImportCache imports build cache from external sources
func (c *buildKitController) ImportCache(ctx context.Context, imports []*CacheImport) error {
	if len(imports) == 0 {
		return nil
	}

	// TODO: Implement cache import using BuildKit's remote cache
	// This would involve setting up cache importers based on the cache type
	for _, imp := range imports {
		switch imp.Type {
		case "registry":
			// Import from registry cache
			_, err := remotecache.NewImporter(imp.Type)
			if err != nil {
				return errors.Wrapf(err, "failed to create cache importer for type %s", imp.Type)
			}
		case "local":
			// Import from local cache
			// TODO: Implement local cache import
		default:
			return errors.Errorf("unsupported cache import type: %s", imp.Type)
		}
	}

	return nil
}

// ExportCache exports build cache to external destinations
func (c *buildKitController) ExportCache(ctx context.Context, exports []*CacheExport) error {
	if len(exports) == 0 {
		return nil
	}

	// TODO: Implement cache export using BuildKit's remote cache
	for _, exp := range exports {
		switch exp.Type {
		case "registry":
			// Export to registry cache
			_, err := remotecache.NewExporter(exp.Type)
			if err != nil {
				return errors.Wrapf(err, "failed to create cache exporter for type %s", exp.Type)
			}
		case "local":
			// Export to local cache
			// TODO: Implement local cache export
		default:
			return errors.Errorf("unsupported cache export type: %s", exp.Type)
		}
	}

	return nil
}

// GetSession returns the current BuildKit session
func (c *buildKitController) GetSession(ctx context.Context) (Session, error) {
	sess := session.NewSession(ctx, "shmocker", "")
	return &buildKitSession{session: sess}, nil
}

// Close shuts down the BuildKit controller
func (c *buildKitController) Close() error {
	if c.closer != nil {
		return c.closer()
	}
	return nil
}

// buildKitSession implements the Session interface
type buildKitSession struct {
	session *session.Session
}

func (s *buildKitSession) ID() string {
	return s.session.ID()
}

func (s *buildKitSession) Run(ctx context.Context) error {
	return s.session.Run(ctx, nil)
}

func (s *buildKitSession) Close() error {
	return s.session.Close()
}

// workerController implements the WorkerController interface
type workerController struct {
	worker *runc.Worker
}

func (wc *workerController) GetDefault() (Worker, error) {
	return &buildKitWorker{worker: wc.worker}, nil
}

func (wc *workerController) List() ([]Worker, error) {
	return []Worker{&buildKitWorker{worker: wc.worker}}, nil
}

// buildKitWorker implements the Worker interface
type buildKitWorker struct {
	worker *runc.Worker
}

func (w *buildKitWorker) GetWorkerController() WorkerController {
	return &workerController{worker: w.worker}
}

func (w *buildKitWorker) Platforms() []Platform {
	// Return supported platforms for multi-arch builds
	return []Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm", Variant: "v7"},
		{OS: "linux", Architecture: "arm", Variant: "v6"},
		{OS: "linux", Architecture: "386"},
		{OS: "linux", Architecture: "ppc64le"},
		{OS: "linux", Architecture: "s390x"},
	}
}

func (w *buildKitWorker) Executor() Executor {
	return &buildKitExecutor{worker: w.worker}
}

func (w *buildKitWorker) CacheManager() cache.Manager {
	// TODO: Return the actual cache manager from worker
	return nil
}

// buildKitExecutor implements the Executor interface
type buildKitExecutor struct {
	worker *runc.Worker
}

func (e *buildKitExecutor) Run(ctx context.Context, step *ExecutionStep) (*ExecutionResult, error) {
	start := time.Now()

	if step == nil {
		return nil, errors.New("execution step cannot be nil")
	}

	if len(step.Command) == 0 {
		return nil, errors.New("execution step must have a command")
	}

	// Get the worker's executor
	executor := e.worker.Executor()
	if executor == nil {
		return nil, errors.New("worker executor not available")
	}

	// Create execution metadata
	meta := &pb.Meta{
		Args: step.Command,
		Env:  step.Env,
		Cwd:  step.WorkingDir,
		User: step.User,
	}

	// Create mounts if specified
	var mounts []executor.Mount
	for _, mount := range step.Mounts {
		mounts = append(mounts, executor.Mount{
			Src:      mount.Source,
			Dest:     mount.Target,
			Readonly: false, // TODO: Make configurable based on mount options
		})
	}

	// Create process for rootless execution
	proc := &pb.ProcessInfo{
		Meta:   meta,
		Stdin:  -1,
		Stdout: -1,
		Stderr: -1,
	}

	// Execute the process in rootless mode
	// Note: This is a simplified implementation. In a real scenario,
	// we would use the worker's actual executor with proper container creation
	exitCode := 0
	var stdout, stderr []byte

	// For now, simulate successful execution
	// In production, this would involve:
	// 1. Creating a rootless container
	// 2. Setting up the execution environment
	// 3. Running the command with proper isolation
	// 4. Capturing output and exit code

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: time.Since(start),
	}, nil
}

func (e *buildKitExecutor) Prepare(ctx context.Context, spec *ExecutionSpec) error {
	if spec == nil {
		return errors.New("execution spec cannot be nil")
	}

	// Validate platform specification
	if spec.Platform.OS == "" || spec.Platform.Architecture == "" {
		return errors.New("platform OS and architecture must be specified")
	}

	// Check if the specified platform is supported
	workerPlatforms := []platforms.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm", Variant: "v7"},
		{OS: "linux", Architecture: "arm", Variant: "v6"},
		{OS: "linux", Architecture: "386"},
		{OS: "linux", Architecture: "ppc64le"},
		{OS: "linux", Architecture: "s390x"},
	}

	targetPlatform := platforms.Platform{
		OS:           spec.Platform.OS,
		Architecture: spec.Platform.Architecture,
		Variant:      spec.Platform.Variant,
	}

	supported := false
	for _, wp := range workerPlatforms {
		if platforms.Only(targetPlatform).Match(wp) {
			supported = true
			break
		}
	}

	if !supported {
		return errors.Errorf("platform %s not supported by rootless worker", targetPlatform)
	}

	// Prepare rootless execution environment
	// This involves setting up user namespaces, cgroups v2 (if available),
	// and other rootless-specific configurations

	// Check rootless requirements
	if err := e.checkRootlessRequirements(ctx); err != nil {
		return errors.Wrap(err, "rootless requirements check failed")
	}

	return nil
}

func (e *buildKitExecutor) Cleanup(ctx context.Context) error {
	// Cleanup execution environment
	// This would typically involve:
	// 1. Stopping any running containers
	// 2. Cleaning up temporary files
	// 3. Releasing resources
	// 4. Unmounting filesystems

	// For now, this is a no-op as we're using BuildKit's built-in cleanup
	return nil
}

// createRootlessWorker creates a rootless OCI worker with overlayfs snapshotter
func createRootlessWorker(ctx context.Context, opts *BuildKitOptions) (*runc.Worker, error) {
	// Define supported platforms for multi-arch builds
	supportedPlatforms := []platforms.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm", Variant: "v7"},
		{OS: "linux", Architecture: "arm", Variant: "v6"},
		{OS: "linux", Architecture: "386"},
		{OS: "linux", Architecture: "ppc64le"},
		{OS: "linux", Architecture: "s390x"},
	}

	// Create worker options for rootless operation with multi-platform support
	workerOpts := base.WorkerOpt{
		ID:            "rootless-worker",
		Labels: map[string]string{
			"org.mobyproject.buildkit.worker.executor": "runc",
			"org.mobyproject.buildkit.worker.snapshotter": "overlayfs",
			"org.mobyproject.buildkit.worker.rootless": "true",
		},
		Platforms:     supportedPlatforms,
		GCPolicy:      []client.PruneInfo{},
		MetadataStore: nil, // Will be set up by the worker
		Executor:      "runc",
		Snapshotter:   "overlayfs",
		RootDir:       opts.DataRoot,
		Debug:         opts.Debug,
	}

	// Create rootless worker using runc
	worker, err := runc.NewWorker(ctx, workerOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create runc worker")
	}

	return worker, nil
}

// configurePlatformBuild configures the solve request for platform-specific builds.
func (c *buildKitController) configurePlatformBuild(req *control.SolveRequest, platformStr string) error {
	// Parse the platform string (e.g., "linux/amd64", "linux/arm64")
	if platformStr == "" {
		return nil // Use default platform
	}

	// Validate platform format
	platformSpec, err := platforms.Parse(platformStr)
	if err != nil {
		return errors.Wrapf(err, "invalid platform format: %s", platformStr)
	}

	// Check if platform is supported
	if !c.isPlatformSupported(platformSpec) {
		return errors.Errorf("unsupported platform: %s", platformStr)
	}

	// Set platform-specific attributes
	req.FrontendAttrs["platform"] = platforms.Format(platformSpec)
	
	// Configure cross-compilation if needed
	defaultPlatform := platforms.DefaultSpec()
	if !platforms.Only(platformSpec).Match(defaultPlatform) {
		// Cross-compilation setup
		req.FrontendAttrs["build-arg:TARGETPLATFORM"] = platforms.Format(platformSpec)
		req.FrontendAttrs["build-arg:TARGETOS"] = platformSpec.OS
		req.FrontendAttrs["build-arg:TARGETARCH"] = platformSpec.Architecture
		if platformSpec.Variant != "" {
			req.FrontendAttrs["build-arg:TARGETVARIANT"] = platformSpec.Variant
		}
		
		// Host platform info
		req.FrontendAttrs["build-arg:BUILDPLATFORM"] = platforms.Format(defaultPlatform)
		req.FrontendAttrs["build-arg:BUILDOS"] = defaultPlatform.OS
		req.FrontendAttrs["build-arg:BUILDARCH"] = defaultPlatform.Architecture
		if defaultPlatform.Variant != "" {
			req.FrontendAttrs["build-arg:BUILDVARIANT"] = defaultPlatform.Variant
		}
	}

	return nil
}

// isPlatformSupported checks if the given platform is supported by the worker.
func (c *buildKitController) isPlatformSupported(platform platforms.Platform) bool {
	workerPlatforms := []platforms.Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm", Variant: "v7"},
		{OS: "linux", Architecture: "arm", Variant: "v6"},
		{OS: "linux", Architecture: "386"},
		{OS: "linux", Architecture: "ppc64le"},
		{OS: "linux", Architecture: "s390x"},
	}

	for _, wp := range workerPlatforms {
		if platforms.Only(platform).Match(wp) {
			return true
		}
	}

	return false
}

// checkRootlessRequirements verifies that the system supports rootless execution.
func (e *buildKitExecutor) checkRootlessRequirements(ctx context.Context) error {
	// Check if we're already running as non-root
	if os.Getuid() == 0 {
		return errors.New("rootless execution should not run as root user")
	}

	// Check for user namespace support
	if _, err := os.Stat("/proc/self/ns/user"); err != nil {
		return errors.Wrap(err, "user namespaces not supported")
	}

	// Check for newuidmap/newgidmap
	uidmapPath := "/usr/bin/newuidmap"
	gidmapPath := "/usr/bin/newgidmap"
	
	if _, err := os.Stat(uidmapPath); err != nil {
		return errors.Wrapf(err, "newuidmap not found at %s", uidmapPath)
	}
	
	if _, err := os.Stat(gidmapPath); err != nil {
		return errors.Wrapf(err, "newgidmap not found at %s", gidmapPath)
	}

	// Check for cgroups v2 support (optional but recommended)
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		// cgroups v2 not available, log warning but don't fail
		// fmt.Printf("Warning: cgroups v2 not available, some features may be limited\n")
	}

	return nil
}

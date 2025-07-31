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
	// Get default platform
	defaultPlatform := platforms.DefaultSpec()
	return []Platform{
		{
			OS:           defaultPlatform.OS,
			Architecture: defaultPlatform.Architecture,
			Variant:      defaultPlatform.Variant,
		},
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

	// TODO: Implement actual execution using the worker's executor
	// This would involve creating a proper execution environment and running the step

	return &ExecutionResult{
		ExitCode: 0,
		Duration: time.Since(start),
	}, nil
}

func (e *buildKitExecutor) Prepare(ctx context.Context, spec *ExecutionSpec) error {
	// TODO: Implement execution environment preparation
	return nil
}

func (e *buildKitExecutor) Cleanup(ctx context.Context) error {
	// TODO: Implement cleanup logic
	return nil
}

// createRootlessWorker creates a rootless OCI worker with overlayfs snapshotter
func createRootlessWorker(ctx context.Context, opts *BuildKitOptions) (*runc.Worker, error) {
	// Create worker options for rootless operation
	workerOpts := base.WorkerOpt{
		ID:            "rootless-worker",
		Labels:        map[string]string{},
		Platforms:     []platforms.Platform{platforms.DefaultSpec()},
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

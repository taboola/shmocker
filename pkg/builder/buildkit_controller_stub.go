//go:build !linux
// +build !linux

package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// buildKitControllerStub is a stub implementation for non-Linux platforms
// This allows the CLI to compile and run basic operations
type buildKitControllerStub struct {
	options *BuildKitOptions
}

// NewBuildKitController creates a stub BuildKit controller for non-Linux platforms
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

	return &buildKitControllerStub{
		options: opts,
	}, nil
}

// Solve implements a stub solve operation
func (c *buildKitControllerStub) Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
	if def == nil {
		return nil, errors.New("solve definition cannot be nil")
	}

	// This is a stub implementation that simulates a successful build
	fmt.Printf("STUB: Simulating BuildKit solve operation\n")
	fmt.Printf("STUB: Frontend: %s\n", def.Frontend)
	fmt.Printf("STUB: Definition size: %d bytes\n", len(def.Definition))

	// Return a mock result
	return &SolveResult{
		Ref:      "sha256:stub-image-id-" + fmt.Sprintf("%x", len(def.Definition)),
		Metadata: make(map[string][]byte),
	}, nil
}

// ImportCache implements stub cache import
func (c *buildKitControllerStub) ImportCache(ctx context.Context, imports []*CacheImport) error {
	fmt.Printf("STUB: Would import %d cache sources\n", len(imports))
	return nil
}

// ExportCache implements stub cache export
func (c *buildKitControllerStub) ExportCache(ctx context.Context, exports []*CacheExport) error {
	fmt.Printf("STUB: Would export to %d cache destinations\n", len(exports))
	return nil
}

// GetSession returns a stub session
func (c *buildKitControllerStub) GetSession(ctx context.Context) (Session, error) {
	return &buildKitSessionStub{id: "stub-session-123"}, nil
}

// Close cleans up the stub controller
func (c *buildKitControllerStub) Close() error {
	fmt.Printf("STUB: Closing BuildKit controller\n")
	return nil
}

// buildKitSessionStub implements a stub session
type buildKitSessionStub struct {
	id string
}

func (s *buildKitSessionStub) ID() string {
	return s.id
}

func (s *buildKitSessionStub) Run(ctx context.Context) error {
	fmt.Printf("STUB: Running session %s\n", s.id)
	return nil
}

func (s *buildKitSessionStub) Close() error {
	fmt.Printf("STUB: Closing session %s\n", s.id)
	return nil
}

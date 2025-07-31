//go:build darwin
// +build darwin

package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"

	"github.com/shmocker/shmocker/pkg/cache"
)

const (
	// Lima VM name for shmocker BuildKit
	limaVMName = "shmocker-buildkit"
	
	// BuildKit daemon address inside Lima VM
	buildkitSocketPath = "unix:///run/user/1000/buildkit/buildkitd.sock"
	
	// BuildKit TCP port for host access
	buildkitPort = 1234
	
	// Lima command timeout
	limaCommandTimeout = 30 * time.Second
)

// limaBuildKitController implements BuildKitController using Lima VM
type limaBuildKitController struct {
	client     *client.Client
	vmRunning  bool
	options    *BuildKitOptions
	tcpAddress string
}

// NewBuildKitController creates a new Lima-based BuildKit controller
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

	controller := &limaBuildKitController{
		options:    opts,
		tcpAddress: fmt.Sprintf("tcp://127.0.0.1:%d", buildkitPort),
	}

	// Check if Lima VM is available and running
	if err := controller.ensureVMRunning(); err != nil {
		return nil, errors.Wrap(err, "failed to ensure Lima VM is running")
	}

	// Connect to BuildKit daemon
	if err := controller.connectToBuildKit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to BuildKit daemon")
	}

	return controller, nil
}

// ensureVMRunning checks if the Lima VM is running and starts it if necessary
func (c *limaBuildKitController) ensureVMRunning() error {
	// Check if Lima is installed
	if !isLimaInstalled() {
		return errors.New("Lima is not installed. Please run the setup script: scripts/setup-macos.sh")
	}

	// Check VM status
	status, err := c.getVMStatus()
	if err != nil {
		return errors.Wrap(err, "failed to get VM status")
	}

	switch status {
	case "Running":
		c.vmRunning = true
		return nil
	case "Stopped":
		return c.startVM()
	case "":
		return errors.Errorf("Lima VM '%s' not found. Please run setup script: scripts/setup-macos.sh", limaVMName)
	default:
		return errors.Errorf("Lima VM '%s' is in unexpected state: %s", limaVMName, status)
	}
}

// getVMStatus returns the current status of the Lima VM
func (c *limaBuildKitController) getVMStatus() (string, error) {
	cmd := exec.Command("limactl", "list", "--format", "{{.Status}}", limaVMName)
	output, err := cmd.Output()
	if err != nil {
		// VM might not exist
		return "", nil
	}
	
	return strings.TrimSpace(string(output)), nil
}

// startVM starts the Lima VM
func (c *limaBuildKitController) startVM() error {
	fmt.Printf("Starting Lima VM '%s'...\n", limaVMName)
	
	cmd := exec.Command("limactl", "start", limaVMName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to start Lima VM '%s'", limaVMName)
	}
	
	c.vmRunning = true
	
	// Wait for BuildKit to be ready
	return c.waitForBuildKit()
}

// waitForBuildKit waits for BuildKit daemon to be ready
func (c *limaBuildKitController) waitForBuildKit() error {
	fmt.Println("Waiting for BuildKit daemon to be ready...")
	
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for BuildKit daemon to be ready")
		case <-ticker.C:
			if c.isBuildKitReady() {
				fmt.Println("BuildKit daemon is ready")
				return nil
			}
		}
	}
}

// isBuildKitReady checks if BuildKit daemon is responding
func (c *limaBuildKitController) isBuildKitReady() bool {
	// Check if port is accessible
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", buildkitPort), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	
	// Try to connect with BuildKit client
	client, err := client.New(context.Background(), c.tcpAddress)
	if err != nil {
		return false
	}
	defer client.Close()
	
	// Test with a simple workers call
	_, err = client.ListWorkers(context.Background())
	return err == nil
}

// connectToBuildKit establishes connection to BuildKit daemon
func (c *limaBuildKitController) connectToBuildKit(ctx context.Context) error {
	var err error
	c.client, err = client.New(ctx, c.tcpAddress)
	if err != nil {
		return errors.Wrap(err, "failed to create BuildKit client")
	}
	
	// Verify connection
	workers, err := c.client.ListWorkers(ctx)
	if err != nil {
		c.client.Close()
		c.client = nil
		return errors.Wrap(err, "failed to list workers")
	}
	
	if c.options.Debug {
		fmt.Printf("Connected to BuildKit daemon with %d workers\n", len(workers))
		for _, worker := range workers {
			fmt.Printf("  Worker: %s, Platforms: %v\n", worker.ID, worker.Platforms)
		}
	}
	
	return nil
}

// Solve executes a BuildKit solve operation
func (c *limaBuildKitController) Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
	if c.client == nil {
		return nil, errors.New("BuildKit client not connected")
	}
	
	if def == nil {
		return nil, errors.New("solve definition cannot be nil")
	}

	// Convert solve definition to BuildKit format
	solveOpt := client.SolveOpt{
		Frontend: def.Frontend,
	}

	// Set frontend attributes from metadata
	if def.Metadata != nil {
		solveOpt.FrontendAttrs = make(map[string]string)
		for k, v := range def.Metadata {
			solveOpt.FrontendAttrs[k] = string(v)
		}
	}

	// Parse definition
	if def.Definition != nil {
		llbDef, err := llb.ReadFrom(def.Definition)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse LLB definition")
		}
		solveOpt.Definition = llbDef
	}

	// Configure output
	solveOpt.Exports = []client.ExportEntry{
		{
			Type: client.ExporterDocker,
			Attrs: map[string]string{
				"name": "shmocker-build",
				"push": "false",
			},
		},
	}

	// Execute solve
	ch := make(chan *client.SolveStatus)
	eg := make(chan error, 1)
	
	go func() {
		_, err := c.client.Solve(ctx, def.Definition, solveOpt, ch)
		eg <- err
	}()

	// Process status updates
	for status := range ch {
		if c.options.Debug {
			for _, vertex := range status.Vertexes {
				fmt.Printf("Vertex %s: %s\n", vertex.Digest, vertex.Name)
			}
		}
	}

	// Wait for completion
	if err := <-eg; err != nil {
		return nil, errors.Wrap(err, "solve failed")
	}

	result := &SolveResult{
		Ref:      "completed", // Simplified for now
		Metadata: make(map[string][]byte),
	}

	// Copy metadata if available
	for k, v := range def.Metadata {
		result.Metadata[k] = v
	}

	return result, nil
}

// ImportCache imports build cache from external sources
func (c *limaBuildKitController) ImportCache(ctx context.Context, imports []*CacheImport) error {
	if len(imports) == 0 {
		return nil
	}

	if c.options.Debug {
		fmt.Printf("Importing cache from %d sources\n", len(imports))
	}

	// Cache import is handled through solve options in BuildKit
	// This is a placeholder for explicit cache import operations
	for _, imp := range imports {
		switch imp.Type {
		case "registry":
			if c.options.Debug {
				fmt.Printf("Cache import from registry: %s\n", imp.Ref)
			}
		case "local":
			if c.options.Debug {
				fmt.Printf("Cache import from local: %s\n", imp.Ref)
			}
		default:
			return errors.Errorf("unsupported cache import type: %s", imp.Type)
		}
	}

	return nil
}

// ExportCache exports build cache to external destinations
func (c *limaBuildKitController) ExportCache(ctx context.Context, exports []*CacheExport) error {
	if len(exports) == 0 {
		return nil
	}

	if c.options.Debug {
		fmt.Printf("Exporting cache to %d destinations\n", len(exports))
	}

	// Cache export is handled through solve options in BuildKit
	// This is a placeholder for explicit cache export operations
	for _, exp := range exports {
		switch exp.Type {
		case "registry":
			if c.options.Debug {
				fmt.Printf("Cache export to registry: %s\n", exp.Ref)
			}
		case "local":
			if c.options.Debug {
				fmt.Printf("Cache export to local: %s\n", exp.Ref)
			}
		default:
			return errors.Errorf("unsupported cache export type: %s", exp.Type)
		}
	}

	return nil
}

// GetSession returns the current BuildKit session
func (c *limaBuildKitController) GetSession(ctx context.Context) (Session, error) {
	if c.client == nil {
		return nil, errors.New("BuildKit client not connected")
	}

	// Create a Lima session wrapper
	session := &limaBuildKitSession{
		id:         fmt.Sprintf("lima-session-%d", time.Now().Unix()),
		controller: c,
	}

	return session, nil
}

// Close shuts down the BuildKit controller
func (c *limaBuildKitController) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// limaBuildKitSession implements the Session interface for Lima
type limaBuildKitSession struct {
	id         string
	controller *limaBuildKitController
}

func (s *limaBuildKitSession) ID() string {
	return s.id
}

func (s *limaBuildKitSession) Run(ctx context.Context) error {
	// Session management for Lima is handled by the VM
	return nil
}

func (s *limaBuildKitSession) Close() error {
	// Session cleanup for Lima is handled by the VM
	return nil
}

// Utility functions

// isLimaInstalled checks if Lima is available
func isLimaInstalled() bool {
	_, err := exec.LookPath("limactl")
	return err == nil
}

// runLimaCommand executes a command inside the Lima VM
func runLimaCommand(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limaCommandTimeout)
	defer cancel()
	
	fullArgs := append([]string{"shell", limaVMName}, args...)
	cmd := exec.CommandContext(ctx, "limactl", fullArgs...)
	
	return cmd.Output()
}

// Lima-specific worker implementations

// limaWorkerController implements WorkerController for Lima
type limaWorkerController struct {
	controller *limaBuildKitController
}

func (wc *limaWorkerController) GetDefault() (Worker, error) {
	return &limaWorker{controller: wc.controller}, nil
}

func (wc *limaWorkerController) List() ([]Worker, error) {
	return []Worker{&limaWorker{controller: wc.controller}}, nil
}

// limaWorker implements Worker for Lima
type limaWorker struct {
	controller *limaBuildKitController
}

func (w *limaWorker) GetWorkerController() WorkerController {
	return &limaWorkerController{controller: w.controller}
}

func (w *limaWorker) Platforms() []Platform {
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

func (w *limaWorker) Executor() Executor {
	return &limaExecutor{controller: w.controller}
}

func (w *limaWorker) CacheManager() cache.Manager {
	// Return a Lima-specific cache manager
	return nil // TODO: Implement Lima cache manager
}

// limaExecutor implements Executor for Lima
type limaExecutor struct {
	controller *limaBuildKitController
}

func (e *limaExecutor) Run(ctx context.Context, step *ExecutionStep) (*ExecutionResult, error) {
	start := time.Now()

	if step == nil {
		return nil, errors.New("execution step cannot be nil")
	}

	if len(step.Command) == 0 {
		return nil, errors.New("execution step must have a command")
	}

	// Execute command in Lima VM
	args := append([]string{"shell", limaVMName}, step.Command...)
	cmd := exec.CommandContext(ctx, "limactl", args...)
	
	// Set environment variables
	if len(step.Env) > 0 {
		cmd.Env = append(os.Environ(), step.Env...)
	}

	// Set working directory (within VM)
	if step.WorkingDir != "" {
		cmd.Dir = step.WorkingDir
	}

	// Execute command
	stdout, err := cmd.Output()
	exitCode := 0
	var stderr []byte

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
			stderr = exitError.Stderr
		} else {
			return nil, errors.Wrap(err, "failed to execute command in Lima VM")
		}
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: time.Since(start),
	}, nil
}

func (e *limaExecutor) Prepare(ctx context.Context, spec *ExecutionSpec) error {
	if spec == nil {
		return errors.New("execution spec cannot be nil")
	}

	// Validate platform specification
	if spec.Platform.OS == "" || spec.Platform.Architecture == "" {
		return errors.New("platform OS and architecture must be specified")
	}

	// Check if the specified platform is supported
	supportedPlatforms := []Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
		{OS: "linux", Architecture: "arm", Variant: "v7"},
		{OS: "linux", Architecture: "arm", Variant: "v6"},
		{OS: "linux", Architecture: "386"},
		{OS: "linux", Architecture: "ppc64le"},
		{OS: "linux", Architecture: "s390x"},
	}

	supported := false
	for _, sp := range supportedPlatforms {
		if sp.OS == spec.Platform.OS && 
		   sp.Architecture == spec.Platform.Architecture &&
		   (sp.Variant == spec.Platform.Variant || (sp.Variant == "" && spec.Platform.Variant == "")) {
			supported = true
			break
		}
	}

	if !supported {
		return errors.Errorf("platform %s not supported by Lima worker", spec.Platform.String())
	}

	return nil
}

func (e *limaExecutor) Cleanup(ctx context.Context) error {
	// Cleanup is handled by the Lima VM and BuildKit daemon
	return nil
}

// Enhanced Lima VM management functions

// VMInfo represents Lima VM information
type VMInfo struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Arch      string    `json:"arch"`
	CPUs      int       `json:"cpus"`
	Memory    string    `json:"memory"`
	Disk      string    `json:"disk"`
	Dir       string    `json:"dir"`
	CreatedAt time.Time `json:"created_at"`
}

// GetVMInfo returns detailed information about the Lima VM
func (c *limaBuildKitController) GetVMInfo() (*VMInfo, error) {
	cmd := exec.Command("limactl", "list", "--json", limaVMName)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VM info")
	}

	var vms []VMInfo
	if err := json.Unmarshal(output, &vms); err != nil {
		return nil, errors.Wrap(err, "failed to parse VM info")
	}

	if len(vms) == 0 {
		return nil, errors.Errorf("Lima VM '%s' not found", limaVMName)
	}

	return &vms[0], nil
}

// RestartVM restarts the Lima VM
func (c *limaBuildKitController) RestartVM() error {
	fmt.Printf("Restarting Lima VM '%s'...\n", limaVMName)
	
	// Stop the VM
	stopCmd := exec.Command("limactl", "stop", limaVMName)
	if err := stopCmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to stop Lima VM '%s'", limaVMName)
	}
	
	// Start the VM
	startCmd := exec.Command("limactl", "start", limaVMName)
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	
	if err := startCmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to start Lima VM '%s'", limaVMName)
	}
	
	// Reconnect to BuildKit
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	if err := c.connectToBuildKit(ctx); err != nil {
		return errors.Wrap(err, "failed to reconnect to BuildKit after restart")
	}
	
	return nil
}

// GetBuildKitLogs retrieves BuildKit daemon logs from the Lima VM
func (c *limaBuildKitController) GetBuildKitLogs(lines int) (string, error) {
	args := []string{"shell", limaVMName, "systemctl", "--user", "status", "buildkit", "--no-pager"}
	if lines > 0 {
		args = append(args, "-n", strconv.Itoa(lines))
	}
	
	cmd := exec.Command("limactl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to get BuildKit logs")
	}
	
	return string(output), nil
}
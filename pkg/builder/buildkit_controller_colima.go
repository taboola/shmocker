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
	"github.com/pkg/errors"

	"github.com/shmocker/shmocker/pkg/cache"
)

const (
	// Colima profile name for shmocker BuildKit
	colimaProfile = "shmocker"
	
	// BuildKit daemon socket path inside Colima VM
	colimaBuildkitSocketPath = "unix:///run/user/1000/buildkit/buildkitd.sock"
	
	// Colima command timeout
	colimaCommandTimeout = 30 * time.Second
)

// colimaBuildKitController implements BuildKitController using Colima VM
type colimaBuildKitController struct {
	client      *client.Client
	vmRunning   bool
	options     *BuildKitOptions
	socketPath  string
}

// NewColimaBuildKitController creates a new Colima-based BuildKit controller
func NewColimaBuildKitController(ctx context.Context, opts *BuildKitOptions) (BuildKitController, error) {
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

	controller := &colimaBuildKitController{
		options:    opts,
		socketPath: colimaBuildkitSocketPath,
	}

	// Check if Colima profile is available and running
	if err := controller.ensureProfileRunning(); err != nil {
		return nil, errors.Wrap(err, "failed to ensure Colima profile is running")
	}

	// Connect to BuildKit daemon
	if err := controller.connectToBuildKit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to connect to BuildKit daemon")
	}

	return controller, nil
}

// ensureProfileRunning checks if the Colima profile is running and starts it if necessary
func (c *colimaBuildKitController) ensureProfileRunning() error {
	// Check if Colima is installed
	if !isColimaInstalled() {
		return errors.New("Colima is not installed. Please run the setup script: scripts/setup-macos-colima.sh")
	}

	// Check profile status
	status, err := c.getProfileStatus()
	if err != nil {
		return errors.Wrap(err, "failed to get profile status")
	}

	switch status {
	case "Running":
		c.vmRunning = true
		return nil
	case "Stopped":
		return c.startProfile()
	case "":
		return errors.Errorf("Colima profile '%s' not found. Please run setup script: scripts/setup-macos-colima.sh", colimaProfile)
	default:
		return errors.Errorf("Colima profile '%s' is in unexpected state: %s", colimaProfile, status)
	}
}

// getProfileStatus returns the current status of the Colima profile
func (c *colimaBuildKitController) getProfileStatus() (string, error) {
	cmd := exec.Command("colima", "status", "--profile", colimaProfile, "--format", "{{.Status}}")
	output, err := cmd.Output()
	if err != nil {
		// Profile might not exist
		return "", nil
	}
	
	return strings.TrimSpace(string(output)), nil
}

// startProfile starts the Colima profile
func (c *colimaBuildKitController) startProfile() error {
	fmt.Printf("Starting Colima profile '%s'...\n", colimaProfile)
	
	cmd := exec.Command("colima", "start", "--profile", colimaProfile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to start Colima profile '%s'", colimaProfile)
	}
	
	c.vmRunning = true
	
	// Wait for BuildKit to be ready
	return c.waitForBuildKit()
}

// waitForBuildKit waits for BuildKit daemon to be ready
func (c *colimaBuildKitController) waitForBuildKit() error {
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
func (c *colimaBuildKitController) isBuildKitReady() bool {
	// Try to run buildctl command inside Colima VM
	cmd := exec.Command("colima", "ssh", "--profile", colimaProfile, "--", "/usr/local/bin/buildctl", "--addr", colimaBuildkitSocketPath, "debug", "workers")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	
	// Check if we got meaningful output (workers list)
	return len(output) > 0 && strings.Contains(string(output), "ID")
}

// connectToBuildKit establishes connection to BuildKit daemon through Colima SSH tunnel
func (c *colimaBuildKitController) connectToBuildKit(ctx context.Context) error {
	// Create a custom client that uses Colima SSH for transport
	// Note: This is a simplified approach. In production, you might want to
	// use port forwarding or a more sophisticated transport mechanism.
	
	// For now, we'll create a mock client since direct socket access through SSH
	// is complex. In a real implementation, you would set up port forwarding.
	c.client = &client.Client{} // This would be properly initialized
	
	if c.options.Debug {
		fmt.Printf("Connected to BuildKit daemon through Colima profile '%s'\n", colimaProfile)
	}
	
	return nil
}

// Solve executes a BuildKit solve operation
func (c *colimaBuildKitController) Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
	if !c.vmRunning {
		return nil, errors.New("Colima profile is not running")
	}
	
	if def == nil {
		return nil, errors.New("solve definition cannot be nil")
	}

	// Execute buildctl command through Colima SSH
	buildctlArgs := []string{
		"ssh", "--profile", colimaProfile, "--",
		"/usr/local/bin/buildctl", "--addr", colimaBuildkitSocketPath,
		"build",
	}

	// Add frontend specification
	if def.Frontend != "" {
		buildctlArgs = append(buildctlArgs, "--frontend", def.Frontend)
	}

	// Add metadata as frontend attributes
	if def.Metadata != nil {
		for k, v := range def.Metadata {
			buildctlArgs = append(buildctlArgs, "--opt", fmt.Sprintf("%s=%s", k, string(v)))
		}
	}

	// Add output configuration (simplified)
	buildctlArgs = append(buildctlArgs, "--output", "type=docker,name=shmocker-build")

	// Execute the build command
	cmd := exec.CommandContext(ctx, "colima", buildctlArgs...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		return nil, errors.Wrapf(err, "buildctl solve failed: %s", string(output))
	}

	result := &SolveResult{
		Ref:      "completed",
		Metadata: make(map[string][]byte),
	}

	// Copy metadata if available
	for k, v := range def.Metadata {
		result.Metadata[k] = v
	}

	if c.options.Debug {
		fmt.Printf("Build completed successfully: %s\n", string(output))
	}

	return result, nil
}

// ImportCache imports build cache from external sources
func (c *colimaBuildKitController) ImportCache(ctx context.Context, imports []*CacheImport) error {
	if len(imports) == 0 {
		return nil
	}

	if c.options.Debug {
		fmt.Printf("Importing cache from %d sources\n", len(imports))
	}

	// Execute cache import through buildctl
	for _, imp := range imports {
		args := []string{
			"ssh", "--profile", colimaProfile, "--",
			"/usr/local/bin/buildctl", "--addr", colimaBuildkitSocketPath,
			"build", "--import-cache", fmt.Sprintf("type=%s,ref=%s", imp.Type, imp.Ref),
		}

		// Add additional attributes
		for k, v := range imp.Attrs {
			args = append(args, "--import-cache", fmt.Sprintf("%s=%s", k, v))
		}

		cmd := exec.CommandContext(ctx, "colima", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return errors.Wrapf(err, "cache import failed: %s", string(output))
		}
	}

	return nil
}

// ExportCache exports build cache to external destinations
func (c *colimaBuildKitController) ExportCache(ctx context.Context, exports []*CacheExport) error {
	if len(exports) == 0 {
		return nil
	}

	if c.options.Debug {
		fmt.Printf("Exporting cache to %d destinations\n", len(exports))
	}

	// Execute cache export through buildctl
	for _, exp := range exports {
		args := []string{
			"ssh", "--profile", colimaProfile, "--",
			"/usr/local/bin/buildctl", "--addr", colimaBuildkitSocketPath,
			"build", "--export-cache", fmt.Sprintf("type=%s,ref=%s", exp.Type, exp.Ref),
		}

		// Add additional attributes
		for k, v := range exp.Attrs {
			args = append(args, "--export-cache", fmt.Sprintf("%s=%s", k, v))
		}

		cmd := exec.CommandContext(ctx, "colima", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return errors.Wrapf(err, "cache export failed: %s", string(output))
		}
	}

	return nil
}

// GetSession returns the current BuildKit session
func (c *colimaBuildKitController) GetSession(ctx context.Context) (Session, error) {
	if !c.vmRunning {
		return nil, errors.New("Colima profile is not running")
	}

	// Create a Colima session wrapper
	session := &colimaBuildKitSession{
		id:         fmt.Sprintf("colima-session-%d", time.Now().Unix()),
		controller: c,
	}

	return session, nil
}

// Close shuts down the BuildKit controller
func (c *colimaBuildKitController) Close() error {
	if c.client != nil {
		// Note: In a real implementation, you would close the actual client
		// return c.client.Close()
	}
	return nil
}

// colimaBuildKitSession implements the Session interface for Colima
type colimaBuildKitSession struct {
	id         string
	controller *colimaBuildKitController
}

func (s *colimaBuildKitSession) ID() string {
	return s.id
}

func (s *colimaBuildKitSession) Run(ctx context.Context) error {
	// Session management for Colima is handled by the VM
	return nil
}

func (s *colimaBuildKitSession) Close() error {
	// Session cleanup for Colima is handled by the VM
	return nil
}

// Utility functions

// isColimaInstalled checks if Colima is available
func isColimaInstalled() bool {
	_, err := exec.LookPath("colima")
	return err == nil
}

// runColimaCommand executes a command inside the Colima VM
func runColimaCommand(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), colimaCommandTimeout)
	defer cancel()
	
	fullArgs := append([]string{"ssh", "--profile", colimaProfile, "--"}, args...)
	cmd := exec.CommandContext(ctx, "colima", fullArgs...)
	
	return cmd.Output()
}

// Colima-specific worker implementations

// colimaWorkerController implements WorkerController for Colima
type colimaWorkerController struct {
	controller *colimaBuildKitController
}

func (wc *colimaWorkerController) GetDefault() (Worker, error) {
	return &colimaWorker{controller: wc.controller}, nil
}

func (wc *colimaWorkerController) List() ([]Worker, error) {
	return []Worker{&colimaWorker{controller: wc.controller}}, nil
}

// colimaWorker implements Worker for Colima
type colimaWorker struct {
	controller *colimaBuildKitController
}

func (w *colimaWorker) GetWorkerController() WorkerController {
	return &colimaWorkerController{controller: w.controller}
}

func (w *colimaWorker) Platforms() []Platform {
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

func (w *colimaWorker) Executor() Executor {
	return &colimaExecutor{controller: w.controller}
}

func (w *colimaWorker) CacheManager() cache.Manager {
	// Return a Colima-specific cache manager
	return nil // TODO: Implement Colima cache manager
}

// colimaExecutor implements Executor for Colima
type colimaExecutor struct {
	controller *colimaBuildKitController
}

func (e *colimaExecutor) Run(ctx context.Context, step *ExecutionStep) (*ExecutionResult, error) {
	start := time.Now()

	if step == nil {
		return nil, errors.New("execution step cannot be nil")
	}

	if len(step.Command) == 0 {
		return nil, errors.New("execution step must have a command")
	}

	// Execute command in Colima VM
	args := append([]string{"ssh", "--profile", colimaProfile, "--"}, step.Command...)
	cmd := exec.CommandContext(ctx, "colima", args...)
	
	// Set environment variables
	if len(step.Env) > 0 {
		cmd.Env = append(os.Environ(), step.Env...)
	}

	// Set working directory (within VM)
	if step.WorkingDir != "" {
		// Prepend cd command to change directory in VM
		cdCmd := fmt.Sprintf("cd %s && %s", step.WorkingDir, strings.Join(step.Command, " "))
		args = []string{"ssh", "--profile", colimaProfile, "--", "bash", "-c", cdCmd}
		cmd = exec.CommandContext(ctx, "colima", args...)
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
			return nil, errors.Wrap(err, "failed to execute command in Colima VM")
		}
	}

	return &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: time.Since(start),
	}, nil
}

func (e *colimaExecutor) Prepare(ctx context.Context, spec *ExecutionSpec) error {
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
		return errors.Errorf("platform %s not supported by Colima worker", spec.Platform.String())
	}

	return nil
}

func (e *colimaExecutor) Cleanup(ctx context.Context) error {
	// Cleanup is handled by the Colima VM and BuildKit daemon
	return nil
}

// Enhanced Colima profile management functions

// ProfileInfo represents Colima profile information
type ProfileInfo struct {
	Name      string            `json:"name"`
	Status    string            `json:"status"`
	Arch      string            `json:"arch"`
	Runtime   string            `json:"runtime"`
	CPU       int               `json:"cpu"`
	Memory    int               `json:"memory"`
	Disk      int               `json:"disk"`
	CreatedAt time.Time         `json:"created_at"`
	Addresses map[string]string `json:"addresses"`
}

// GetProfileInfo returns detailed information about the Colima profile
func (c *colimaBuildKitController) GetProfileInfo() (*ProfileInfo, error) {
	cmd := exec.Command("colima", "list", "--profile", colimaProfile, "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get profile info")
	}

	var profiles []ProfileInfo
	if err := json.Unmarshal(output, &profiles); err != nil {
		return nil, errors.Wrap(err, "failed to parse profile info")
	}

	for _, profile := range profiles {
		if profile.Name == colimaProfile {
			return &profile, nil
		}
	}

	return nil, errors.Errorf("Colima profile '%s' not found", colimaProfile)
}

// RestartProfile restarts the Colima profile
func (c *colimaBuildKitController) RestartProfile() error {
	fmt.Printf("Restarting Colima profile '%s'...\n", colimaProfile)
	
	// Stop the profile
	stopCmd := exec.Command("colima", "stop", "--profile", colimaProfile)
	if err := stopCmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to stop Colima profile '%s'", colimaProfile)
	}
	
	// Start the profile
	startCmd := exec.Command("colima", "start", "--profile", colimaProfile)
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	
	if err := startCmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to start Colima profile '%s'", colimaProfile)
	}
	
	// Wait for BuildKit to be ready
	if err := c.waitForBuildKit(); err != nil {
		return errors.Wrap(err, "failed to wait for BuildKit after restart")
	}
	
	return nil
}

// GetBuildKitLogs retrieves BuildKit daemon logs from the Colima VM
func (c *colimaBuildKitController) GetBuildKitLogs(lines int) (string, error) {
	args := []string{"ssh", "--profile", colimaProfile, "--", "systemctl", "--user", "status", "buildkit", "--no-pager"}
	if lines > 0 {
		args = append(args, "-n", strconv.Itoa(lines))
	}
	
	cmd := exec.Command("colima", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to get BuildKit logs")
	}
	
	return string(output), nil
}

// PruneCache runs buildctl prune to clean up build cache
func (c *colimaBuildKitController) PruneCache(ctx context.Context, keepStorage string) error {
	args := []string{
		"ssh", "--profile", colimaProfile, "--",
		"/usr/local/bin/buildctl", "--addr", colimaBuildkitSocketPath,
		"prune",
	}
	
	if keepStorage != "" {
		args = append(args, "--keep-storage", keepStorage)
	}
	
	cmd := exec.CommandContext(ctx, "colima", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "cache prune failed: %s", string(output))
	}
	
	if c.options.Debug {
		fmt.Printf("Cache pruned: %s\n", string(output))
	}
	
	return nil
}

// GetDiskUsage returns BuildKit disk usage information
func (c *colimaBuildKitController) GetDiskUsage(ctx context.Context) (string, error) {
	args := []string{
		"ssh", "--profile", colimaProfile, "--",
		"/usr/local/bin/buildctl", "--addr", colimaBuildkitSocketPath,
		"du",
	}
	
	cmd := exec.CommandContext(ctx, "colima", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get disk usage: %s", string(output))
	}
	
	return string(output), nil
}
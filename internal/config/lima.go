//go:build darwin
// +build darwin

package config

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// LimaConfig holds Lima-specific configuration
type LimaConfig struct {
	VMName           string `yaml:"vm_name" json:"vm_name"`
	AutoStart        bool   `yaml:"auto_start" json:"auto_start"`
	BuildKitPort     int    `yaml:"buildkit_port" json:"buildkit_port"`
	BuildKitAddress  string `yaml:"buildkit_address" json:"buildkit_address"`
	SetupScriptPath  string `yaml:"setup_script_path" json:"setup_script_path"`
}

// DefaultLimaConfig returns the default Lima configuration
func DefaultLimaConfig() *LimaConfig {
	return &LimaConfig{
		VMName:          "shmocker-buildkit",
		AutoStart:       true,
		BuildKitPort:    1234,
		BuildKitAddress: "tcp://127.0.0.1:1234",
		SetupScriptPath: "scripts/setup-macos.sh",
	}
}

// DetectLima checks if Lima is available and properly configured
func DetectLima() (*LimaDetectionResult, error) {
	result := &LimaDetectionResult{
		Available:    false,
		VMExists:     false,
		VMRunning:    false,
		BuildKitReady: false,
	}

	// Check if we're on macOS
	if runtime.GOOS != "darwin" {
		result.Error = "Lima is only supported on macOS"
		return result, nil
	}

	// Check if Lima is installed
	if _, err := exec.LookPath("limactl"); err != nil {
		result.Error = "Lima is not installed. Please run: brew install lima"
		result.SetupRequired = true
		return result, nil
	}

	result.Available = true

	// Check if Lima VM exists
	vmName := "shmocker-buildkit"
	cmd := exec.Command("limactl", "list", "--format", "{{.Name}}", vmName)
	output, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(output)) == "" {
		result.Error = fmt.Sprintf("Lima VM '%s' not found", vmName)
		result.SetupRequired = true
		return result, nil
	}

	result.VMExists = true

	// Check VM status
	cmd = exec.Command("limactl", "list", "--format", "{{.Status}}", vmName)
	output, err = cmd.Output()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to get VM status: %v", err)
		return result, nil
	}

	status := strings.TrimSpace(string(output))
	if status == "Running" {
		result.VMRunning = true
		
		// Check if BuildKit port is accessible
		result.BuildKitReady = isPortAccessible("127.0.0.1", 1234)
	} else {
		result.Error = fmt.Sprintf("Lima VM is not running (status: %s)", status)
	}

	return result, nil
}

// LimaDetectionResult contains the results of Lima detection
type LimaDetectionResult struct {
	Available      bool   `json:"available"`
	VMExists       bool   `json:"vm_exists"`
	VMRunning      bool   `json:"vm_running"`
	BuildKitReady  bool   `json:"buildkit_ready"`
	SetupRequired  bool   `json:"setup_required"`
	Error          string `json:"error,omitempty"`
}

// IsReady returns true if Lima is fully ready for use
func (r *LimaDetectionResult) IsReady() bool {
	return r.Available && r.VMExists && r.VMRunning && r.BuildKitReady
}

// GetSetupInstructions returns instructions for setting up Lima
func (r *LimaDetectionResult) GetSetupInstructions() []string {
	var instructions []string

	if !r.Available {
		instructions = append(instructions, "1. Install Lima: brew install lima")
	}

	if !r.VMExists || r.SetupRequired {
		instructions = append(instructions, "2. Run the setup script: ./scripts/setup-macos.sh")
	}

	if !r.VMRunning && r.VMExists {
		instructions = append(instructions, "3. Start the Lima VM: limactl start shmocker-buildkit")
	}

	if !r.BuildKitReady && r.VMRunning {
		instructions = append(instructions, "4. Wait for BuildKit to be ready (may take a few seconds)")
		instructions = append(instructions, "5. Check BuildKit status: ./scripts/lima-vm.sh buildctl debug workers")
	}

	return instructions
}

// StartVM starts the Lima VM if it exists but is not running
func StartVM(vmName string) error {
	cmd := exec.Command("limactl", "start", vmName)
	return cmd.Run()
}

// StopVM stops the Lima VM
func StopVM(vmName string) error {
	cmd := exec.Command("limactl", "stop", vmName)
	return cmd.Run()
}

// GetVMStatus returns the current status of the Lima VM
func GetVMStatus(vmName string) (string, error) {
	cmd := exec.Command("limactl", "list", "--format", "{{.Status}}", vmName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Helper functions

// isPortAccessible checks if a port is accessible
func isPortAccessible(host string, port int) bool {
	// This would normally use net.Dial, but to avoid the import in this context,
	// we'll use a simple command-based check
	cmd := exec.Command("nc", "-z", host, fmt.Sprintf("%d", port))
	err := cmd.Run()
	return err == nil
}
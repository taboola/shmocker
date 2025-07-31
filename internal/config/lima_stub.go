//go:build !darwin
// +build !darwin

package config

import "errors"

// LimaConfig holds Lima-specific configuration (stub for non-Darwin)
type LimaConfig struct {
	VMName           string `yaml:"vm_name" json:"vm_name"`
	AutoStart        bool   `yaml:"auto_start" json:"auto_start"`
	BuildKitPort     int    `yaml:"buildkit_port" json:"buildkit_port"`
	BuildKitAddress  string `yaml:"buildkit_address" json:"buildkit_address"`
	SetupScriptPath  string `yaml:"setup_script_path" json:"setup_script_path"`
}

// DefaultLimaConfig returns the default Lima configuration (stub)
func DefaultLimaConfig() *LimaConfig {
	return &LimaConfig{
		VMName:          "shmocker-buildkit",
		AutoStart:       false,
		BuildKitPort:    1234,
		BuildKitAddress: "tcp://127.0.0.1:1234",
		SetupScriptPath: "scripts/setup-macos.sh",
	}
}

// LimaDetectionResult contains the results of Lima detection (stub)
type LimaDetectionResult struct {
	Available      bool   `json:"available"`
	VMExists       bool   `json:"vm_exists"`
	VMRunning      bool   `json:"vm_running"`
	BuildKitReady  bool   `json:"buildkit_ready"`
	SetupRequired  bool   `json:"setup_required"`
	Error          string `json:"error,omitempty"`
}

// DetectLima checks if Lima is available (stub for non-Darwin)
func DetectLima() (*LimaDetectionResult, error) {
	return &LimaDetectionResult{
		Available:     false,
		VMExists:      false,
		VMRunning:     false,
		BuildKitReady: false,
		SetupRequired: false,
		Error:         "Lima is only supported on macOS",
	}, nil
}

// IsReady returns false for non-Darwin systems
func (r *LimaDetectionResult) IsReady() bool {
	return false
}

// GetSetupInstructions returns empty instructions for non-Darwin systems
func (r *LimaDetectionResult) GetSetupInstructions() []string {
	return []string{"Lima is only supported on macOS"}
}

// StartVM is not supported on non-Darwin systems
func StartVM(vmName string) error {
	return errors.New("Lima is only supported on macOS")
}

// StopVM is not supported on non-Darwin systems
func StopVM(vmName string) error {
	return errors.New("Lima is only supported on macOS")
}

// GetVMStatus is not supported on non-Darwin systems
func GetVMStatus(vmName string) (string, error) {
	return "", errors.New("Lima is only supported on macOS")
}
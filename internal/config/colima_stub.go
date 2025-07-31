//go:build !darwin
// +build !darwin

package config

import "errors"

// ColimaConfig holds Colima-specific configuration (stub for non-macOS)
type ColimaConfig struct {
	Profile         string `yaml:"profile" json:"profile"`
	AutoStart       bool   `yaml:"auto_start" json:"auto_start"`
	BuildKitSocket  string `yaml:"buildkit_socket" json:"buildkit_socket"`
	SetupScriptPath string `yaml:"setup_script_path" json:"setup_script_path"`
	CPU             int    `yaml:"cpu" json:"cpu"`
	Memory          int    `yaml:"memory" json:"memory"`
	Disk            int    `yaml:"disk" json:"disk"`
}

// ColimaDetectionResult contains the results of Colima detection (stub for non-macOS)
type ColimaDetectionResult struct {
	Available      bool   `json:"available"`
	ProfileExists  bool   `json:"profile_exists"`
	ProfileRunning bool   `json:"profile_running"`
	BuildKitReady  bool   `json:"buildkit_ready"`
	SetupRequired  bool   `json:"setup_required"`
	Error          string `json:"error,omitempty"`
}

// ColimaProfileConfig represents the configuration for a Colima profile (stub for non-macOS)
type ColimaProfileConfig struct {
	CPU       int                    `yaml:"cpu" json:"cpu"`
	Memory    int                    `yaml:"memory" json:"memory"`
	Disk      int                    `yaml:"disk" json:"disk"`
	VMType    string                 `yaml:"vmType" json:"vmType"`
	Runtime   string                 `yaml:"runtime" json:"runtime"`
	Network   ColimaNetworkConfig    `yaml:"network" json:"network"`
	Mounts    []ColimaMountConfig    `yaml:"mounts" json:"mounts"`
	Provision []ColimaProvisionConfig `yaml:"provision" json:"provision"`
}

// ColimaNetworkConfig represents network configuration for Colima (stub for non-macOS)
type ColimaNetworkConfig struct {
	Address  bool              `yaml:"address" json:"address"`
	DNS      []string          `yaml:"dns" json:"dns"`
	DNSHosts map[string]string `yaml:"dnsHosts" json:"dnsHosts"`
}

// ColimaMountConfig represents mount configuration for Colima (stub for non-macOS)
type ColimaMountConfig struct {
	Location   string `yaml:"location" json:"location"`
	MountPoint string `yaml:"mountPoint" json:"mountPoint"`
	Writable   bool   `yaml:"writable" json:"writable"`
}

// ColimaProvisionConfig represents provisioning script configuration (stub for non-macOS)
type ColimaProvisionConfig struct {
	Mode   string `yaml:"mode" json:"mode"`
	Script string `yaml:"script" json:"script"`
}

// DefaultColimaConfig returns the default Colima configuration (stub for non-macOS)
func DefaultColimaConfig() *ColimaConfig {
	return &ColimaConfig{
		Profile:         "shmocker",
		AutoStart:       false,
		BuildKitSocket:  "unix:///run/user/1000/buildkit/buildkitd.sock",
		SetupScriptPath: "scripts/setup-macos-colima.sh",
		CPU:             4,
		Memory:          8,
		Disk:            64,
	}
}

// DetectColima returns an error on non-macOS systems
func DetectColima() (*ColimaDetectionResult, error) {
	return &ColimaDetectionResult{
		Available: false,
		Error:     "Colima is only supported on macOS",
	}, nil
}

// IsReady always returns false on non-macOS systems
func (r *ColimaDetectionResult) IsReady() bool {
	return false
}

// GetSetupInstructions returns appropriate message for non-macOS systems
func (r *ColimaDetectionResult) GetSetupInstructions() []string {
	return []string{"Colima is only supported on macOS"}
}

// StartProfile returns an error on non-macOS systems
func StartProfile(profileName string) error {
	return errors.New("Colima is only supported on macOS")
}

// StopProfile returns an error on non-macOS systems
func StopProfile(profileName string) error {
	return errors.New("Colima is only supported on macOS")
}

// GetProfileStatus returns an error on non-macOS systems
func GetProfileStatus(profileName string) (string, error) {
	return "", errors.New("Colima is only supported on macOS")
}

// GetProfileInfo returns an error on non-macOS systems
func GetProfileInfo(profileName string) (map[string]interface{}, error) {
	return nil, errors.New("Colima is only supported on macOS")
}

// RestartProfile returns an error on non-macOS systems
func RestartProfile(profileName string) error {
	return errors.New("Colima is only supported on macOS")
}

// DeleteProfile returns an error on non-macOS systems
func DeleteProfile(profileName string) error {
	return errors.New("Colima is only supported on macOS")
}

// GetColimaVersion returns an error on non-macOS systems
func GetColimaVersion() (string, error) {
	return "", errors.New("Colima is only supported on macOS")
}

// ListProfiles returns an error on non-macOS systems
func ListProfiles() ([]string, error) {
	return nil, errors.New("Colima is only supported on macOS")
}

// GetBuildKitStatus returns an error on non-macOS systems
func GetBuildKitStatus(profileName string) (string, error) {
	return "", errors.New("Colima is only supported on macOS")
}

// GetBuildKitLogs returns an error on non-macOS systems
func GetBuildKitLogs(profileName string, lines int) (string, error) {
	return "", errors.New("Colima is only supported on macOS")
}

// RunBuildCtl returns an error on non-macOS systems
func RunBuildCtl(profileName string, args ...string) (string, error) {
	return "", errors.New("Colima is only supported on macOS")
}

// ColimaEnvironment returns empty environment on non-macOS systems
func ColimaEnvironment(profileName string) map[string]string {
	return map[string]string{}
}

// GenerateColimaConfig returns nil configuration on non-macOS systems
func GenerateColimaConfig(cpu, memory, disk int) *ColimaProfileConfig {
	return nil
}
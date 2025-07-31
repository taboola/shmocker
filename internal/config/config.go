// Package config provides configuration management for shmocker.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration.
type Config struct {
	// Build settings
	DefaultPlatform string            `mapstructure:"default_platform"`
	BuildArgs       map[string]string `mapstructure:"build_args"`
	
	// Registry settings
	Registries map[string]RegistryConfig `mapstructure:"registries"`
	DefaultRegistry string `mapstructure:"default_registry"`
	
	// Cache settings
	CacheDir string `mapstructure:"cache_dir"`
	CacheType string `mapstructure:"cache_type"`
	
	// Security settings
	SigningEnabled bool   `mapstructure:"signing_enabled"`
	SBOMEnabled    bool   `mapstructure:"sbom_enabled"`
	KeyPath        string `mapstructure:"key_path"`
	
	// BuildKit settings
	BuildKitRoot     string `mapstructure:"buildkit_root"`
	BuildKitDataRoot string `mapstructure:"buildkit_data_root"`
	Debug            bool   `mapstructure:"debug"`
}

// RegistryConfig contains registry-specific configuration.
type RegistryConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Token    string `mapstructure:"token"`
	Insecure bool   `mapstructure:"insecure"`
	CACert   string `mapstructure:"ca_cert"`
	SkipTLS  bool   `mapstructure:"skip_tls"`
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	
	// Set defaults
	v.SetDefault("default_platform", "linux/amd64")
	v.SetDefault("cache_dir", filepath.Join(homeDir(), ".shmocker", "cache"))
	v.SetDefault("cache_type", "local")
	v.SetDefault("signing_enabled", false)
	v.SetDefault("sbom_enabled", false)
	v.SetDefault("buildkit_root", filepath.Join(homeDir(), ".shmocker", "buildkit"))
	v.SetDefault("buildkit_data_root", filepath.Join(homeDir(), ".shmocker", "buildkit", "data"))
	v.SetDefault("debug", false)
	v.SetDefault("default_registry", "docker.io")
	
	// Configure viper for environment variables
	v.SetEnvPrefix("SHMOCKER")
	v.AutomaticEnv()
	
	// Bind specific environment variables
	v.BindEnv("default_platform", "SHMOCKER_DEFAULT_PLATFORM")
	v.BindEnv("debug", "SHMOCKER_DEBUG")
	v.BindEnv("signing_enabled", "SHMOCKER_SIGNING_ENABLED")
	v.BindEnv("sbom_enabled", "SHMOCKER_SBOM_ENABLED")
	v.BindEnv("key_path", "SHMOCKER_KEY_PATH")
	
	// Configure config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Search for config file in standard locations
		v.SetConfigName(".shmocker")
		v.SetConfigType("json")
		v.AddConfigPath(homeDir())
		v.AddConfigPath(filepath.Join(homeDir(), ".shmocker"))
		v.AddConfigPath(".")
	}
	
	// Read config file if it exists
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}
	
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Apply environment variable overrides for registry authentication
	if config.Registries == nil {
		config.Registries = make(map[string]RegistryConfig)
	}
	
	// Set up default Docker Hub registry config if not exists
	if _, exists := config.Registries["docker.io"]; !exists {
		config.Registries["docker.io"] = RegistryConfig{
			URL: "https://index.docker.io/v1/",
		}
	}
	
	// Override with environment variables
	if username := os.Getenv("SHMOCKER_REGISTRY_USERNAME"); username != "" {
		reg := config.Registries["docker.io"]
		reg.Username = username
		config.Registries["docker.io"] = reg
	} else if username := os.Getenv("DOCKER_USERNAME"); username != "" {
		reg := config.Registries["docker.io"]
		reg.Username = username
		config.Registries["docker.io"] = reg
	}
	
	if password := os.Getenv("SHMOCKER_REGISTRY_PASSWORD"); password != "" {
		reg := config.Registries["docker.io"]
		reg.Password = password
		config.Registries["docker.io"] = reg
	} else if password := os.Getenv("DOCKER_PASSWORD"); password != "" {
		reg := config.Registries["docker.io"]
		reg.Password = password
		config.Registries["docker.io"] = reg
	}
	
	if token := os.Getenv("SHMOCKER_REGISTRY_TOKEN"); token != "" {
		reg := config.Registries["docker.io"]
		reg.Token = token
		config.Registries["docker.io"] = reg
	} else if token := os.Getenv("DOCKER_TOKEN"); token != "" {
		reg := config.Registries["docker.io"]
		reg.Token = token
		config.Registries["docker.io"] = reg
	}
	
	return &config, nil
}

// GetRegistryConfig returns the registry configuration for a given registry hostname
func (c *Config) GetRegistryConfig(registry string) (RegistryConfig, bool) {
	if c.Registries == nil {
		return RegistryConfig{}, false
	}
	
	config, exists := c.Registries[registry]
	if !exists && registry == "docker.io" {
		// Return default Docker Hub config
		return RegistryConfig{
			URL: "https://index.docker.io/v1/",
		}, true
	}
	
	return config, exists
}

// GetBuildKitRoot returns the BuildKit root directory
func (c *Config) GetBuildKitRoot() string {
	if c.BuildKitRoot != "" {
		return c.BuildKitRoot
	}
	// Use project-local directory
	workDir, err := os.Getwd()
	if err != nil {
		return ".shmocker/buildkit"
	}
	return filepath.Join(workDir, ".shmocker", "buildkit")
}

// GetBuildKitDataRoot returns the BuildKit data root directory
func (c *Config) GetBuildKitDataRoot() string {
	if c.BuildKitDataRoot != "" {
		return c.BuildKitDataRoot
	}
	return filepath.Join(c.GetBuildKitRoot(), "data")
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return home
}
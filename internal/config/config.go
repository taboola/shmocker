// Package config provides configuration management for shmocker.
package config

import (
	"fmt"
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
	
	// Cache settings
	CacheDir string `mapstructure:"cache_dir"`
	
	// Security settings
	SigningEnabled bool   `mapstructure:"signing_enabled"`
	SBOMEnabled    bool   `mapstructure:"sbom_enabled"`
	KeyPath        string `mapstructure:"key_path"`
}

// RegistryConfig contains registry-specific configuration.
type RegistryConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Insecure bool   `mapstructure:"insecure"`
}

// Load loads configuration from file and environment variables.
func Load(configPath string) (*Config, error) {
	v := viper.New()
	
	// Set defaults
	v.SetDefault("default_platform", "linux/amd64")
	v.SetDefault("cache_dir", filepath.Join(homeDir(), ".shmocker", "cache"))
	v.SetDefault("signing_enabled", false)
	v.SetDefault("sbom_enabled", false)
	
	// Configure viper
	v.SetConfigFile(configPath)
	v.SetEnvPrefix("SHMOCKER")
	v.AutomaticEnv()
	
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
	
	return &config, nil
}

func homeDir() string {
	// TODO: Implement proper home directory detection
	return "/tmp"
}
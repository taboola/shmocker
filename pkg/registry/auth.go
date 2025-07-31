// Package registry provides authentication functionality for container registries.
package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// DockerConfigAuth represents Docker config authentication.
type DockerConfigAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

// DockerConfig represents the Docker configuration file format.
type DockerConfig struct {
	Auths       map[string]DockerConfigAuth `json:"auths,omitempty"`
	CredsStore  string                      `json:"credsStore,omitempty"`
	CredHelpers map[string]string           `json:"credHelpers,omitempty"`
}

// RegistryAuthProvider provides enhanced authentication for various registry types.
type RegistryAuthProvider struct {
	credentials map[string]*Credentials
	tokens      map[string]*Token
	config      *AuthConfig
	httpClient  *http.Client
}

// AuthConfig contains authentication configuration.  
type AuthConfig struct {
	// ConfigPaths are paths to Docker config files to check
	ConfigPaths []string `json:"config_paths,omitempty"`
	
	// CredentialsStore is the name of the credentials store to use
	CredentialsStore string `json:"credentials_store,omitempty"`
	
	// InsecureRegistries is a list of registries to skip TLS verification
	InsecureRegistries []string `json:"insecure_registries,omitempty"`
	
	// TokenEndpoints maps registry hostnames to OAuth2 token endpoints
	TokenEndpoints map[string]string `json:"token_endpoints,omitempty"`
	
	// DefaultNamespace is the default namespace for registry operations
	DefaultNamespace string `json:"default_namespace,omitempty"`
}

// NewRegistryAuthProvider creates a new registry authentication provider.
func NewRegistryAuthProvider(config *AuthConfig) (*RegistryAuthProvider, error) {
	if config == nil {
		config = &AuthConfig{
			ConfigPaths: []string{
				filepath.Join(os.Getenv("HOME"), ".docker", "config.json"),
				filepath.Join(os.Getenv("HOME"), ".config", "containers", "auth.json"),
			},
			TokenEndpoints: map[string]string{
				"registry-1.docker.io": "https://auth.docker.io/token",
				"ghcr.io":              "https://ghcr.io/token",
				"quay.io":              "https://quay.io/v2/auth",
			},
		}
	}

	provider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		tokens:      make(map[string]*Token),
		config:      config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Load credentials from Docker config files
	if err := provider.loadDockerConfigs(); err != nil {
		return nil, errors.Wrap(err, "failed to load Docker configs")
	}

	return provider, nil
}

// GetCredentials returns credentials for the given registry.
func (p *RegistryAuthProvider) GetCredentials(ctx context.Context, registry string) (*Credentials, error) {
	// Normalize registry hostname
	hostname := p.normalizeRegistry(registry)
	
	// Check for cached credentials
	if creds, exists := p.credentials[hostname]; exists {
		// Check if token needs refresh
		if creds.ExpiresAt != nil && time.Now().After(*creds.ExpiresAt) {
			// Try to refresh token
			if refreshedToken, err := p.RefreshToken(ctx, hostname); err == nil {
				creds.Token = refreshedToken.AccessToken
				if refreshedToken.ExpiresIn > 0 {
					expiresAt := time.Now().Add(time.Duration(refreshedToken.ExpiresIn) * time.Second)
					creds.ExpiresAt = &expiresAt
				}
			}
		}
		return creds, nil
	}

	// Try to get token from OAuth2 endpoint if supported
	if token, err := p.getOAuth2Token(ctx, hostname); err == nil {
		creds := &Credentials{
			Registry: hostname,
			Token:    token.AccessToken,
		}
		if token.ExpiresIn > 0 {
			expiresAt := time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
			creds.ExpiresAt = &expiresAt
		}
		p.credentials[hostname] = creds
		return creds, nil
	}

	// Check environment variables for specific registries
	if creds := p.getCredentialsFromEnv(hostname); creds != nil {
		p.credentials[hostname] = creds
		return creds, nil
	}

	return nil, errors.Errorf("no credentials found for registry %s", hostname)
}

// RefreshToken refreshes an access token if needed.
func (p *RegistryAuthProvider) RefreshToken(ctx context.Context, registry string) (*Token, error) {
	hostname := p.normalizeRegistry(registry)
	
	// Check if we have a refresh token
	if cachedToken, exists := p.tokens[hostname]; exists && cachedToken.RefreshToken != "" {
		return p.refreshOAuth2Token(ctx, hostname, cachedToken.RefreshToken)
	}

	// Try to get a new token
	return p.getOAuth2Token(ctx, hostname)
}

// ClearCredentials clears cached credentials for a registry.
func (p *RegistryAuthProvider) ClearCredentials(registry string) {
	hostname := p.normalizeRegistry(registry)
	delete(p.credentials, hostname)
	delete(p.tokens, hostname)
}

// SetCredentials sets credentials for a registry.
func (p *RegistryAuthProvider) SetCredentials(registry string, creds *Credentials) {
	hostname := p.normalizeRegistry(registry)
	p.credentials[hostname] = creds
}

// loadDockerConfigs loads credentials from Docker configuration files.
func (p *RegistryAuthProvider) loadDockerConfigs() error {
	for _, configPath := range p.config.ConfigPaths {
		if err := p.loadDockerConfig(configPath); err != nil {
			// Log but don't fail if config file doesn't exist or is invalid
			continue
		}
	}
	return nil
}

// loadDockerConfig loads credentials from a single Docker configuration file.
func (p *RegistryAuthProvider) loadDockerConfig(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // File doesn't exist, skip
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return errors.Wrapf(err, "failed to read config file %s", configPath)
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return errors.Wrapf(err, "failed to parse config file %s", configPath)
	}

	// Process auth entries
	for registry, auth := range config.Auths {
		hostname := p.normalizeRegistry(registry)
		
		var username, password string
		
		if auth.Auth != "" {
			// Decode base64 auth string
			decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
			if err != nil {
				continue // Skip invalid auth
			}
			
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) == 2 {
				username = parts[0]
				password = parts[1]
			}
		} else {
			username = auth.Username
			password = auth.Password
		}

		if username != "" && password != "" {
			p.credentials[hostname] = &Credentials{
				Registry: hostname,
				Username: username,
				Password: password,
			}
		}
	}

	return nil
}

// getCredentialsFromEnv gets credentials from environment variables.
func (p *RegistryAuthProvider) getCredentialsFromEnv(hostname string) *Credentials {
	// Check for registry-specific environment variables
	var username, password, token string
	
	// Docker Hub
	if hostname == "registry-1.docker.io" || hostname == "docker.io" {
		username = os.Getenv("DOCKER_USERNAME")
		password = os.Getenv("DOCKER_PASSWORD")
		token = os.Getenv("DOCKER_TOKEN")
	}
	
	// GitHub Container Registry
	if hostname == "ghcr.io" {
		username = os.Getenv("GHCR_USERNAME")
		password = os.Getenv("GHCR_PASSWORD")
		token = os.Getenv("GHCR_TOKEN")
		// GitHub personal access tokens can be used as username
		if token != "" && username == "" {
			username = token
			password = token
		}
	}
	
	// Amazon ECR
	if strings.Contains(hostname, ".ecr.") && strings.Contains(hostname, ".amazonaws.com") {
		// ECR uses AWS credentials, which should be handled by AWS SDK
		// For now, check for ECR-specific token
		token = os.Getenv("ECR_TOKEN")
		if token != "" {
			return &Credentials{
				Registry: hostname,
				Token:    token,
			}
		}
	}
	
	// Generic registry credentials
	if username == "" && password == "" && token == "" {
		username = os.Getenv("REGISTRY_USERNAME")
		password = os.Getenv("REGISTRY_PASSWORD")
		token = os.Getenv("REGISTRY_TOKEN")
	}

	if token != "" {
		return &Credentials{
			Registry: hostname,
			Token:    token,
		}
	}
	
	if username != "" && password != "" {
		return &Credentials{
			Registry: hostname,
			Username: username,
			Password: password,
		}
	}

	return nil
}

// getOAuth2Token gets an OAuth2 token from the registry's token endpoint.
func (p *RegistryAuthProvider) getOAuth2Token(ctx context.Context, hostname string) (*Token, error) {
	tokenEndpoint, exists := p.config.TokenEndpoints[hostname]
	if !exists {
		return nil, errors.Errorf("no token endpoint configured for %s", hostname)
	}

	// Prepare token request
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("service", hostname)
	data.Set("scope", "repository:*:*") // Request broad permissions

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create token request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "shmocker/1.0")

	// Add basic auth if we have username/password
	if creds := p.getCredentialsFromEnv(hostname); creds != nil && creds.Username != "" {
		req.SetBasicAuth(creds.Username, creds.Password)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "token request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read token response")
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, errors.Wrap(err, "failed to parse token response")
	}

	// Cache the token
	p.tokens[hostname] = &token

	return &token, nil
}

// refreshOAuth2Token refreshes an OAuth2 token using a refresh token.
func (p *RegistryAuthProvider) refreshOAuth2Token(ctx context.Context, hostname, refreshToken string) (*Token, error) {
	tokenEndpoint, exists := p.config.TokenEndpoints[hostname]
	if !exists {
		return nil, errors.Errorf("no token endpoint configured for %s", hostname)
	}

	// Prepare refresh request
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create refresh request")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "shmocker/1.0")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "refresh request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh request failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read refresh response")
	}

	var token Token
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, errors.Wrap(err, "failed to parse refresh response")
	}

	// Cache the new token
	p.tokens[hostname] = &token

	return &token, nil
}

// normalizeRegistry normalizes a registry hostname for consistent lookup.
func (p *RegistryAuthProvider) normalizeRegistry(registry string) string {
	// Remove protocol if present
	if strings.HasPrefix(registry, "http://") {
		registry = strings.TrimPrefix(registry, "http://")
	}
	if strings.HasPrefix(registry, "https://") {
		registry = strings.TrimPrefix(registry, "https://")
	}

	// Remove port if it's the default
	if strings.HasSuffix(registry, ":443") {
		registry = strings.TrimSuffix(registry, ":443")
	}
	if strings.HasSuffix(registry, ":80") {
		registry = strings.TrimSuffix(registry, ":80")
	}

	// Handle Docker Hub special cases
	if registry == "docker.io" || registry == "index.docker.io" {
		return "registry-1.docker.io"
	}

	return registry
}

// SupportedRegistries returns a list of well-known registries with built-in support.
func (p *RegistryAuthProvider) SupportedRegistries() []string {
	return []string{
		"registry-1.docker.io", // Docker Hub
		"ghcr.io",              // GitHub Container Registry
		"quay.io",              // Red Hat Quay
		"gcr.io",               // Google Container Registry
		"us.gcr.io",            // Google Container Registry (US)
		"eu.gcr.io",            // Google Container Registry (EU)
		"asia.gcr.io",          // Google Container Registry (Asia)
		"*.azurecr.io",         // Azure Container Registry
		"*.dkr.ecr.*.amazonaws.com", // Amazon ECR
	}
}

// IsInsecureRegistry checks if a registry is marked as insecure.
func (p *RegistryAuthProvider) IsInsecureRegistry(registry string) bool {
	hostname := p.normalizeRegistry(registry)
	for _, insecure := range p.config.InsecureRegistries {
		if insecure == hostname {
			return true
		}
	}
	return false
}

// GetNamespace returns the appropriate namespace for a registry.
func (p *RegistryAuthProvider) GetNamespace(registry string) string {
	hostname := p.normalizeRegistry(registry)
	
	// Docker Hub uses library for official images
	if hostname == "registry-1.docker.io" {
		return "library"
	}
	
	// Use default namespace if configured
	if p.config.DefaultNamespace != "" {
		return p.config.DefaultNamespace
	}
	
	return ""
}

// ValidateCredentials validates credentials against a registry.
func (p *RegistryAuthProvider) ValidateCredentials(ctx context.Context, registry string) error {
	creds, err := p.GetCredentials(ctx, registry)
	if err != nil {
		return errors.Wrap(err, "failed to get credentials")
	}

	// Create a test request to validate credentials
	testURL := fmt.Sprintf("https://%s/v2/", p.normalizeRegistry(registry))
	req, err := http.NewRequestWithContext(ctx, "GET", testURL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create test request")
	}

	// Add authentication
	if creds.Token != "" {
		req.Header.Set("Authorization", "Bearer "+creds.Token)
	} else if creds.Username != "" && creds.Password != "" {
		req.SetBasicAuth(creds.Username, creds.Password)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "validation request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("validation failed with status: %d", resp.StatusCode)
	}

	return nil
}
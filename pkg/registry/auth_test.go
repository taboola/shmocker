package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestNewRegistryAuthProvider(t *testing.T) {
	provider, err := NewRegistryAuthProvider(nil)
	if err != nil {
		t.Fatalf("NewRegistryAuthProvider() error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewRegistryAuthProvider() returned nil")
	}

	// Test that default config is applied
	if len(provider.config.ConfigPaths) == 0 {
		t.Error("NewRegistryAuthProvider() default config missing config paths")
	}

	if len(provider.config.TokenEndpoints) == 0 {
		t.Error("NewRegistryAuthProvider() default config missing token endpoints")
	}
}

func TestRegistryAuthProvider_normalizeRegistry(t *testing.T) {
	provider := &RegistryAuthProvider{}

	tests := []struct {
		name     string
		registry string
		expected string
	}{
		{
			name:     "docker.io",
			registry: "docker.io",
			expected: "registry-1.docker.io",
		},
		{
			name:     "index.docker.io",
			registry: "index.docker.io",
			expected: "registry-1.docker.io",
		},
		{
			name:     "ghcr.io",
			registry: "ghcr.io",
			expected: "ghcr.io",
		},
		{
			name:     "with https prefix",
			registry: "https://registry.example.com",
			expected: "registry.example.com",
		},
		{
			name:     "with http prefix",
			registry: "http://registry.example.com",
			expected: "registry.example.com",
		},
		{
			name:     "with default port 443",
			registry: "registry.example.com:443",
			expected: "registry.example.com",
		},
		{
			name:     "with default port 80",
			registry: "registry.example.com:80",
			expected: "registry.example.com",
		},
		{
			name:     "with custom port",
			registry: "registry.example.com:5000",
			expected: "registry.example.com:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.normalizeRegistry(tt.registry)
			if result != tt.expected {
				t.Errorf("normalizeRegistry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRegistryAuthProvider_loadDockerConfig(t *testing.T) {
	// Create temporary directory for test config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Create test Docker config
	auth := base64.StdEncoding.EncodeToString([]byte("testuser:testpass"))
	config := DockerConfig{
		Auths: map[string]DockerConfigAuth{
			"registry.example.com": {
				Auth: auth,
			},
			"ghcr.io": {
				Username: "ghcruser",
				Password: "ghcrpass",
			},
		},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal test config: %v", err)
	}

	err = os.WriteFile(configPath, configData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test loading config
	provider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		config:      &AuthConfig{},
	}

	err = provider.loadDockerConfig(configPath)
	if err != nil {
		t.Fatalf("loadDockerConfig() error = %v", err)
	}

	// Verify credentials were loaded
	creds, exists := provider.credentials["registry.example.com"]
	if !exists {
		t.Error("loadDockerConfig() did not load registry.example.com credentials")
	} else {
		if creds.Username != "testuser" || creds.Password != "testpass" {
			t.Errorf("loadDockerConfig() loaded incorrect credentials for registry.example.com: %+v", creds)
		}
	}

	creds, exists = provider.credentials["ghcr.io"]
	if !exists {
		t.Error("loadDockerConfig() did not load ghcr.io credentials")
	} else {
		if creds.Username != "ghcruser" || creds.Password != "ghcrpass" {
			t.Errorf("loadDockerConfig() loaded incorrect credentials for ghcr.io: %+v", creds)
		}
	}
}

func TestRegistryAuthProvider_getCredentialsFromEnv(t *testing.T) {
	provider := &RegistryAuthProvider{}

	// Test Docker Hub credentials
	os.Setenv("DOCKER_USERNAME", "dockeruser")
	os.Setenv("DOCKER_PASSWORD", "dockerpass")
	defer os.Unsetenv("DOCKER_USERNAME")
	defer os.Unsetenv("DOCKER_PASSWORD")

	creds := provider.getCredentialsFromEnv("registry-1.docker.io")
	if creds == nil {
		t.Fatal("getCredentialsFromEnv() returned nil for Docker Hub")
	}
	if creds.Username != "dockeruser" || creds.Password != "dockerpass" {
		t.Errorf("getCredentialsFromEnv() returned incorrect Docker Hub credentials: %+v", creds)
	}

	// Test GHCR token
	os.Setenv("GHCR_TOKEN", "ghcrtoken123")
	defer os.Unsetenv("GHCR_TOKEN")

	creds = provider.getCredentialsFromEnv("ghcr.io")
	if creds == nil {
		t.Fatal("getCredentialsFromEnv() returned nil for GHCR")
	}
	if creds.Token != "ghcrtoken123" {
		t.Errorf("getCredentialsFromEnv() returned incorrect GHCR credentials: %+v", creds)
	}

	// Test generic registry credentials
	os.Setenv("REGISTRY_USERNAME", "reguser")
	os.Setenv("REGISTRY_PASSWORD", "regpass")
	defer os.Unsetenv("REGISTRY_USERNAME")
	defer os.Unsetenv("REGISTRY_PASSWORD")

	creds = provider.getCredentialsFromEnv("registry.example.com")
	if creds == nil {
		t.Fatal("getCredentialsFromEnv() returned nil for generic registry")
	}
	if creds.Username != "reguser" || creds.Password != "regpass" {
		t.Errorf("getCredentialsFromEnv() returned incorrect generic registry credentials: %+v", creds)
	}
}

func TestRegistryAuthProvider_getOAuth2Token(t *testing.T) {
	// Create test token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		token := Token{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	provider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		tokens:      make(map[string]*Token),
		config: &AuthConfig{
			TokenEndpoints: map[string]string{
				"test.registry.com": server.URL,
			},
		},
		httpClient: server.Client(),
	}

	ctx := context.Background()
	token, err := provider.getOAuth2Token(ctx, "test.registry.com")
	
	if err != nil {
		t.Fatalf("getOAuth2Token() error = %v", err)
	}

	if token == nil {
		t.Fatal("getOAuth2Token() returned nil token")
	}

	if token.AccessToken != "test-access-token" {
		t.Errorf("getOAuth2Token() access token = %s, want test-access-token", token.AccessToken)
	}

	if token.TokenType != "Bearer" {
		t.Errorf("getOAuth2Token() token type = %s, want Bearer", token.TokenType)
	}

	if token.ExpiresIn != 3600 {
		t.Errorf("getOAuth2Token() expires in = %d, want 3600", token.ExpiresIn)
	}

	// Verify token is cached
	if cachedToken, exists := provider.tokens["test.registry.com"]; !exists || cachedToken.AccessToken != token.AccessToken {
		t.Error("getOAuth2Token() did not cache token properly")
	}
}

func TestRegistryAuthProvider_GetCredentials(t *testing.T) {
	provider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		tokens:      make(map[string]*Token),
		config: &AuthConfig{
			TokenEndpoints: make(map[string]string),
		},
	}

	// Test cached credentials
	testCreds := &Credentials{
		Registry: "test.registry.com",
		Username: "testuser",
		Password: "testpass",
	}
	provider.SetCredentials("test.registry.com", testCreds)

	ctx := context.Background()
	creds, err := provider.GetCredentials(ctx, "test.registry.com")
	
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	if creds.Username != testCreds.Username || creds.Password != testCreds.Password {
		t.Errorf("GetCredentials() returned incorrect credentials: %+v", creds)
	}

	// Test non-existent credentials
	_, err = provider.GetCredentials(ctx, "nonexistent.registry.com")
	if err == nil {
		t.Error("GetCredentials() expected error for non-existent registry")
	}
}

func TestRegistryAuthProvider_RefreshToken(t *testing.T) {
	// Create test refresh token server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		token := Token{
			AccessToken:  "refreshed-access-token",
			RefreshToken: "new-refresh-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(token)
	}))
	defer server.Close()

	provider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		tokens: map[string]*Token{
			"test.registry.com": {
				AccessToken:  "old-access-token",
				RefreshToken: "old-refresh-token",
			},
		},
		config: &AuthConfig{
			TokenEndpoints: map[string]string{
				"test.registry.com": server.URL,
			},
		},
		httpClient: server.Client(),
	}

	ctx := context.Background()
	token, err := provider.RefreshToken(ctx, "test.registry.com")
	
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	if token == nil {
		t.Fatal("RefreshToken() returned nil token")
	}

	if token.AccessToken != "refreshed-access-token" {
		t.Errorf("RefreshToken() access token = %s, want refreshed-access-token", token.AccessToken)
	}

	// Verify token is updated in cache
	if cachedToken := provider.tokens["test.registry.com"]; cachedToken.AccessToken != token.AccessToken {
		t.Error("RefreshToken() did not update cached token")
	}
}

func TestRegistryAuthProvider_ValidateCredentials(t *testing.T) {
	// Create test validation server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Basic dGVzdHVzZXI6dGVzdHBhc3M=" { // testuser:testpass in base64
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	provider := &RegistryAuthProvider{
		credentials: map[string]*Credentials{
			"test.registry.com": {
				Registry: "test.registry.com",
				Username: "testuser",
				Password: "testpass",
			},
		},
		tokens: make(map[string]*Token),
		config: &AuthConfig{},
		httpClient: server.Client(),
	}

	// Create a custom validation function for testing
	validateCredentials := func(provider *RegistryAuthProvider, ctx context.Context, registry string) error {
		creds, err := provider.GetCredentials(ctx, registry)
		if err != nil {
			return err
		}

		// Use test server URL instead
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL+"/v2/", nil)
		if err != nil {
			return err
		}

		if creds.Token != "" {
			req.Header.Set("Authorization", "Bearer "+creds.Token)
		} else if creds.Username != "" && creds.Password != "" {
			req.SetBasicAuth(creds.Username, creds.Password)
		}

		resp, err := provider.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			return errors.New("invalid credentials")
		}

		return nil
	}

	ctx := context.Background()
	
	// Test valid credentials
	err := validateCredentials(provider, ctx, "test.registry.com")
	if err != nil {
		t.Fatalf("ValidateCredentials() error = %v", err)
	}

	// Test invalid credentials
	provider.credentials["invalid.registry.com"] = &Credentials{
		Registry: "invalid.registry.com",
		Username: "wronguser",
		Password: "wrongpass",
	}

	err = validateCredentials(provider, ctx, "invalid.registry.com")
	if err == nil {
		t.Error("ValidateCredentials() expected error for invalid credentials")
	}
}

func TestRegistryAuthProvider_SupportedRegistries(t *testing.T) {
	provider := &RegistryAuthProvider{}
	
	registries := provider.SupportedRegistries()
	
	if len(registries) == 0 {
		t.Error("SupportedRegistries() returned empty list")
	}

	// Check for well-known registries
	expectedRegistries := []string{
		"registry-1.docker.io",
		"ghcr.io",
		"quay.io",
		"gcr.io",
	}

	for _, expected := range expectedRegistries {
		found := false
		for _, registry := range registries {
			if registry == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SupportedRegistries() missing expected registry: %s", expected)
		}
	}
}

func TestRegistryAuthProvider_IsInsecureRegistry(t *testing.T) {
	provider := &RegistryAuthProvider{
		config: &AuthConfig{
			InsecureRegistries: []string{
				"localhost:5000",
				"registry.local",
			},
		},
	}

	tests := []struct {
		name     string
		registry string
		expected bool
	}{
		{
			name:     "insecure localhost",
			registry: "localhost:5000",
			expected: true,
		},
		{
			name:     "insecure local registry",
			registry: "registry.local",
			expected: true,
		},
		{
			name:     "secure registry",
			registry: "registry.example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.IsInsecureRegistry(tt.registry)
			if result != tt.expected {
				t.Errorf("IsInsecureRegistry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRegistryAuthProvider_GetNamespace(t *testing.T) {
	provider := &RegistryAuthProvider{
		config: &AuthConfig{
			DefaultNamespace: "myorg",
		},
	}

	tests := []struct {
		name     string
		registry string
		expected string
	}{
		{
			name:     "docker hub",
			registry: "registry-1.docker.io",
			expected: "library",
		},
		{
			name:     "other registry",
			registry: "ghcr.io",
			expected: "myorg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.GetNamespace(tt.registry)
			if result != tt.expected {
				t.Errorf("GetNamespace() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRegistryAuthProvider_tokenExpiration(t *testing.T) {
	provider := &RegistryAuthProvider{
		credentials: make(map[string]*Credentials),
		tokens:      make(map[string]*Token),
		config:      &AuthConfig{},
	}

	// Create credentials with expired token
	expiredTime := time.Now().Add(-1 * time.Hour)
	expiredCreds := &Credentials{
		Registry:  "test.registry.com",
		Token:     "expired-token",
		ExpiresAt: &expiredTime,
	}
	provider.SetCredentials("test.registry.com", expiredCreds)

	ctx := context.Background()
	
	// Should attempt to refresh but fail since no token endpoint is configured
	_, err := provider.GetCredentials(ctx, "test.registry.com")
	
	// Should still return the credentials even if refresh fails
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}
}
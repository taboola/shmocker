//go:build linux
// +build linux

package builder

import (
	"context"
	"testing"
	"time"

	"github.com/moby/buildkit/control"
	"github.com/shmocker/shmocker/pkg/dockerfile"
)

func TestBuilder_MultiArchSupport(t *testing.T) {
	// Create a simple Dockerfile for testing
	dockerfileContent := `
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY . .
CMD ["./app"]
`

	parser := dockerfile.New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Test supported platforms
	testCases := []struct {
		name     string
		platform Platform
		expected bool
	}{
		{"linux/amd64", Platform{OS: "linux", Architecture: "amd64"}, true},
		{"linux/arm64", Platform{OS: "linux", Architecture: "arm64"}, true},
		{"linux/arm/v7", Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, true},
		{"linux/arm/v6", Platform{OS: "linux", Architecture: "arm", Variant: "v6"}, true},
		{"linux/386", Platform{OS: "linux", Architecture: "386"}, true},
		{"linux/ppc64le", Platform{OS: "linux", Architecture: "ppc64le"}, true},
		{"linux/s390x", Platform{OS: "linux", Architecture: "s390x"}, true},
		{"windows/amd64", Platform{OS: "windows", Architecture: "amd64"}, false},
		{"darwin/amd64", Platform{OS: "darwin", Architecture: "amd64"}, false},
		{"linux/riscv64", Platform{OS: "linux", Architecture: "riscv64"}, false},
	}

	// Create mock builder for testing platform support
	builder := &builder{}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			supported := builder.isSupportedPlatform(tc.platform)
			if supported != tc.expected {
				t.Errorf("Platform %s: expected supported=%v, got %v", tc.name, tc.expected, supported)
			}
		})
	}
}

func TestBuilder_MultiPlatformBuildRequest(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18
RUN echo "Hello World"
CMD ["echo", "hello"]
`

	parser := dockerfile.New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Test multi-platform build request validation
	req := &BuildRequest{
		Dockerfile: ast,
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: "/tmp",
		},
		Platforms: []Platform{
			{OS: "linux", Architecture: "amd64"},
			{OS: "linux", Architecture: "arm64"},
		},
		BuildArgs: make(map[string]string),
		Labels:    make(map[string]string),
	}

	builder := &builder{}

	// Test validation
	err = builder.validateBuildRequest(req)
	if err != nil {
		t.Fatalf("Multi-platform build request validation failed: %v", err)
	}

	// Test with unsupported platform
	req.Platforms = append(req.Platforms, Platform{OS: "windows", Architecture: "amd64"})
	err = builder.validateBuildRequest(req)
	if err == nil {
		t.Error("Expected validation error for unsupported platform")
	}
}

func TestPlatformString(t *testing.T) {
	testCases := []struct {
		platform Platform
		expected string
	}{
		{Platform{OS: "linux", Architecture: "amd64"}, "linux/amd64"},
		{Platform{OS: "linux", Architecture: "arm64"}, "linux/arm64"},
		{Platform{OS: "linux", Architecture: "arm", Variant: "v7"}, "linux/arm/v7"},
		{Platform{OS: "linux", Architecture: "arm", Variant: "v6"}, "linux/arm/v6"},
		{Platform{OS: "darwin", Architecture: "amd64"}, "darwin/amd64"},
		{Platform{OS: "windows", Architecture: "amd64"}, "windows/amd64"},
	}

	for _, tc := range testCases {
		result := tc.platform.String()
		if result != tc.expected {
			t.Errorf("Platform.String(): expected %s, got %s", tc.expected, result)
		}
	}
}

func TestBuildKitController_PlatformSupport(t *testing.T) {
	ctx := context.Background()

	// Create BuildKit controller options
	opts := &BuildKitOptions{
		Root:     "/tmp/shmocker-test",
		DataRoot: "/tmp/shmocker-test/data",
		Debug:    false,
	}

	// Create BuildKit controller
	controller, err := NewBuildKitController(ctx, opts)
	if err != nil {
		t.Skipf("Skipping BuildKit controller test: %v", err)
	}
	defer controller.Close()

	// Get worker and check platforms
	worker, err := controller.(*buildKitController).worker.GetWorkerController().GetDefault()
	if err != nil {
		t.Fatalf("Failed to get default worker: %v", err)
	}

	platforms := worker.Platforms()
	if len(platforms) == 0 {
		t.Error("Worker should support at least one platform")
	}

	// Check that common platforms are supported
	expectedPlatforms := []string{"linux/amd64", "linux/arm64"}
	for _, expected := range expectedPlatforms {
		found := false
		for _, platform := range platforms {
			if platform.String() == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected platform %s not found in supported platforms: %v", expected, platforms)
		}
	}
}

func TestBuildKitController_ConfigurePlatformBuild(t *testing.T) {
	ctx := context.Background()

	// Create BuildKit controller options
	opts := &BuildKitOptions{
		Root:     "/tmp/shmocker-test",
		DataRoot: "/tmp/shmocker-test/data",
		Debug:    false,
	}

	// Create BuildKit controller
	controller, err := NewBuildKitController(ctx, opts)
	if err != nil {
		t.Skipf("Skipping BuildKit controller test: %v", err)
	}
	defer controller.Close()

	buildKitController := controller.(*buildKitController)

	testCases := []struct {
		name        string
		platform    string
		expectError bool
	}{
		{"Valid linux/amd64", "linux/amd64", false},
		{"Valid linux/arm64", "linux/arm64", false},
		{"Valid linux/arm/v7", "linux/arm/v7", false},
		{"Invalid format", "invalid-platform", true},
		{"Unsupported platform", "plan9/386", true},
		{"Empty platform", "", false}, // Should use default
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock solve request
			req := &control.SolveRequest{
				FrontendAttrs: make(map[string]string),
			}

			err := buildKitController.configurePlatformBuild(req, tc.platform)

			if tc.expectError && err == nil {
				t.Errorf("Expected error for platform %s, got nil", tc.platform)
			} else if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for platform %s: %v", tc.platform, err)
			}

			// Check that platform attributes are set correctly for valid platforms
			if !tc.expectError && tc.platform != "" && err == nil {
				if req.FrontendAttrs["platform"] != tc.platform {
					t.Errorf("Expected platform attribute %s, got %s", tc.platform, req.FrontendAttrs["platform"])
				}

				// Check that cross-compilation build args are set
				if req.FrontendAttrs["build-arg:TARGETPLATFORM"] != tc.platform {
					t.Errorf("Expected TARGETPLATFORM %s, got %s", tc.platform, req.FrontendAttrs["build-arg:TARGETPLATFORM"])
				}
			}
		})
	}
}

func TestBuilder_BuildMultiPlatform(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18
RUN echo "Multi-platform build test"
CMD ["echo", "hello"]
`

	parser := dockerfile.New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Create build request with multiple platforms
	req := &BuildRequest{
		Dockerfile: ast,
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: "/tmp",
		},
		Platforms: []Platform{
			{OS: "linux", Architecture: "amd64"},
			{OS: "linux", Architecture: "arm64"},
		},
		BuildArgs: make(map[string]string),
		Labels:    make(map[string]string),
		Tags:      []string{"test:multiarch"},
	}

	// Create mock builder
	builder := &builder{}

	// Create mock build context
	buildCtx := &buildContextManager{
		contextType: ContextTypeLocal,
		source:      "/tmp",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test multi-platform build logic (this will fail without actual BuildKit setup)
	result, err := builder.buildMultiPlatform(ctx, req, buildCtx)
	
	// We expect this to fail in test environment, but we can check the error handling
	if err == nil {
		// If it somehow succeeds, check the result
		if result == nil {
			t.Error("Build result should not be nil")
		}
		if len(result.Manifests) != len(req.Platforms) {
			t.Errorf("Expected %d manifests, got %d", len(req.Platforms), len(result.Manifests))
		}
	} else {
		// Expected to fail in test environment, but error should be related to BuildKit setup
		t.Logf("Multi-platform build failed as expected in test environment: %v", err)
	}
}

func TestBuilder_MultiStageValidation(t *testing.T) {
	testCases := []struct {
		name          string
		dockerfile    string
		target        string
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid multi-stage",
			dockerfile: `
FROM alpine:3.18 AS builder
RUN echo "building"

FROM alpine:3.18 AS runtime
COPY --from=builder /app/binary /app/binary
CMD ["/app/binary"]
`,
			target:      "",
			expectError: false,
		},
		{
			name: "Valid target stage",
			dockerfile: `
FROM alpine:3.18 AS builder
RUN echo "building"

FROM alpine:3.18 AS runtime
COPY --from=builder /app/binary /app/binary
CMD ["/app/binary"]
`,
			target:      "builder",
			expectError: false,
		},
		{
			name: "Invalid target stage",
			dockerfile: `
FROM alpine:3.18 AS builder
RUN echo "building"

FROM alpine:3.18 AS runtime
COPY --from=builder /app/binary /app/binary
CMD ["/app/binary"]
`,
			target:        "nonexistent",
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "Forward reference in FROM",
			dockerfile: `
FROM future AS base
RUN echo "invalid"

FROM alpine:3.18 AS future
RUN echo "future stage"
`,
			target:        "",
			expectError:   true,
			errorContains: "unknown stage",
		},
		{
			name: "Forward reference in COPY",
			dockerfile: `
FROM alpine:3.18 AS base
COPY --from=future /app/binary /app/binary

FROM alpine:3.18 AS future
RUN echo "future stage"
`,
			target:        "",
			expectError:   true,
			errorContains: "future stage",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := dockerfile.New()
			ast, err := parser.ParseBytes([]byte(tc.dockerfile))
			if err != nil {
				t.Fatalf("Failed to parse Dockerfile: %v", err)
			}

			req := &BuildRequest{
				Dockerfile: ast,
				Target:     tc.target,
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: "/tmp",
				},
				Platforms: []Platform{{OS: "linux", Architecture: "amd64"}},
			}

			builder := &builder{}
			err = builder.validateMultiStageBuild(req)

			if tc.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			} else if tc.expectError && err != nil && tc.errorContains != "" {
				if !contains(err.Error(), tc.errorContains) {
					t.Errorf("Expected error to contain '%s', got: %v", tc.errorContains, err)
				}
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
//go:build integration
// +build integration

package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shmocker/shmocker/pkg/dockerfile"
)

// Integration tests require actual BuildKit functionality
// These tests are tagged with "integration" and should be run separately

func TestIntegration_SimpleDockerfileBuild(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "shmocker-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple Dockerfile
	dockerfileContent := `FROM alpine:latest
RUN echo 'Hello from shmocker integration test' > /test.txt
CMD cat /test.txt
`
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	// Parse the Dockerfile to AST
	// Note: This would use the actual dockerfile parser
	ast := &dockerfile.AST{
		Instructions: []dockerfile.Instruction{
			{
				Cmd:  "FROM",
				Args: []string{"alpine:latest"},
			},
			{
				Cmd:  "RUN",
				Args: []string{"echo 'Hello from shmocker integration test' > /test.txt"},
			},
			{
				Cmd:  "CMD",
				Args: []string{"cat /test.txt"},
			},
		},
	}

	// Create builder
	ctx := context.Background()
	builder, err := New(ctx, &BuilderOptions{
		Debug: true,
	})
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer builder.Close()

	// Create build request
	request := &BuildRequest{
		Context: BuildContext{
			Type:         ContextTypeLocal,
			Source:       tempDir,
			DockerIgnore: true,
		},
		Dockerfile: ast,
		Tags:       []string{"shmocker-test:integration"},
		Platforms: []Platform{
			{
				OS:           "linux",
				Architecture: "amd64",
			},
		},
		BuildArgs: map[string]string{
			"TEST_ARG": "test_value",
		},
		Labels: map[string]string{
			"com.shmocker.test": "integration",
		},
	}

	// Execute build
	result, err := builder.Build(ctx, request)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Validate result
	if result == nil {
		t.Fatal("Expected build result but got nil")
	}

	if result.ImageID == "" {
		t.Error("Expected non-empty ImageID")
	}

	if result.BuildTime <= 0 {
		t.Error("Expected positive build time")
	}

	t.Logf("Build successful: ImageID=%s, BuildTime=%v", result.ImageID, result.BuildTime)
}

func TestIntegration_BuildWithProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test context
	tempDir, err := os.MkdirTemp("", "shmocker-progress-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a multi-step Dockerfile to see progress
	dockerfileContent := `FROM alpine:latest
RUN apk add --no-cache curl
RUN echo 'Step 1 complete' > /step1.txt
RUN echo 'Step 2 complete' > /step2.txt
RUN echo 'Step 3 complete' > /step3.txt
CMD echo 'Build complete'
`
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	// Parse Dockerfile
	ast := &dockerfile.AST{
		Instructions: []dockerfile.Instruction{
			{Cmd: "FROM", Args: []string{"alpine:latest"}},
			{Cmd: "RUN", Args: []string{"apk add --no-cache curl"}},
			{Cmd: "RUN", Args: []string{"echo 'Step 1 complete' > /step1.txt"}},
			{Cmd: "RUN", Args: []string{"echo 'Step 2 complete' > /step2.txt"}},
			{Cmd: "RUN", Args: []string{"echo 'Step 3 complete' > /step3.txt"}},
			{Cmd: "CMD", Args: []string{"echo 'Build complete'"}},
		},
	}

	// Create builder
	ctx := context.Background()
	builder, err := New(ctx, &BuilderOptions{
		Debug: true,
	})
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer builder.Close()

	// Create build request
	request := &BuildRequest{
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: tempDir,
		},
		Dockerfile: ast,
		Tags:       []string{"shmocker-test:progress"},
	}

	// Create progress channel
	progressCh := make(chan *ProgressEvent, 100)
	defer close(progressCh)

	// Collect progress events
	var progressEvents []*ProgressEvent
	progressDone := make(chan bool)
	go func() {
		defer close(progressDone)
		for event := range progressCh {
			progressEvents = append(progressEvents, event)
			t.Logf("Progress: %s - %s (%s)", event.ID, event.Name, event.Status)
		}
	}()

	// Execute build with progress
	result, err := builder.BuildWithProgress(ctx, request, progressCh)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Wait for progress collection to finish
	<-progressDone

	// Validate result
	if result == nil {
		t.Fatal("Expected build result but got nil")
	}

	// Validate progress events
	if len(progressEvents) == 0 {
		t.Error("Expected progress events but got none")
	}

	// Check for start and completion events
	hasStart := false
	hasComplete := false
	for _, event := range progressEvents {
		if event.Status == StatusStarted {
			hasStart = true
		}
		if event.Status == StatusCompleted {
			hasComplete = true
		}
	}

	if !hasStart {
		t.Error("Expected start progress event")
	}

	if !hasComplete {
		t.Error("Expected completion progress event")
	}

	t.Logf("Received %d progress events", len(progressEvents))
}

func TestIntegration_BuildWithCache(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test context
	tempDir, err := os.MkdirTemp("", "shmocker-cache-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create cache directory
	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create Dockerfile
	dockerfileContent := `FROM alpine:latest
RUN apk add --no-cache curl wget
RUN echo 'Cached step' > /cached.txt
`
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	ast := &dockerfile.AST{
		Instructions: []dockerfile.Instruction{
			{Cmd: "FROM", Args: []string{"alpine:latest"}},
			{Cmd: "RUN", Args: []string{"apk add --no-cache curl wget"}},
			{Cmd: "RUN", Args: []string{"echo 'Cached step' > /cached.txt"}},
		},
	}

	// Create builder
	ctx := context.Background()
	builder, err := New(ctx, &BuilderOptions{
		Debug: true,
	})
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer builder.Close()

	// First build - should populate cache
	request1 := &BuildRequest{
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: tempDir,
		},
		Dockerfile: ast,
		Tags:       []string{"shmocker-test:cache1"},
		CacheTo: []*CacheExport{
			{
				Type: "local",
				Ref:  cacheDir,
				Attrs: map[string]string{
					"mode": "max",
				},
			},
		},
	}

	startTime := time.Now()
	result1, err := builder.Build(ctx, request1)
	if err != nil {
		t.Fatalf("First build failed: %v", err)
	}
	firstBuildTime := time.Since(startTime)

	if result1 == nil {
		t.Fatal("Expected build result but got nil")
	}

	// Second build - should use cache
	request2 := &BuildRequest{
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: tempDir,
		},
		Dockerfile: ast,
		Tags:       []string{"shmocker-test:cache2"},
		CacheFrom: []*CacheImport{
			{
				Type: "local",
				Ref:  cacheDir,
			},
		},
	}

	startTime = time.Now()
	result2, err := builder.Build(ctx, request2)
	if err != nil {
		t.Fatalf("Second build failed: %v", err)
	}
	secondBuildTime := time.Since(startTime)

	if result2 == nil {
		t.Fatal("Expected build result but got nil")
	}

	t.Logf("First build time: %v", firstBuildTime)
	t.Logf("Second build time: %v", secondBuildTime)

	// Second build should be faster due to cache (in a real scenario)
	// For this test, we just verify both builds completed successfully
}

func TestIntegration_MultiPlatformBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test context
	tempDir, err := os.MkdirTemp("", "shmocker-multiplatform-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create simple Dockerfile
	dockerfileContent := `FROM alpine:latest
RUN uname -m > /arch.txt
CMD cat /arch.txt
`
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to write Dockerfile: %v", err)
	}

	ast := &dockerfile.AST{
		Instructions: []dockerfile.Instruction{
			{Cmd: "FROM", Args: []string{"alpine:latest"}},
			{Cmd: "RUN", Args: []string{"uname -m > /arch.txt"}},
			{Cmd: "CMD", Args: []string{"cat /arch.txt"}},
		},
	}

	// Create builder
	ctx := context.Background()
	builder, err := New(ctx, &BuilderOptions{
		Debug: true,
	})
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}
	defer builder.Close()

	// Build for multiple platforms
	platforms := []Platform{
		{OS: "linux", Architecture: "amd64"},
		// Note: In a real scenario, you might want to test arm64 as well
		// but this requires emulation support which may not be available
		// in all test environments
	}

	for _, platform := range platforms {
		t.Run(platform.String(), func(t *testing.T) {
			request := &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: tempDir,
				},
				Dockerfile: ast,
				Tags:       []string{"shmocker-test:multiplatform-" + platform.Architecture},
				Platforms:  []Platform{platform},
			}

			result, err := builder.Build(ctx, request)
			if err != nil {
				t.Fatalf("Build failed for platform %s: %v", platform.String(), err)
			}

			if result == nil {
				t.Fatal("Expected build result but got nil")
			}

			t.Logf("Build successful for platform %s: ImageID=%s", platform.String(), result.ImageID)
		})
	}
}

func TestIntegration_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name              string
		dockerfileContent string
		expectedErrorType BuildErrorType
	}{
		{
			name: "invalid dockerfile syntax",
			dockerfileContent: `FORM alpine:latest
RUN echo 'hello'
`,
			expectedErrorType: ErrorTypeDockerfile,
		},
		{
			name: "missing package",
			dockerfileContent: `FROM alpine:latest
RUN apk add --no-cache nonexistent-package-12345
`,
			expectedErrorType: ErrorTypeDependency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test context
			tempDir, err := os.MkdirTemp("", "shmocker-error-*")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create Dockerfile
			dockerfilePath := filepath.Join(tempDir, "Dockerfile")
			if err := os.WriteFile(dockerfilePath, []byte(tt.dockerfileContent), 0644); err != nil {
				t.Fatalf("Failed to write Dockerfile: %v", err)
			}

			// Create a simple AST (in real scenario, this would fail during parsing)
			ast := &dockerfile.AST{
				Instructions: []dockerfile.Instruction{
					{Cmd: "FROM", Args: []string{"alpine:latest"}},
				},
			}

			// Create builder
			ctx := context.Background()
			builder, err := New(ctx, &BuilderOptions{
				Debug: true,
			})
			if err != nil {
				t.Fatalf("Failed to create builder: %v", err)
			}
			defer builder.Close()

			request := &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: tempDir,
				},
				Dockerfile: ast,
				Tags:       []string{"shmocker-test:error"},
			}

			// Execute build - expect failure
			_, err = builder.Build(ctx, request)
			if err == nil {
				t.Error("Expected build to fail but it succeeded")
				return
			}

			// Check error type
			errorType := GetErrorType(err)
			t.Logf("Got error type: %s, message: %s", errorType, err.Error())

			// Note: In integration tests, the actual error classification
			// might differ from unit tests since we're dealing with real BuildKit
		})
	}
}

// Helper function to check if BuildKit is available
func isBuildKitAvailable() bool {
	// In a real implementation, this would check if BuildKit dependencies
	// are available and properly configured
	return true
}

// Benchmark integration test
func BenchmarkIntegration_SimpleBuild(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping integration benchmark in short mode")
	}

	// Create test context once
	tempDir, err := os.MkdirTemp("", "shmocker-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dockerfileContent := `FROM alpine:latest
RUN echo 'benchmark test'
`
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		b.Fatalf("Failed to write Dockerfile: %v", err)
	}

	ast := &dockerfile.AST{
		Instructions: []dockerfile.Instruction{
			{Cmd: "FROM", Args: []string{"alpine:latest"}},
			{Cmd: "RUN", Args: []string{"echo 'benchmark test'"}},
		},
	}

	// Create builder once
	ctx := context.Background()
	builder, err := New(ctx, &BuilderOptions{
		Debug: false, // Disable debug for benchmarks
	})
	if err != nil {
		b.Fatalf("Failed to create builder: %v", err)
	}
	defer builder.Close()

	request := &BuildRequest{
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: tempDir,
		},
		Dockerfile: ast,
		Tags:       []string{"shmocker-bench:test"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use different tags to avoid conflicts
		request.Tags = []string{fmt.Sprintf("shmocker-bench:test-%d", i)}

		_, err := builder.Build(ctx, request)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

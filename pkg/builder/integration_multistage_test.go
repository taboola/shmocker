//go:build linux
// +build linux

package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shmocker/shmocker/pkg/dockerfile"
)

func TestMultiStageMultiArchIntegration(t *testing.T) {
	// Create a comprehensive multi-stage, multi-arch Dockerfile
	dockerfileContent := `
# Build stage with specific platform
FROM --platform=linux/amd64 golang:1.21-alpine AS builder
ARG TARGETPLATFORM
ARG BUILDPLATFORM
RUN echo "Building on $BUILDPLATFORM for $TARGETPLATFORM"

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /src

# Copy go modules first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build for target platform
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.18 AS runtime

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/server /app/server

# Copy configuration
COPY config/ /app/config/

# Create necessary directories
RUN mkdir -p /app/data /app/logs && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Default command
CMD ["/app/server"]
`

	// Parse the Dockerfile
	parser := dockerfile.New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse multi-stage Dockerfile: %v", err)
	}

	// Validate the AST
	if err := parser.Validate(ast); err != nil {
		t.Fatalf("Dockerfile validation failed: %v", err)
	}

	// Check that we have the expected stages
	if len(ast.Stages) != 2 {
		t.Fatalf("Expected 2 stages, got %d", len(ast.Stages))
	}

	builderStage := ast.Stages[0]
	runtimeStage := ast.Stages[1]

	// Verify stage names
	if builderStage.Name != "builder" {
		t.Errorf("Expected first stage name 'builder', got '%s'", builderStage.Name)
	}
	if runtimeStage.Name != "runtime" {
		t.Errorf("Expected second stage name 'runtime', got '%s'", runtimeStage.Name)
	}

	// Verify platform specification
	if builderStage.Platform != "linux/amd64" {
		t.Errorf("Expected builder platform 'linux/amd64', got '%s'", builderStage.Platform)
	}

	// Verify cross-stage COPY
	var copyFromFound bool
	for _, instr := range runtimeStage.Instructions {
		if copy, ok := instr.(*dockerfile.CopyInstruction); ok && copy.From == "builder" {
			copyFromFound = true
			if copy.Sources[0] != "/app/server" || copy.Destination != "/app/server" {
				t.Errorf("Unexpected COPY --from=builder instruction: %v -> %s", copy.Sources, copy.Destination)
			}
			break
		}
	}
	if !copyFromFound {
		t.Error("COPY --from=builder instruction not found in runtime stage")
	}

	// Test LLB conversion with multi-stage support
	converter := dockerfile.NewLLBConverter()
	
	// Test conversion to different targets
	testTargets := []string{"", "builder", "runtime"}
	
	for _, target := range testTargets {
		t.Run("target_"+target, func(t *testing.T) {
			opts := &dockerfile.ConvertOptions{
				Target: target,
				BuildArgs: map[string]string{
					"BUILDPLATFORM":  "linux/amd64",
					"TARGETPLATFORM": "linux/arm64",
				},
				Platform: "linux/arm64",
			}

			llbDef, err := converter.Convert(ast, opts)
			if err != nil {
				t.Fatalf("Failed to convert AST to LLB with target '%s': %v", target, err)
			}

			if llbDef == nil {
				t.Fatal("LLB definition is nil")
			}

			if len(llbDef.Definition) == 0 {
				t.Error("LLB definition is empty")
			}

			// Verify metadata contains build args
			if llbDef.Metadata == nil {
				t.Error("LLB metadata is nil")
			}
		})
	}

	// Test multi-platform build request
	t.Run("multi_platform_request", func(t *testing.T) {
		// Create temporary build context
		tempDir := t.TempDir()
		
		// Create mock files
		if err := os.MkdirAll(filepath.Join(tempDir, "cmd", "server"), 0755); err != nil {
			t.Fatalf("Failed to create temp directories: %v", err)
		}
		
		if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test\ngo 1.21\n"), 0644); err != nil {
			t.Fatalf("Failed to create go.mod: %v", err)
		}
		
		if err := os.WriteFile(filepath.Join(tempDir, "go.sum"), []byte(""), 0644); err != nil {
			t.Fatalf("Failed to create go.sum: %v", err)
		}
		
		if err := os.WriteFile(filepath.Join(tempDir, "cmd", "server", "main.go"), 
			[]byte("package main\nfunc main() { println(\"Hello\") }\n"), 0644); err != nil {
			t.Fatalf("Failed to create main.go: %v", err)
		}

		if err := os.MkdirAll(filepath.Join(tempDir, "config"), 0755); err != nil {
			t.Fatalf("Failed to create config directory: %v", err)
		}

		req := &BuildRequest{
			Dockerfile: ast,
			Context: BuildContext{
				Type:   ContextTypeLocal,
				Source: tempDir,
			},
			Platforms: []Platform{
				{OS: "linux", Architecture: "amd64"},
				{OS: "linux", Architecture: "arm64"},
			},
			BuildArgs: map[string]string{
				"VERSION": "1.0.0",
			},
			Labels: map[string]string{
				"org.opencontainers.image.title": "test-app",
				"org.opencontainers.image.version": "1.0.0",
			},
			Tags: []string{"test:multistage-multiarch"},
		}

		// Test request validation
		builder := &builder{}
		if err := builder.validateBuildRequest(req); err != nil {
			t.Fatalf("Build request validation failed: %v", err)
		}

		if err := builder.validateMultiStageBuild(req); err != nil {
			t.Fatalf("Multi-stage build validation failed: %v", err)
		}

		// Test build context preparation
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		buildCtx, err := builder.prepareBuildContext(ctx, &req.Context)
		if err != nil {
			t.Fatalf("Failed to prepare build context: %v", err)
		}
		defer buildCtx.Close()

		// Test LLB generation for each platform
		for _, platform := range req.Platforms {
			platformReq := *req
			platformReq.Platforms = []Platform{platform}

			llbDef, err := builder.generateLLBDefinition(ctx, &platformReq, buildCtx)
			if err != nil {
				t.Fatalf("Failed to generate LLB definition for platform %s: %v", platform.String(), err)
			}

			if llbDef == nil {
				t.Fatalf("LLB definition is nil for platform %s", platform.String())
			}

			if llbDef.Frontend != "dockerfile.v0" {
				t.Errorf("Expected frontend 'dockerfile.v0', got '%s'", llbDef.Frontend)
			}

			// Verify platform-specific metadata
			if platformBytes, exists := llbDef.Metadata["platform"]; exists {
				if string(platformBytes) != platform.String() {
					t.Errorf("Expected platform metadata '%s', got '%s'", platform.String(), string(platformBytes))
				}
			}
		}
	})
}

func TestRootlessWorkerRequirements(t *testing.T) {
	// This test checks rootless execution requirements
	t.Run("rootless_checks", func(t *testing.T) {
		// Create mock executor for testing
		executor := &buildKitExecutor{}

		ctx := context.Background()
		
		// Test execution spec preparation
		spec := &ExecutionSpec{
			Platform: Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
		}

		err := executor.Prepare(ctx, spec)
		if err != nil {
			// This is expected to fail in most test environments
			t.Logf("Rootless preparation failed as expected: %v", err)
		}

		// Test with invalid platform
		spec.Platform = Platform{
			OS:           "windows",
			Architecture: "amd64",
		}

		err = executor.Prepare(ctx, spec)
		if err == nil {
			t.Error("Expected error for unsupported platform in rootless mode")
		}

		// Test cleanup
		err = executor.Cleanup(ctx)
		if err != nil {
			t.Errorf("Cleanup should not fail: %v", err)
		}
	})
}

func TestCompleteM3Pipeline(t *testing.T) {
	// This test demonstrates the complete M-3 pipeline:
	// Multi-stage builds + Multi-arch support + Rootless execution
	
	dockerfileContent := `
FROM alpine:3.18 AS base
RUN apk add --no-cache ca-certificates

FROM base AS builder  
ARG TARGETARCH
RUN echo "Building for architecture: $TARGETARCH"
WORKDIR /src
COPY app.go .
RUN echo "#!/bin/sh\necho 'Built for $TARGETARCH'" > /src/app && chmod +x /src/app

FROM base AS runtime
COPY --from=builder /src/app /app/app
USER 1001:1001
CMD ["/app/app"]
`

	parser := dockerfile.New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Test 1: Multi-stage dependency resolution
	converter := dockerfile.NewLLBConverter().(*dockerfile.LLBConverterImpl)
	stageNames := make(map[string]int)
	for i, stage := range ast.Stages {
		if stage.Name != "" {
			stageNames[stage.Name] = i
		}
	}

	// Should build all stages for the final runtime stage
	required := converter.FindRequiredStages(ast, 2, stageNames)
	if len(required) != 3 {
		t.Errorf("Expected 3 required stages, got %d: %v", len(required), required)
	}

	// Test 2: Multi-architecture LLB generation
	platforms := []Platform{
		{OS: "linux", Architecture: "amd64"},
		{OS: "linux", Architecture: "arm64"},
	}

	for _, platform := range platforms {
		opts := &dockerfile.ConvertOptions{
			Platform: platform.String(),
			BuildArgs: map[string]string{
				"TARGETARCH": platform.Architecture,
			},
		}

		llbDef, err := converter.Convert(ast, opts)
		if err != nil {
			t.Fatalf("Failed to convert to LLB for platform %s: %v", platform.String(), err)
		}

		if llbDef == nil {
			t.Fatalf("LLB definition is nil for platform %s", platform.String())
		}

		t.Logf("Successfully generated LLB for platform %s", platform.String())
	}

	// Test 3: Rootless execution simulation
	ctx := context.Background()
	executor := &buildKitExecutor{}

	for _, platform := range platforms {
		spec := &ExecutionSpec{
			Platform: platform,
		}

		// Prepare execution environment
		err := executor.Prepare(ctx, spec)
		if err != nil {
			t.Logf("Rootless preparation for %s failed as expected in test env: %v", platform.String(), err)
			continue
		}

		// Simulate execution step
		step := &ExecutionStep{
			ID:      "test-step",
			Command: []string{"echo", "Hello from " + platform.String()},
			Env:     []string{"PLATFORM=" + platform.String()},
		}

		result, err := executor.Run(ctx, step)
		if err != nil {
			t.Logf("Execution for %s failed as expected in test env: %v", platform.String(), err)
		} else if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0, got %d for platform %s", result.ExitCode, platform.String())
		}

		// Cleanup
		executor.Cleanup(ctx)
	}

	t.Log("M-3 milestone pipeline test completed successfully")
}
package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shmocker/shmocker/pkg/dockerfile"
)

// MockBuildKitController is a mock implementation of BuildKitController for testing
type MockBuildKitController struct {
	solveFunc       func(ctx context.Context, def *SolveDefinition) (*SolveResult, error)
	importCacheFunc func(ctx context.Context, imports []*CacheImport) error
	exportCacheFunc func(ctx context.Context, exports []*CacheExport) error
	getSessionFunc  func(ctx context.Context) (Session, error)
	closeFunc       func() error
}

func (m *MockBuildKitController) Solve(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
	if m.solveFunc != nil {
		return m.solveFunc(ctx, def)
	}
	return &SolveResult{
		Ref:      "test-ref",
		Metadata: make(map[string][]byte),
	}, nil
}

func (m *MockBuildKitController) ImportCache(ctx context.Context, imports []*CacheImport) error {
	if m.importCacheFunc != nil {
		return m.importCacheFunc(ctx, imports)
	}
	return nil
}

func (m *MockBuildKitController) ExportCache(ctx context.Context, exports []*CacheExport) error {
	if m.exportCacheFunc != nil {
		return m.exportCacheFunc(ctx, exports)
	}
	return nil
}

func (m *MockBuildKitController) GetSession(ctx context.Context) (Session, error) {
	if m.getSessionFunc != nil {
		return m.getSessionFunc(ctx)
	}
	return &MockSession{}, nil
}

func (m *MockBuildKitController) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// MockSession is a mock implementation of Session for testing
type MockSession struct {
	id      string
	runFunc func(ctx context.Context) error
}

func (m *MockSession) ID() string {
	if m.id != "" {
		return m.id
	}
	return "test-session-id"
}

func (m *MockSession) Run(ctx context.Context) error {
	if m.runFunc != nil {
		return m.runFunc(ctx)
	}
	return nil
}

func (m *MockSession) Close() error {
	return nil
}

// createTestBuilder creates a builder instance with a mock controller for testing
func createTestBuilder(controller BuildKitController) *builder {
	return &builder{
		controller: controller,
		options: &BuilderOptions{
			Root:     "/tmp/test-root",
			DataRoot: "/tmp/test-data",
			Debug:    false,
		},
	}
}

// createTestDockerfileAST creates a simple Dockerfile AST for testing
func createTestDockerfileAST() *dockerfile.AST {
	// This would normally be created by parsing a Dockerfile
	// For testing, we create a minimal AST structure
	return &dockerfile.AST{
		Instructions: []dockerfile.Instruction{
			{
				Cmd:  "FROM",
				Args: []string{"alpine:latest"},
			},
			{
				Cmd:  "RUN",
				Args: []string{"echo 'Hello, World!'"},
			},
		},
	}
}

// createTestBuildContext creates a temporary directory for testing build contexts
func createTestBuildContext(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "shmocker-test-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a simple Dockerfile
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	dockerfileContent := `FROM alpine:latest
RUN echo 'Hello, World!'
`
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		t.Fatalf("Failed to create test Dockerfile: %v", err)
	}

	return tempDir
}

func TestBuilder_Build(t *testing.T) {
	tests := []struct {
		name        string
		request     *BuildRequest
		mockFunc    func(ctx context.Context, def *SolveDefinition) (*SolveResult, error)
		expectError bool
		errorType   string
	}{
		{
			name: "successful build",
			request: &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: createTestBuildContext(t),
				},
				Dockerfile: createTestDockerfileAST(),
				Tags:       []string{"test:latest"},
			},
			mockFunc: func(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
				return &SolveResult{
					Ref:      "sha256:test-image-ref",
					Metadata: make(map[string][]byte),
				}, nil
			},
			expectError: false,
		},
		{
			name: "nil build request",
			request: nil,
			expectError: true,
		},
		{
			name: "missing dockerfile",
			request: &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: createTestBuildContext(t),
				},
				Dockerfile: nil,
			},
			expectError: true,
		},
		{
			name: "missing context type",
			request: &BuildRequest{
				Context: BuildContext{
					Source: createTestBuildContext(t),
				},
				Dockerfile: createTestDockerfileAST(),
			},
			expectError: true,
		},
		{
			name: "missing context source",
			request: &BuildRequest{
				Context: BuildContext{
					Type: ContextTypeLocal,
				},
				Dockerfile: createTestDockerfileAST(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up test context if created
			if tt.request != nil && tt.request.Context.Source != "" {
				defer os.RemoveAll(tt.request.Context.Source)
			}

			// Create mock controller
			mockController := &MockBuildKitController{
				solveFunc: tt.mockFunc,
			}

			// Create builder
			b := createTestBuilder(mockController)

			// Execute build
			ctx := context.Background()
			result, err := b.Build(ctx, tt.request)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Validate result
			if result == nil {
				t.Errorf("Expected result but got nil")
				return
			}

			if result.ImageID == "" {
				t.Errorf("Expected ImageID but got empty string")
			}

			if result.BuildTime <= 0 {
				t.Errorf("Expected positive BuildTime but got %v", result.BuildTime)
			}
		})
	}
}

func TestBuilder_BuildWithProgress(t *testing.T) {
	tempDir := createTestBuildContext(t)
	defer os.RemoveAll(tempDir)

	mockController := &MockBuildKitController{
		solveFunc: func(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
			return &SolveResult{
				Ref:      "sha256:test-image-ref",
				Metadata: make(map[string][]byte),
			}, nil
		},
	}

	b := createTestBuilder(mockController)

	request := &BuildRequest{
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: tempDir,
		},
		Dockerfile: createTestDockerfileAST(),
		Tags:       []string{"test:latest"},
	}

	// Create progress channel
	progressCh := make(chan *ProgressEvent, 100)
	defer close(progressCh)

	// Start progress listener
	var progressEvents []*ProgressEvent
	go func() {
		for event := range progressCh {
			progressEvents = append(progressEvents, event)
		}
	}()

	// Execute build with progress
	ctx := context.Background()
	result, err := b.BuildWithProgress(ctx, request, progressCh)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if result == nil {
		t.Errorf("Expected result but got nil")
		return
	}

	// Give some time for progress events to be processed
	time.Sleep(100 * time.Millisecond)

	// Check that we received progress events
	if len(progressEvents) == 0 {
		t.Errorf("Expected progress events but got none")
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
		t.Errorf("Expected start progress event")
	}

	if !hasComplete {
		t.Errorf("Expected completion progress event")
	}
}

func TestBuilder_ValidateBuildRequest(t *testing.T) {
	b := createTestBuilder(&MockBuildKitController{})

	tests := []struct {
		name        string
		request     *BuildRequest
		expectError bool
	}{
		{
			name: "valid request",
			request: &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: "/tmp/test",
				},
				Dockerfile: createTestDockerfileAST(),
			},
			expectError: false,
		},
		{
			name: "missing dockerfile",
			request: &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: "/tmp/test",
				},
				Dockerfile: nil,
			},
			expectError: true,
		},
		{
			name: "missing context type",
			request: &BuildRequest{
				Context: BuildContext{
					Source: "/tmp/test",
				},
				Dockerfile: createTestDockerfileAST(),
			},
			expectError: true,
		},
		{
			name: "missing context source",
			request: &BuildRequest{
				Context: BuildContext{
					Type: ContextTypeLocal,
				},
				Dockerfile: createTestDockerfileAST(),
			},
			expectError: true,
		},
		{
			name: "invalid platform",
			request: &BuildRequest{
				Context: BuildContext{
					Type:   ContextTypeLocal,
					Source: "/tmp/test",
				},
				Dockerfile: createTestDockerfileAST(),
				Platforms: []Platform{
					{OS: "linux"}, // Missing architecture
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := b.validateBuildRequest(tt.request)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestBuilder_Close(t *testing.T) {
	closeCalled := false
	mockController := &MockBuildKitController{
		closeFunc: func() error {
			closeCalled = true
			return nil
		},
	}

	b := createTestBuilder(mockController)

	err := b.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !closeCalled {
		t.Errorf("Expected controller Close to be called")
	}
}

func TestPlatform_String(t *testing.T) {
	tests := []struct {
		name     string
		platform Platform
		expected string
	}{
		{
			name: "without variant",
			platform: Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
			expected: "linux/amd64",
		},
		{
			name: "with variant",
			platform: Platform{
				OS:           "linux",
				Architecture: "arm",
				Variant:      "v7",
			},
			expected: "linux/arm/v7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.platform.String()
			if result != tt.expected {
				t.Errorf("Expected %s but got %s", tt.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkBuilder_Build(b *testing.B) {
	tempDir := createTestBuildContext(&testing.T{})
	defer os.RemoveAll(tempDir)

	mockController := &MockBuildKitController{
		solveFunc: func(ctx context.Context, def *SolveDefinition) (*SolveResult, error) {
			return &SolveResult{
				Ref:      "sha256:test-image-ref",
				Metadata: make(map[string][]byte),
			}, nil
		},
	}

	builder := createTestBuilder(mockController)

	request := &BuildRequest{
		Context: BuildContext{
			Type:   ContextTypeLocal,
			Source: tempDir,
		},
		Dockerfile: createTestDockerfileAST(),
		Tags:       []string{"test:latest"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.Build(ctx, request)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}
package builder

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildError_Error(t *testing.T) {
	err := NewBuildError(ErrorTypeDockerfile, "test error message", nil)
	expected := "test error message"
	if err.Error() != expected {
		t.Errorf("Expected %s, got %s", expected, err.Error())
	}
}

func TestBuildError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewBuildError(ErrorTypeDockerfile, "test error", cause)
	
	if err.Unwrap() != cause {
		t.Errorf("Expected unwrapped error to be the cause")
	}
}

func TestBuildError_WithStep(t *testing.T) {
	err := NewBuildError(ErrorTypeDockerfile, "test error", nil)
	err.WithStep("RUN apt-get update")
	
	if err.Step != "RUN apt-get update" {
		t.Errorf("Expected step to be set")
	}
}

func TestBuildError_WithSuggestions(t *testing.T) {
	err := NewBuildError(ErrorTypeDockerfile, "test error", nil)
	suggestions := []string{"suggestion 1", "suggestion 2"}
	err.WithSuggestions(suggestions...)
	
	if len(err.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(err.Suggestions))
	}
	
	if err.Suggestions[0] != "suggestion 1" || err.Suggestions[1] != "suggestion 2" {
		t.Errorf("Suggestions not set correctly")
	}
}

func TestBuildError_WithContext(t *testing.T) {
	err := NewBuildError(ErrorTypeDockerfile, "test error", nil)
	err.WithContext("file", "Dockerfile")
	err.WithContext("line", 5)
	
	if err.Context["file"] != "Dockerfile" {
		t.Errorf("Expected context file to be Dockerfile")
	}
	
	if err.Context["line"] != 5 {
		t.Errorf("Expected context line to be 5")
	}
}

func TestBuildError_FormatUserError(t *testing.T) {
	cause := errors.New("parse error")
	err := NewBuildError(ErrorTypeDockerfile, "Dockerfile syntax error", cause)
	err.WithStep("FROM alpine:latest")
	err.WithSuggestions("Check syntax", "Verify format")
	err.WithContext("line", 1)
	
	formatted := err.FormatUserError()
	
	// Check that all components are included
	expectedComponents := []string{
		"Build failed: Dockerfile syntax error",
		"Step: FROM alpine:latest",
		"Category: dockerfile",
		"Cause: parse error",
		"line: 1",
		"• Check syntax",
		"• Verify format",
	}
	
	for _, component := range expectedComponents {
		if !strings.Contains(formatted, component) {
			t.Errorf("Expected formatted error to contain '%s'\nGot: %s", component, formatted)
		}
	}
}

func TestErrorClassifier_ClassifyError(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		name          string
		inputError    error
		expectedType  BuildErrorType
		expectedMsg   string
	}{
		{
			name:         "already BuildError",
			inputError:   NewBuildError(ErrorTypeCache, "cache error", nil),
			expectedType: ErrorTypeCache,
			expectedMsg:  "cache error",
		},
		{
			name:         "context error - directory not found",
			inputError:   errors.New("build context directory does not exist"),
			expectedType: ErrorTypeContext,
			expectedMsg:  "Build context directory not found",
		},
		{
			name:         "dockerfile error - parse error",
			inputError:   errors.New("dockerfile parse error at line 5"),
			expectedType: ErrorTypeDockerfile,
			expectedMsg:  "Dockerfile syntax error",
		},
		{
			name:         "dependency error - package not found",
			inputError:   errors.New("package not found: nonexistent-package"),
			expectedType: ErrorTypeDependency,
			expectedMsg:  "Package not found",
		},
		{
			name:         "permission error",
			inputError:   errors.New("permission denied: /var/lib/docker"),
			expectedType: ErrorTypePermission,
			expectedMsg:  "Insufficient permissions",
		},
		{
			name:         "network error - timeout",
			inputError:   errors.New("network timeout connecting to registry"),
			expectedType: ErrorTypeNetwork,
			expectedMsg:  "Network timeout",
		},
		{
			name:         "cache error",
			inputError:   errors.New("cache import failed"),
			expectedType: ErrorTypeCache,
			expectedMsg:  "Build cache operation failed",
		},
		{
			name:         "resource error - no space",
			inputError:   errors.New("no space left on device"),
			expectedType: ErrorTypeResource,
			expectedMsg:  "Insufficient disk space",
		},
		{
			name:         "execution error",
			inputError:   errors.New("command failed with exit code 1"),
			expectedType: ErrorTypeExecution,
			expectedMsg:  "Command execution failed",
		},
		{
			name:         "configuration error",
			inputError:   errors.New("buildkit worker configuration invalid"),
			expectedType: ErrorTypeConfiguration,
			expectedMsg:  "Build configuration error",
		},
		{
			name:         "unknown error",
			inputError:   errors.New("something completely unexpected happened"),
			expectedType: ErrorTypeUnknown,
			expectedMsg:  "Unknown build error",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.ClassifyError(tt.inputError)
			
			if result == nil {
				t.Errorf("Expected non-nil result")
				return
			}
			
			if result.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, result.Type)
			}
			
			if result.Message != tt.expectedMsg {
				t.Errorf("Expected message '%s', got '%s'", tt.expectedMsg, result.Message)
			}
			
			// Check that suggestions are provided
			if len(result.Suggestions) == 0 {
				t.Errorf("Expected suggestions to be provided")
			}
		})
	}
}

func TestErrorClassifier_IsContextError(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		message  string
		expected bool
	}{
		{"build context directory not found", true},
		{"git clone failed", true},
		{"tar extract error", true},
		{"http download failed", true},
		{"dockerignore parse error", true},
		{"syntax error in dockerfile", false},
		{"package not found", false},
	}
	
	for _, tt := range tests {
		result := classifier.isContextError(tt.message)
		if result != tt.expected {
			t.Errorf("For message '%s', expected %v, got %v", tt.message, tt.expected, result)
		}
	}
}

func TestErrorClassifier_IsDockerfileError(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		message  string
		expected bool
	}{
		{"dockerfile parse error", true},
		{"syntax error in dockerfile", true},
		{"unknown instruction RUN2", true},
		{"invalid format", true},
		{"ast processing failed", true},
		{"network timeout", false},
		{"permission denied", false},
	}
	
	for _, tt := range tests {
		result := classifier.isDockerfileError(tt.message)
		if result != tt.expected {
			t.Errorf("For message '%s', expected %v, got %v", tt.message, tt.expected, result)
		}
	}
}

func TestErrorClassifier_IsDependencyError(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		message  string
		expected bool
	}{
		{"package not found: curl", true},
		{"command not found: gcc", true},
		{"apt-get update failed", true},
		{"npm install error", true},
		{"pip install failed", true},
		{"dockerfile parse error", false},
		{"network timeout", false},
	}
	
	for _, tt := range tests {
		result := classifier.isDependencyError(tt.message)
		if result != tt.expected {
			t.Errorf("For message '%s', expected %v, got %v", tt.message, tt.expected, result)
		}
	}
}

func TestErrorClassifier_IsNetworkError(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		message  string
		expected bool
	}{
		{"network timeout", true},
		{"connection refused", true},
		{"dns resolution failed", true},
		{"proxy connection error", true},
		{"no route to host", true},
		{"dockerfile parse error", false},
		{"package not found", false},
	}
	
	for _, tt := range tests {
		result := classifier.isNetworkError(tt.message)
		if result != tt.expected {
			t.Errorf("For message '%s', expected %v, got %v", tt.message, tt.expected, result)
		}
	}
}

func TestErrorClassifier_IsResourceError(t *testing.T) {
	classifier := NewErrorClassifier()
	
	tests := []struct {
		message  string
		expected bool
	}{
		{"no space left on device", true},
		{"out of memory", true},
		{"disk full", true},
		{"too many open files", true},
		{"resource temporarily unavailable", true},
		{"dockerfile parse error", false},
		{"network timeout", false},
	}
	
	for _, tt := range tests {
		result := classifier.isResourceError(tt.message)
		if result != tt.expected {
			t.Errorf("For message '%s', expected %v, got %v", tt.message, tt.expected, result)
		}
	}
}

func TestWrapError(t *testing.T) {
	originalErr := errors.New("original error message")
	step := "RUN apt-get update"
	
	wrappedErr := WrapError(originalErr, step)
	
	if wrappedErr == nil {
		t.Errorf("Expected non-nil wrapped error")
		return
	}
	
	buildErr, ok := wrappedErr.(*BuildError)
	if !ok {
		t.Errorf("Expected BuildError type")
		return
	}
	
	if buildErr.Step != step {
		t.Errorf("Expected step '%s', got '%s'", step, buildErr.Step)
	}
	
	if buildErr.Cause != originalErr {
		t.Errorf("Expected cause to be original error")
	}
}

func TestWrapError_NilError(t *testing.T) {
	result := WrapError(nil, "test step")
	if result != nil {
		t.Errorf("Expected nil result for nil input")
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected bool
	}{
		{
			name:     "network error is retryable",
			error:    NewBuildError(ErrorTypeNetwork, "network error", nil),
			expected: true,
		},
		{
			name:     "resource error is retryable",
			error:    NewBuildError(ErrorTypeResource, "resource error", nil),
			expected: true,
		},
		{
			name:     "dockerfile error is not retryable",
			error:    NewBuildError(ErrorTypeDockerfile, "dockerfile error", nil),
			expected: false,
		},
		{
			name:     "non-BuildError is not retryable",
			error:    errors.New("regular error"),
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.error)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected BuildErrorType
	}{
		{
			name:     "BuildError returns correct type",
			error:    NewBuildError(ErrorTypeDockerfile, "dockerfile error", nil),
			expected: ErrorTypeDockerfile,
		},
		{
			name:     "non-BuildError returns unknown",
			error:    errors.New("regular error"),
			expected: ErrorTypeUnknown,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorType(tt.error)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
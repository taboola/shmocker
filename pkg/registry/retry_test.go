package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      fmt.Errorf("connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      fmt.Errorf("connection reset by peer"),
			expected: true,
		},
		{
			name:     "no such host",
			err:      fmt.Errorf("no such host"),
			expected: true,
		},
		{
			name:     "broken pipe",
			err:      fmt.Errorf("write: broken pipe"),
			expected: true,
		},
		{
			name: "temporary network error",
			err: &net.OpError{
				Op:  "dial",
				Err: &temporaryError{},
			},
			expected: true,
		},
		{
			name: "timeout error",
			err: &net.OpError{
				Op:  "dial",
				Err: &timeoutError{},
			},
			expected: true,
		},
		{
			name: "dns temporary error",
			err: &net.DNSError{
				IsTemporary: true,
			},
			expected: true,
		},
		{
			name:     "retryable error wrapper",
			err:      &RetryableError{Err: fmt.Errorf("test"), Retryable: true},
			expected: true,
		},
		{
			name:     "non-retryable error wrapper",
			err:      &RetryableError{Err: fmt.Errorf("test"), Retryable: false},
			expected: false,
		},
		{
			name:     "generic error",
			err:      fmt.Errorf("generic error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsHTTPRetryable(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   bool
	}{
		{
			name:       "too many requests",
			statusCode: http.StatusTooManyRequests,
			expected:   true,
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			expected:   true,
		},
		{
			name:       "bad gateway",
			statusCode: http.StatusBadGateway,
			expected:   true,
		},
		{
			name:       "service unavailable",
			statusCode: http.StatusServiceUnavailable,
			expected:   true,
		},
		{
			name:       "gateway timeout",
			statusCode: http.StatusGatewayTimeout,
			expected:   true,
		},
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			expected:   false,
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			expected:   false,
		},
		{
			name:       "forbidden",
			statusCode: http.StatusForbidden,
			expected:   false,
		},
		{
			name:       "ok",
			statusCode: http.StatusOK,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHTTPRetryable(tt.statusCode)
			if result != tt.expected {
				t.Errorf("IsHTTPRetryable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWrapHTTPError(t *testing.T) {
	originalErr := fmt.Errorf("original error")
	
	// Test with retryable status code
	wrapped := WrapHTTPError(originalErr, http.StatusInternalServerError)
	
	var retryableErr *RetryableError
	if !errors.As(wrapped, &retryableErr) {
		t.Fatal("WrapHTTPError() did not return RetryableError")
	}
	
	if !retryableErr.Retryable {
		t.Error("WrapHTTPError() should mark 500 as retryable")
	}
	
	if retryableErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("WrapHTTPError() status code = %d, want %d", retryableErr.StatusCode, http.StatusInternalServerError)
	}
	
	if retryableErr.Unwrap() != originalErr {
		t.Error("WrapHTTPError() did not preserve original error")
	}

	// Test with non-retryable status code
	wrapped = WrapHTTPError(originalErr, http.StatusNotFound)
	if !errors.As(wrapped, &retryableErr) {
		t.Fatal("WrapHTTPError() did not return RetryableError")
	}
	
	if retryableErr.Retryable {
		t.Error("WrapHTTPError() should not mark 404 as retryable")
	}

	// Test with nil error
	wrapped = WrapHTTPError(nil, http.StatusInternalServerError)
	if wrapped != nil {
		t.Error("WrapHTTPError() should return nil for nil error")
	}
}

func TestRetryWithBackoff(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        2,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            false, // Disable jitter for predictable testing
	}

	// Test successful operation (no retries needed)
	ctx := context.Background()
	callCount := 0
	
	operation := func() error {
		callCount++
		return nil
	}
	
	err := RetryWithBackoff(ctx, config, operation)
	if err != nil {
		t.Errorf("RetryWithBackoff() error = %v, want nil", err)
	}
	if callCount != 1 {
		t.Errorf("RetryWithBackoff() call count = %d, want 1", callCount)
	}

	// Test operation that succeeds after retries
	callCount = 0
	operation = func() error {
		callCount++
		if callCount < 3 {
			return &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
		}
		return nil
	}
	
	startTime := time.Now()
	err = RetryWithBackoff(ctx, config, operation)
	elapsed := time.Since(startTime)
	
	if err != nil {
		t.Errorf("RetryWithBackoff() error = %v, want nil", err)
	}
	if callCount != 3 {
		t.Errorf("RetryWithBackoff() call count = %d, want 3", callCount)
	}
	
	// Should have waited for at least initial delay + (initial delay * multiplier)
	expectedMinDelay := config.InitialDelay + time.Duration(float64(config.InitialDelay)*config.BackoffMultiplier)
	if elapsed < expectedMinDelay {
		t.Errorf("RetryWithBackoff() elapsed time %v less than expected minimum %v", elapsed, expectedMinDelay)
	}

	// Test non-retryable error
	callCount = 0
	operation = func() error {
		callCount++
		return fmt.Errorf("non-retryable error")
	}
	
	err = RetryWithBackoff(ctx, config, operation)
	if err == nil {
		t.Error("RetryWithBackoff() expected error for non-retryable operation")
	}
	if callCount != 1 {
		t.Errorf("RetryWithBackoff() call count = %d, want 1", callCount)
	}

	// Test max retries exceeded
	callCount = 0
	operation = func() error {
		callCount++
		return &RetryableError{Err: fmt.Errorf("persistent failure"), Retryable: true}
	}
	
	err = RetryWithBackoff(ctx, config, operation)
	if err == nil {
		t.Error("RetryWithBackoff() expected error when max retries exceeded")
	}
	expectedCalls := config.MaxRetries + 1 // Initial attempt + retries
	if callCount != expectedCalls {
		t.Errorf("RetryWithBackoff() call count = %d, want %d", callCount, expectedCalls)
	}

	// Test context cancellation
	cancelCtx, cancel := context.WithCancel(context.Background())
	callCount = 0
	
	operation = func() error {
		callCount++
		if callCount == 1 {
			// Cancel context after first call
			cancel()
			return &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
		}
		return nil
	}
	
	err = RetryWithBackoff(cancelCtx, config, operation)
	if err != context.Canceled {
		t.Errorf("RetryWithBackoff() error = %v, want %v", err, context.Canceled)
	}
}

func TestCalculateDelay(t *testing.T) {
	config := &RetryConfig{
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
	}

	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "first retry",
			attempt:  0,
			expected: 1 * time.Second,
		},
		{
			name:     "second retry",
			attempt:  1,
			expected: 2 * time.Second,
		},
		{
			name:     "third retry",
			attempt:  2,
			expected: 4 * time.Second,
		},
		{
			name:     "max delay exceeded",
			attempt:  10,
			expected: 30 * time.Second, // Should be capped at MaxDelay
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateDelay(tt.attempt, config)
			if result != tt.expected {
				t.Errorf("CalculateDelay() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRetryableHTTPClient(t *testing.T) {
	// Create test server that fails initially then succeeds
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}
	}))
	defer server.Close()

	config := &RetryConfig{
		MaxRetries:        3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            false,
	}

	client := NewRetryableHTTPClient(server.Client(), config)
	
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("RetryableHTTPClient.Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("RetryableHTTPClient.Do() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	if callCount != 3 {
		t.Errorf("RetryableHTTPClient.Do() call count = %d, want 3", callCount)
	}
}

func TestRetryableRegistryClient(t *testing.T) {
	// Create mock registry client
	mockClient := &mockRegistryClient{
		failCount: 2, // Fail first 2 attempts
	}

	config := &RetryConfig{
		MaxRetries:        3,
		InitialDelay:      1 * time.Millisecond,
		MaxDelay:          10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            false,
	}

	retryClient := NewRetryableRegistryClient(mockClient, config)

	ctx := context.Background()
	ref := "registry.example.com/test:latest"

	// Test GetManifest with retries
	manifest, err := retryClient.GetManifest(ctx, ref)
	if err != nil {
		t.Fatalf("RetryableRegistryClient.GetManifest() error = %v", err)
	}

	if manifest == nil {
		t.Fatal("RetryableRegistryClient.GetManifest() returned nil manifest")
	}

	if mockClient.callCount != 3 {
		t.Errorf("RetryableRegistryClient.GetManifest() call count = %d, want 3", mockClient.callCount)
	}
}

// Helper types for testing

type temporaryError struct{}

func (e *temporaryError) Error() string   { return "temporary error" }
func (e *temporaryError) Temporary() bool { return true }
func (e *temporaryError) Timeout() bool   { return false }

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout error" }
func (e *timeoutError) Temporary() bool { return false }
func (e *timeoutError) Timeout() bool   { return true }

// Mock registry client for testing
type mockRegistryClient struct {
	failCount int
	callCount int
}

func (m *mockRegistryClient) Push(ctx context.Context, req *PushRequest) (*PushResult, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return &PushResult{}, nil
}

func (m *mockRegistryClient) Pull(ctx context.Context, req *PullRequest) (*PullResult, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return &PullResult{}, nil
}

func (m *mockRegistryClient) GetManifest(ctx context.Context, ref string) (*Manifest, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return &Manifest{SchemaVersion: 2, MediaType: MediaTypes.OCIManifest}, nil
}

func (m *mockRegistryClient) PutManifest(ctx context.Context, ref string, manifest *Manifest) error {
	m.callCount++
	if m.callCount <= m.failCount {
		return &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return nil
}

func (m *mockRegistryClient) GetBlob(ctx context.Context, ref string, digest string) (io.ReadCloser, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	
	// Return appropriate content based on digest
	if strings.Contains(digest, "config") {
		config := map[string]interface{}{
			"version": "v1",
		}
		configData, _ := json.Marshal(config)
		return io.NopCloser(bytes.NewReader(configData)), nil
	}
	
	return io.NopCloser(strings.NewReader("blob content")), nil
}

func (m *mockRegistryClient) PutBlob(ctx context.Context, ref string, content io.Reader) (*BlobResult, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return &BlobResult{}, nil
}

func (m *mockRegistryClient) DeleteBlob(ctx context.Context, ref string, digest string) error {
	m.callCount++
	if m.callCount <= m.failCount {
		return &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return nil
}

func (m *mockRegistryClient) ListTags(ctx context.Context, repository string) ([]string, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return []string{"latest"}, nil
}

func (m *mockRegistryClient) GetImageConfig(ctx context.Context, ref string) (*ImageConfig, error) {
	m.callCount++
	if m.callCount <= m.failCount {
		return nil, &RetryableError{Err: fmt.Errorf("temporary failure"), Retryable: true}
	}
	return &ImageConfig{}, nil
}

func (m *mockRegistryClient) Close() error {
	return nil
}
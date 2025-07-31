// Package registry provides retry and error handling utilities for registry operations.
package registry

import (
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// RetryConfig contains configuration for retry operations.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int `json:"max_retries"`
	
	// InitialDelay is the initial delay between retries
	InitialDelay time.Duration `json:"initial_delay"`
	
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration `json:"max_delay"`
	
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64 `json:"backoff_multiplier"`
	
	// Jitter adds randomness to retry delays
	Jitter bool `json:"jitter"`
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            true,
	}
}

// RetryableError represents an error that can be retried.
type RetryableError struct {
	Err        error
	Retryable  bool
	StatusCode int
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Context timeout/cancellation are not retryable (check first)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check if it's already wrapped as RetryableError
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		return retryableErr.Retryable
	}

	// Network errors are typically retryable
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Temporary() || netErr.Timeout()
	}

	// DNS errors are retryable
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return dnsErr.Temporary()
	}

	// Connection errors
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "broken pipe") ||
		strings.Contains(err.Error(), "no such host") {
		return true
	}

	return false
}

// IsHTTPRetryable checks if an HTTP error is retryable based on status code.
func IsHTTPRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,      // 429
		http.StatusInternalServerError,   // 500
		http.StatusBadGateway,            // 502
		http.StatusServiceUnavailable,    // 503
		http.StatusGatewayTimeout:        // 504
		return true
	default:
		return false
	}
}

// WrapHTTPError wraps an HTTP error with retry information.
func WrapHTTPError(err error, statusCode int) error {
	if err == nil {
		return nil
	}

	return &RetryableError{
		Err:        err,
		Retryable:  IsHTTPRetryable(statusCode),
		StatusCode: statusCode,
	}
}

// RetryableOperation represents an operation that can be retried.
type RetryableOperation func() error

// RetryWithBackoff executes an operation with exponential backoff retry logic.
func RetryWithBackoff(ctx context.Context, config *RetryConfig, operation RetryableOperation) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute the operation
		err := operation()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if we should retry
		if !IsRetryable(err) {
			return err // Not retryable
		}

		// Don't sleep after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate next delay with exponential backoff
		actualDelay := delay
		if config.Jitter {
			// Add jitter (±25% of delay)
			jitter := time.Duration(float64(delay) * 0.25 * (2*float64(time.Now().UnixNano()%1000)/1000 - 1))
			actualDelay = delay + jitter
		}

		// Ensure delay doesn't exceed maximum
		if actualDelay > config.MaxDelay {
			actualDelay = config.MaxDelay
		}

		// Sleep with context cancellation support
		timer := time.NewTimer(actualDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		// Calculate next delay
		delay = time.Duration(float64(delay) * config.BackoffMultiplier)
	}

	return lastErr
}

// RetryableHTTPClient wraps an HTTP client with retry logic.
type RetryableHTTPClient struct {
	client *http.Client
	config *RetryConfig
}

// NewRetryableHTTPClient creates a new HTTP client with retry capabilities.
func NewRetryableHTTPClient(client *http.Client, config *RetryConfig) *RetryableHTTPClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &RetryableHTTPClient{
		client: client,
		config: config,
	}
}

// Do executes an HTTP request with retry logic.
func (c *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	retryOp := func() error {
		// Clone the request for retry attempts
		reqClone := req.Clone(req.Context())

		resp, err = c.client.Do(reqClone)
		if err != nil {
			return err
		}

		// Check if HTTP status code is retryable
		if IsHTTPRetryable(resp.StatusCode) {
			resp.Body.Close() // Close the body before retrying
			return WrapHTTPError(fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status), resp.StatusCode)
		}

		return nil
	}

	if err := RetryWithBackoff(req.Context(), c.config, retryOp); err != nil {
		return nil, err
	}

	return resp, nil
}

// RetryableRegistryClient wraps a registry client with retry logic.
type RetryableRegistryClient struct {
	client Client
	config *RetryConfig
}

// NewRetryableRegistryClient creates a registry client with retry capabilities.
func NewRetryableRegistryClient(client Client, config *RetryConfig) *RetryableRegistryClient {
	if config == nil {
		config = DefaultRetryConfig()
	}

	return &RetryableRegistryClient{
		client: client,
		config: config,
	}
}

// Push pushes an image with retry logic.
func (c *RetryableRegistryClient) Push(ctx context.Context, req *PushRequest) (*PushResult, error) {
	var result *PushResult
	var err error

	retryOp := func() error {
		result, err = c.client.Push(ctx, req)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return result, nil
}

// Pull pulls an image with retry logic.
func (c *RetryableRegistryClient) Pull(ctx context.Context, req *PullRequest) (*PullResult, error) {
	var result *PullResult
	var err error

	retryOp := func() error {
		result, err = c.client.Pull(ctx, req)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return result, nil
}

// GetManifest retrieves a manifest with retry logic.
func (c *RetryableRegistryClient) GetManifest(ctx context.Context, ref string) (*Manifest, error) {
	var manifest *Manifest
	var err error

	retryOp := func() error {
		manifest, err = c.client.GetManifest(ctx, ref)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return manifest, nil
}

// PutManifest uploads a manifest with retry logic.
func (c *RetryableRegistryClient) PutManifest(ctx context.Context, ref string, manifest *Manifest) error {
	retryOp := func() error {
		return c.client.PutManifest(ctx, ref, manifest)
	}

	return RetryWithBackoff(ctx, c.config, retryOp)
}

// GetBlob retrieves a blob with retry logic.
func (c *RetryableRegistryClient) GetBlob(ctx context.Context, ref string, digest string) (io.ReadCloser, error) {
	var blob io.ReadCloser
	var err error

	retryOp := func() error {
		blob, err = c.client.GetBlob(ctx, ref, digest)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return blob, nil
}

// PutBlob uploads a blob with retry logic.
func (c *RetryableRegistryClient) PutBlob(ctx context.Context, ref string, content io.Reader) (*BlobResult, error) {
	var result *BlobResult
	var err error

	retryOp := func() error {
		result, err = c.client.PutBlob(ctx, ref, content)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteBlob deletes a blob with retry logic.
func (c *RetryableRegistryClient) DeleteBlob(ctx context.Context, ref string, digest string) error {
	retryOp := func() error {
		return c.client.DeleteBlob(ctx, ref, digest)
	}

	return RetryWithBackoff(ctx, c.config, retryOp)
}

// ListTags lists tags with retry logic.
func (c *RetryableRegistryClient) ListTags(ctx context.Context, repository string) ([]string, error) {
	var tags []string
	var err error

	retryOp := func() error {
		tags, err = c.client.ListTags(ctx, repository)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return tags, nil
}

// GetImageConfig retrieves image config with retry logic.
func (c *RetryableRegistryClient) GetImageConfig(ctx context.Context, ref string) (*ImageConfig, error) {
	var config *ImageConfig
	var err error

	retryOp := func() error {
		config, err = c.client.GetImageConfig(ctx, ref)
		return err
	}

	if err := RetryWithBackoff(ctx, c.config, retryOp); err != nil {
		return nil, err
	}

	return config, nil
}

// Close closes the underlying client.
func (c *RetryableRegistryClient) Close() error {
	return c.client.Close()
}

// CalculateDelay calculates the delay for a given retry attempt.
func CalculateDelay(attempt int, config *RetryConfig) time.Duration {
	if config == nil {
		config = DefaultRetryConfig()
	}

	delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.BackoffMultiplier, float64(attempt)))
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}
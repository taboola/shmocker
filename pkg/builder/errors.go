package builder

import (
	"fmt"
	"strings"
)

// BuildError represents a build-specific error with user-friendly messaging
type BuildError struct {
	Type        BuildErrorType
	Message     string
	Cause       error
	Step        string
	Suggestions []string
	Context     map[string]interface{}
}

// BuildErrorType categorizes different types of build errors
type BuildErrorType string

const (
	ErrorTypeContext       BuildErrorType = "context"
	ErrorTypeDockerfile    BuildErrorType = "dockerfile"
	ErrorTypeDependency    BuildErrorType = "dependency"
	ErrorTypePermission    BuildErrorType = "permission"
	ErrorTypeNetwork       BuildErrorType = "network"
	ErrorTypeCache         BuildErrorType = "cache"
	ErrorTypeResource      BuildErrorType = "resource"
	ErrorTypeExecution     BuildErrorType = "execution"
	ErrorTypeConfiguration BuildErrorType = "configuration"
	ErrorTypeUnknown       BuildErrorType = "unknown"
)

// Error implements the error interface
func (be *BuildError) Error() string {
	return be.Message
}

// Unwrap returns the underlying error
func (be *BuildError) Unwrap() error {
	return be.Cause
}

// NewBuildError creates a new build error
func NewBuildError(errorType BuildErrorType, message string, cause error) *BuildError {
	return &BuildError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// WithStep adds step information to the error
func (be *BuildError) WithStep(step string) *BuildError {
	be.Step = step
	return be
}

// WithSuggestions adds suggestions to resolve the error
func (be *BuildError) WithSuggestions(suggestions ...string) *BuildError {
	be.Suggestions = append(be.Suggestions, suggestions...)
	return be
}

// WithContext adds context information to the error
func (be *BuildError) WithContext(key string, value interface{}) *BuildError {
	be.Context[key] = value
	return be
}

// FormatUserError formats the error for user display
func (be *BuildError) FormatUserError() string {
	var sb strings.Builder

	// Error header
	sb.WriteString(fmt.Sprintf("Build failed: %s\n", be.Message))

	// Step information
	if be.Step != "" {
		sb.WriteString(fmt.Sprintf("Step: %s\n", be.Step))
	}

	// Error type
	sb.WriteString(fmt.Sprintf("Category: %s\n", be.Type))

	// Underlying cause
	if be.Cause != nil {
		sb.WriteString(fmt.Sprintf("Cause: %s\n", be.Cause.Error()))
	}

	// Context information
	if len(be.Context) > 0 {
		sb.WriteString("Context:\n")
		for k, v := range be.Context {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", k, v))
		}
	}

	// Suggestions
	if len(be.Suggestions) > 0 {
		sb.WriteString("Suggestions:\n")
		for _, suggestion := range be.Suggestions {
			sb.WriteString(fmt.Sprintf("  • %s\n", suggestion))
		}
	}

	return sb.String()
}

// ErrorClassifier analyzes errors and converts them to user-friendly BuildErrors
type ErrorClassifier struct{}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier() *ErrorClassifier {
	return &ErrorClassifier{}
}

// ClassifyError analyzes an error and returns a user-friendly BuildError
func (ec *ErrorClassifier) ClassifyError(err error) *BuildError {
	if err == nil {
		return nil
	}

	// Check if it's already a BuildError
	if buildErr, ok := err.(*BuildError); ok {
		return buildErr
	}

	// Analyze error message and classify
	errMsg := strings.ToLower(err.Error())

	switch {
	case ec.isContextError(errMsg):
		return ec.classifyContextError(err, errMsg)
	case ec.isDockerfileError(errMsg):
		return ec.classifyDockerfileError(err, errMsg)
	case ec.isDependencyError(errMsg):
		return ec.classifyDependencyError(err, errMsg)
	case ec.isPermissionError(errMsg):
		return ec.classifyPermissionError(err, errMsg)
	case ec.isNetworkError(errMsg):
		return ec.classifyNetworkError(err, errMsg)
	case ec.isCacheError(errMsg):
		return ec.classifyCacheError(err, errMsg)
	case ec.isResourceError(errMsg):
		return ec.classifyResourceError(err, errMsg)
	case ec.isExecutionError(errMsg):
		return ec.classifyExecutionError(err, errMsg)
	case ec.isConfigurationError(errMsg):
		return ec.classifyConfigurationError(err, errMsg)
	default:
		return ec.classifyUnknownError(err, errMsg)
	}
}

// isContextError checks if the error is context-related
func (ec *ErrorClassifier) isContextError(errMsg string) bool {
	contextKeywords := []string{
		"build context",
		"context directory",
		"dockerignore",
		"git clone",
		"tar extract",
		"download",
		"http",
		"context not found",
	}

	return ec.containsAny(errMsg, contextKeywords)
}

// classifyContextError classifies context-related errors
func (ec *ErrorClassifier) classifyContextError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeContext, "Build context error", err)

	switch {
	case strings.Contains(errMsg, "directory does not exist"):
		buildErr.Message = "Build context directory not found"
		buildErr.WithSuggestions(
			"Verify the build context path is correct",
			"Ensure the directory exists and is accessible",
		)
	case strings.Contains(errMsg, "git clone"):
		buildErr.Message = "Failed to clone Git repository"
		buildErr.WithSuggestions(
			"Check if the Git URL is accessible",
			"Verify authentication credentials if needed",
			"Ensure Git is installed and available",
		)
	case strings.Contains(errMsg, "tar"):
		buildErr.Message = "Failed to extract tar archive"
		buildErr.WithSuggestions(
			"Verify the tar file is not corrupted",
			"Check if the tar format is supported",
		)
	case strings.Contains(errMsg, "http"):
		buildErr.Message = "Failed to download from HTTP URL"
		buildErr.WithSuggestions(
			"Check if the URL is accessible",
			"Verify network connectivity",
			"Check if authentication is required",
		)
	default:
		buildErr.Message = "Build context preparation failed"
		buildErr.WithSuggestions(
			"Check the build context path or URL",
			"Verify file permissions",
		)
	}

	return buildErr
}

// isDockerfileError checks if the error is Dockerfile-related
func (ec *ErrorClassifier) isDockerfileError(errMsg string) bool {
	dockerfileKeywords := []string{
		"dockerfile",
		"parse error",
		"syntax error",
		"unknown instruction",
		"invalid format",
		"ast",
	}

	return ec.containsAny(errMsg, dockerfileKeywords)
}

// classifyDockerfileError classifies Dockerfile-related errors
func (ec *ErrorClassifier) classifyDockerfileError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeDockerfile, "Dockerfile error", err)

	switch {
	case strings.Contains(errMsg, "parse") || strings.Contains(errMsg, "syntax"):
		buildErr.Message = "Dockerfile syntax error"
		buildErr.WithSuggestions(
			"Check Dockerfile syntax for errors",
			"Verify instruction format and arguments",
			"Use 'docker build --dry-run' to validate syntax",
		)
	case strings.Contains(errMsg, "unknown instruction"):
		buildErr.Message = "Unknown Dockerfile instruction"
		buildErr.WithSuggestions(
			"Check for typos in instruction names",
			"Verify the instruction is supported",
			"See Dockerfile reference documentation",
		)
	case strings.Contains(errMsg, "ast"):
		buildErr.Message = "Failed to process Dockerfile AST"
		buildErr.WithSuggestions(
			"Check if the Dockerfile is properly formatted",
			"Try rebuilding with a simpler Dockerfile",
		)
	default:
		buildErr.Message = "Dockerfile processing failed"
		buildErr.WithSuggestions(
			"Review Dockerfile for syntax errors",
			"Check instruction format and arguments",
		)
	}

	return buildErr
}

// isDependencyError checks if the error is dependency-related
func (ec *ErrorClassifier) isDependencyError(errMsg string) bool {
	dependencyKeywords := []string{
		"package not found",
		"no such file",
		"command not found",
		"executable not found",
		"apt-get",
		"yum",
		"apk",
		"npm install",
		"pip install",
		"gem install",
	}

	return ec.containsAny(errMsg, dependencyKeywords)
}

// classifyDependencyError classifies dependency-related errors
func (ec *ErrorClassifier) classifyDependencyError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeDependency, "Dependency error", err)

	switch {
	case strings.Contains(errMsg, "package not found"):
		buildErr.Message = "Package not found"
		buildErr.WithSuggestions(
			"Check if the package name is correct",
			"Update package repository cache",
			"Verify the package is available in the repository",
		)
	case strings.Contains(errMsg, "command not found"):
		buildErr.Message = "Command not found"
		buildErr.WithSuggestions(
			"Install the required package containing the command",
			"Check if the command is in PATH",
			"Verify the command name spelling",
		)
	case strings.Contains(errMsg, "apt-get") || strings.Contains(errMsg, "yum") || strings.Contains(errMsg, "apk"):
		buildErr.Message = "Package manager error"
		buildErr.WithSuggestions(
			"Run package manager update first",
			"Check network connectivity",
			"Verify repository configuration",
		)
	default:
		buildErr.Message = "Dependency installation failed"
		buildErr.WithSuggestions(
			"Check package names and versions",
			"Verify repository availability",
		)
	}

	return buildErr
}

// isPermissionError checks if the error is permission-related
func (ec *ErrorClassifier) isPermissionError(errMsg string) bool {
	permissionKeywords := []string{
		"permission denied",
		"access denied",
		"operation not permitted",
		"insufficient privileges",
		"rootless",
	}

	return ec.containsAny(errMsg, permissionKeywords)
}

// classifyPermissionError classifies permission-related errors
func (ec *ErrorClassifier) classifyPermissionError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypePermission, "Permission error", err)

	buildErr.Message = "Insufficient permissions"
	buildErr.WithSuggestions(
		"Check file and directory permissions",
		"Ensure the user has necessary access rights",
		"Consider running in rootless mode if appropriate",
		"Verify ownership of build context files",
	)

	return buildErr
}

// isNetworkError checks if the error is network-related
func (ec *ErrorClassifier) isNetworkError(errMsg string) bool {
	networkKeywords := []string{
		"network",
		"connection",
		"timeout",
		"resolve",
		"dns",
		"no route",
		"unreachable",
		"proxy",
	}

	return ec.containsAny(errMsg, networkKeywords)
}

// classifyNetworkError classifies network-related errors
func (ec *ErrorClassifier) classifyNetworkError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeNetwork, "Network error", err)

	switch {
	case strings.Contains(errMsg, "timeout"):
		buildErr.Message = "Network timeout"
		buildErr.WithSuggestions(
			"Check network connectivity",
			"Increase timeout values if needed",
			"Verify no firewall is blocking connections",
		)
	case strings.Contains(errMsg, "resolve") || strings.Contains(errMsg, "dns"):
		buildErr.Message = "DNS resolution failed"
		buildErr.WithSuggestions(
			"Check DNS configuration",
			"Verify domain name is correct",
			"Try using IP address instead of hostname",
		)
	case strings.Contains(errMsg, "proxy"):
		buildErr.Message = "Proxy connection error"
		buildErr.WithSuggestions(
			"Check proxy configuration",
			"Verify proxy credentials",
			"Test proxy connectivity",
		)
	default:
		buildErr.Message = "Network connection failed"
		buildErr.WithSuggestions(
			"Check network connectivity",
			"Verify URLs are accessible",
			"Check firewall and proxy settings",
		)
	}

	return buildErr
}

// isCacheError checks if the error is cache-related
func (ec *ErrorClassifier) isCacheError(errMsg string) bool {
	cacheKeywords := []string{
		"cache",
		"import cache",
		"export cache",
		"cache mount",
	}

	return ec.containsAny(errMsg, cacheKeywords)
}

// classifyCacheError classifies cache-related errors
func (ec *ErrorClassifier) classifyCacheError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeCache, "Cache error", err)

	buildErr.Message = "Build cache operation failed"
	buildErr.WithSuggestions(
		"Check cache configuration",
		"Verify cache storage is accessible",
		"Try building without cache if possible",
		"Clear cache and retry",
	)

	return buildErr
}

// isResourceError checks if the error is resource-related
func (ec *ErrorClassifier) isResourceError(errMsg string) bool {
	resourceKeywords := []string{
		"no space left",
		"out of memory",
		"resource temporarily unavailable",
		"too many open files",
		"disk full",
	}

	return ec.containsAny(errMsg, resourceKeywords)
}

// classifyResourceError classifies resource-related errors
func (ec *ErrorClassifier) classifyResourceError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeResource, "Resource error", err)

	switch {
	case strings.Contains(errMsg, "no space left") || strings.Contains(errMsg, "disk full"):
		buildErr.Message = "Insufficient disk space"
		buildErr.WithSuggestions(
			"Free up disk space",
			"Clean up old Docker images and containers",
			"Use a different build location with more space",
		)
	case strings.Contains(errMsg, "out of memory"):
		buildErr.Message = "Insufficient memory"
		buildErr.WithSuggestions(
			"Increase available memory",
			"Reduce build parallelism",
			"Use multi-stage builds to reduce memory usage",
		)
	case strings.Contains(errMsg, "too many open files"):
		buildErr.Message = "File descriptor limit exceeded"
		buildErr.WithSuggestions(
			"Increase file descriptor limits",
			"Close unnecessary files in build process",
		)
	default:
		buildErr.Message = "System resource exhausted"
		buildErr.WithSuggestions(
			"Check system resources (disk, memory, CPU)",
			"Reduce build complexity",
			"Try building at a different time",
		)
	}

	return buildErr
}

// isExecutionError checks if the error is execution-related
func (ec *ErrorClassifier) isExecutionError(errMsg string) bool {
	executionKeywords := []string{
		"exit code",
		"command failed",
		"execution failed",
		"process exited",
	}

	return ec.containsAny(errMsg, executionKeywords)
}

// classifyExecutionError classifies execution-related errors
func (ec *ErrorClassifier) classifyExecutionError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeExecution, "Execution error", err)

	buildErr.Message = "Command execution failed"
	buildErr.WithSuggestions(
		"Check the command syntax and arguments",
		"Verify required files and dependencies exist",
		"Review command output for specific errors",
		"Test the command manually in the container",
	)

	return buildErr
}

// isConfigurationError checks if the error is configuration-related
func (ec *ErrorClassifier) isConfigurationError(errMsg string) bool {
	configKeywords := []string{
		"configuration",
		"invalid option",
		"unsupported",
		"not implemented",
		"buildkit",
		"worker",
	}

	return ec.containsAny(errMsg, configKeywords)
}

// classifyConfigurationError classifies configuration-related errors
func (ec *ErrorClassifier) classifyConfigurationError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeConfiguration, "Configuration error", err)

	buildErr.Message = "Build configuration error"
	buildErr.WithSuggestions(
		"Check build configuration parameters",
		"Verify BuildKit worker setup",
		"Review build options and flags",
		"Check for unsupported features",
	)

	return buildErr
}

// classifyUnknownError classifies unknown errors
func (ec *ErrorClassifier) classifyUnknownError(err error, errMsg string) *BuildError {
	buildErr := NewBuildError(ErrorTypeUnknown, "Unknown build error", err)

	buildErr.WithSuggestions(
		"Review the error message for clues",
		"Check BuildKit and system logs",
		"Try a simpler build to isolate the issue",
		"Report this issue if it persists",
	)

	return buildErr
}

// containsAny checks if the message contains any of the keywords
func (ec *ErrorClassifier) containsAny(message string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}
	return false
}

// WrapError wraps an error with build context and user-friendly messaging
func WrapError(err error, step string) error {
	if err == nil {
		return nil
	}

	classifier := NewErrorClassifier()
	buildErr := classifier.ClassifyError(err)
	buildErr.WithStep(step)

	return buildErr
}

// IsRetryableError checks if an error is retryable
func IsRetryableError(err error) bool {
	if buildErr, ok := err.(*BuildError); ok {
		switch buildErr.Type {
		case ErrorTypeNetwork, ErrorTypeResource:
			return true
		default:
			return false
		}
	}
	return false
}

// GetErrorType returns the error type if it's a BuildError
func GetErrorType(err error) BuildErrorType {
	if buildErr, ok := err.(*BuildError); ok {
		return buildErr.Type
	}
	return ErrorTypeUnknown
}

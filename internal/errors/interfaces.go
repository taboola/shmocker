// Package errors defines interfaces and types for comprehensive error handling.
package errors

import (
	"context"
	"time"
)

// ErrorManager provides centralized error management and reporting.
type ErrorManager interface {
	// HandleError processes and potentially recovers from errors
	HandleError(ctx context.Context, err error, context *ErrorContext) (*ErrorResult, error)
	
	// RecordError records an error for analysis and reporting
	RecordError(ctx context.Context, err error, context *ErrorContext) error
	
	// GetErrorHistory returns error history for analysis
	GetErrorHistory(ctx context.Context, filter *ErrorFilter) ([]*ErrorRecord, error)
	
	// GetErrorStats returns error statistics
	GetErrorStats(ctx context.Context, timeRange TimeRange) (*ErrorStats, error)
	
	// RegisterErrorHandler registers a custom error handler
	RegisterErrorHandler(errorType ErrorType, handler ErrorHandler) error
	
	// SetErrorPolicy sets error handling policies
	SetErrorPolicy(policies []*ErrorPolicy) error
}

// ErrorHandler handles specific types of errors.
type ErrorHandler interface {
	// CanHandle returns true if this handler can handle the error
	CanHandle(err error) bool
	
	// Handle processes the error and returns a result
	Handle(ctx context.Context, err error, context *ErrorContext) (*ErrorResult, error)
	
	// GetErrorType returns the error type this handler processes
	GetErrorType() ErrorType
	
	// GetPriority returns the handler priority (higher = more important)
	GetPriority() int
}

// ErrorReporter provides error reporting and alerting capabilities.
type ErrorReporter interface {
	// ReportError sends an error report
	ReportError(ctx context.Context, err error, context *ErrorContext) error
	
	// ReportBatch sends multiple error reports
	ReportBatch(ctx context.Context, errors []*ErrorRecord) error
	
	// SetDestination configures reporting destination
	SetDestination(destination *ReportingDestination) error
	
	// GetSupportedDestinations returns supported reporting destinations
	GetSupportedDestinations() []DestinationType
}

// ErrorClassifier classifies errors into categories for appropriate handling.
type ErrorClassifier interface {
	// Classify classifies an error into a category
	Classify(err error) *ErrorClassification
	
	// GetErrorType determines the error type
	GetErrorType(err error) ErrorType
	
	// GetSeverity determines the error severity
	GetSeverity(err error) ErrorSeverity
	
	// IsRetryable determines if an error is retryable
	IsRetryable(err error) bool
	
	// GetCategory determines the error category
	GetCategory(err error) ErrorCategory
}

// ErrorRecovery provides error recovery mechanisms.
type ErrorRecovery interface {
	// AttemptRecovery attempts to recover from an error
	AttemptRecovery(ctx context.Context, err error, context *ErrorContext) (*RecoveryResult, error)
	
	// CanRecover returns true if recovery is possible for the error
	CanRecover(err error) bool
	
	// GetRecoveryStrategies returns available recovery strategies
	GetRecoveryStrategies(err error) []RecoveryStrategy
	
	// RegisterRecoveryAction registers a custom recovery action
	RegisterRecoveryAction(errorType ErrorType, action RecoveryAction) error
}

// ErrorContext provides context information about where an error occurred.
type ErrorContext struct {
	// Component is the component where the error occurred
	Component string `json:"component"`
	
	// Operation is the operation being performed
	Operation string `json:"operation"`
	
	// Stage is the workflow stage
	Stage string `json:"stage,omitempty"`
	
	// WorkflowID is the workflow identifier
	WorkflowID string `json:"workflow_id,omitempty"`
	
	// BuildID is the build identifier
	BuildID string `json:"build_id,omitempty"`
	
	// ImageRef is the image reference
	ImageRef string `json:"image_ref,omitempty"`
	
	// UserID is the user identifier
	UserID string `json:"user_id,omitempty"`
	
	// RequestID is the request identifier
	RequestID string `json:"request_id,omitempty"`
	
	// Timestamp is when the error occurred
	Timestamp time.Time `json:"timestamp"`
	
	// Metadata contains additional context data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	
	// StackTrace contains the stack trace
	StackTrace []string `json:"stack_trace,omitempty"`
	
	// Environment contains environment information
	Environment *EnvironmentInfo `json:"environment,omitempty"`
}

// ErrorResult contains the result of error handling.
type ErrorResult struct {
	// Action is the action to take
	Action ErrorAction `json:"action"`
	
	// Recovered indicates if the error was recovered
	Recovered bool `json:"recovered"`
	
	// RetryAfter indicates when to retry (if applicable)
	RetryAfter time.Duration `json:"retry_after,omitempty"`
	
	// Message is a human-readable message
	Message string `json:"message,omitempty"`
	
	// Suggestions contains suggestions for resolving the error
	Suggestions []string `json:"suggestions,omitempty"`
	
	// RecoveryActions contains actions taken to recover
	RecoveryActions []string `json:"recovery_actions,omitempty"`
	
	// AdditionalData contains additional result data
	AdditionalData map[string]interface{} `json:"additional_data,omitempty"`
}

// ErrorRecord represents a recorded error for analysis.
type ErrorRecord struct {
	// ID is the unique error identifier
	ID string `json:"id"`
	
	// Error is the error message
	Error string `json:"error"`
	
	// ErrorType is the classified error type
	ErrorType ErrorType `json:"error_type"`
	
	// Severity is the error severity
	Severity ErrorSeverity `json:"severity"`
	
	// Category is the error category
	Category ErrorCategory `json:"category"`
	
	// Context is the error context
	Context *ErrorContext `json:"context"`
	
	// Classification is the error classification
	Classification *ErrorClassification `json:"classification"`
	
	// Resolution is how the error was resolved
	Resolution *ErrorResolution `json:"resolution,omitempty"`
	
	// Timestamp is when the error was recorded
	Timestamp time.Time `json:"timestamp"`
	
	// Count is how many times this error occurred
	Count int `json:"count"`
	
	// FirstSeen is when this error was first seen
	FirstSeen time.Time `json:"first_seen"`
	
	// LastSeen is when this error was last seen
	LastSeen time.Time `json:"last_seen"`
}

// ErrorType represents the type of error.
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"     // Input validation errors
	ErrorTypeNetwork       ErrorType = "network"        // Network connectivity errors
	ErrorTypeRegistry      ErrorType = "registry"       // Registry operation errors
	ErrorTypeFileSystem    ErrorType = "filesystem"     // File system errors
	ErrorTypePermission    ErrorType = "permission"     // Permission errors
	ErrorTypeResource      ErrorType = "resource"       // Resource exhaustion errors
	ErrorTypeTimeout       ErrorType = "timeout"        // Timeout errors
	ErrorTypeDependency    ErrorType = "dependency"     // Dependency errors
	ErrorTypeConfiguration ErrorType = "configuration"  // Configuration errors
	ErrorTypeAuthentication ErrorType = "authentication" // Authentication errors
	ErrorTypeAuthorization ErrorType = "authorization"  // Authorization errors
	ErrorTypeBuild         ErrorType = "build"          // Build execution errors
	ErrorTypeParsing       ErrorType = "parsing"        // Parsing errors
	ErrorTypeInternal      ErrorType = "internal"       // Internal system errors
	ErrorTypeExternal      ErrorType = "external"       // External service errors
	ErrorTypeUnknown       ErrorType = "unknown"        // Unknown errors
)

// ErrorSeverity represents the severity of an error.
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"      // Low severity (warnings)
	SeverityMedium   ErrorSeverity = "medium"   // Medium severity (recoverable errors)
	SeverityHigh     ErrorSeverity = "high"     // High severity (significant errors)
	SeverityCritical ErrorSeverity = "critical" // Critical severity (system failures)
)

// ErrorCategory represents the category of an error.
type ErrorCategory string

const (
	CategoryUser    ErrorCategory = "user"    // User-caused errors
	CategorySystem  ErrorCategory = "system"  // System errors
	CategoryExternal ErrorCategory = "external" // External service errors
	CategoryConfig  ErrorCategory = "config"  // Configuration errors
)

// ErrorAction represents the action to take after an error.
type ErrorAction string

const (
	ActionFail     ErrorAction = "fail"     // Fail the operation
	ActionRetry    ErrorAction = "retry"    // Retry the operation
	ActionContinue ErrorAction = "continue" // Continue with warnings
	ActionSkip     ErrorAction = "skip"     // Skip the operation
	ActionRecover  ErrorAction = "recover"  // Attempt recovery
)

// ErrorClassification contains error classification information.
type ErrorClassification struct {
	// Type is the error type
	Type ErrorType `json:"type"`
	
	// Severity is the error severity
	Severity ErrorSeverity `json:"severity"`
	
	// Category is the error category
	Category ErrorCategory `json:"category"`
	
	// Retryable indicates if the error is retryable
	Retryable bool `json:"retryable"`
	
	// Recoverable indicates if the error is recoverable
	Recoverable bool `json:"recoverable"`
	
	// UserFacing indicates if the error should be shown to users
	UserFacing bool `json:"user_facing"`
	
	// Confidence is the classification confidence (0-1)
	Confidence float64 `json:"confidence"`
	
	// Tags contains classification tags
	Tags []string `json:"tags,omitempty"`
}

// ErrorResolution contains information about how an error was resolved.
type ErrorResolution struct {
	// Action is the action taken
	Action ErrorAction `json:"action"`
	
	// Success indicates if the resolution was successful
	Success bool `json:"success"`
	
	// Message is a description of the resolution
	Message string `json:"message,omitempty"`
	
	// Duration is how long resolution took
	Duration time.Duration `json:"duration"`
	
	// Attempts is the number of attempts made
	Attempts int `json:"attempts"`
	
	// ResolvedAt is when the error was resolved
	ResolvedAt time.Time `json:"resolved_at"`
	
	// ResolvedBy is who/what resolved the error
	ResolvedBy string `json:"resolved_by,omitempty"`
}

// ErrorPolicy defines how to handle specific types of errors.
type ErrorPolicy struct {
	// Name is the policy name
	Name string `json:"name"`
	
	// ErrorTypes are the error types this policy applies to
	ErrorTypes []ErrorType `json:"error_types"`
	
	// Conditions are additional conditions for policy application
	Conditions []string `json:"conditions,omitempty"`
	
	// Action is the default action to take
	Action ErrorAction `json:"action"`
	
	// MaxRetries is the maximum number of retries
	MaxRetries int `json:"max_retries,omitempty"`
	
	// RetryDelay is the delay between retries
	RetryDelay time.Duration `json:"retry_delay,omitempty"`
	
	// BackoffMultiplier is the retry backoff multiplier
	BackoffMultiplier float64 `json:"backoff_multiplier,omitempty"`
	
	// Timeout is the operation timeout
	Timeout time.Duration `json:"timeout,omitempty"`
	
	// Notification controls error notifications
	Notification *NotificationPolicy `json:"notification,omitempty"`
	
	// Recovery controls error recovery
	Recovery *RecoveryPolicy `json:"recovery,omitempty"`
}

// NotificationPolicy defines error notification behavior.
type NotificationPolicy struct {
	// Enabled indicates if notifications are enabled
	Enabled bool `json:"enabled"`
	
	// Severity is the minimum severity for notifications
	Severity ErrorSeverity `json:"severity,omitempty"`
	
	// Channels are the notification channels to use
	Channels []NotificationChannel `json:"channels,omitempty"`
	
	// Throttle defines notification throttling
	Throttle *NotificationThrottle `json:"throttle,omitempty"`
}

// RecoveryPolicy defines error recovery behavior.
type RecoveryPolicy struct {
	// Enabled indicates if recovery is enabled
	Enabled bool `json:"enabled"`
	
	// Strategies are the recovery strategies to try
	Strategies []RecoveryStrategy `json:"strategies,omitempty"`
	
	// MaxAttempts is the maximum recovery attempts
	MaxAttempts int `json:"max_attempts,omitempty"`
	
	// Timeout is the recovery timeout
	Timeout time.Duration `json:"timeout,omitempty"`
}

// RecoveryStrategy represents a recovery strategy.
type RecoveryStrategy string

const (
	RecoveryRetry        RecoveryStrategy = "retry"         // Retry the operation
	RecoveryFallback     RecoveryStrategy = "fallback"      // Use fallback mechanism
	RecoverySkip         RecoveryStrategy = "skip"          // Skip the operation
	RecoveryClearCache   RecoveryStrategy = "clear_cache"   // Clear cache and retry
	RecoveryRestart      RecoveryStrategy = "restart"       // Restart component
	RecoveryRollback     RecoveryStrategy = "rollback"      // Rollback changes
	RecoveryAlternative  RecoveryStrategy = "alternative"   // Use alternative approach
)

// RecoveryResult contains the result of a recovery attempt.
type RecoveryResult struct {
	// Success indicates if recovery was successful
	Success bool `json:"success"`
	
	// Strategy is the strategy that was used
	Strategy RecoveryStrategy `json:"strategy"`
	
	// Message is a description of the recovery
	Message string `json:"message,omitempty"`
	
	// Duration is how long recovery took
	Duration time.Duration `json:"duration"`
	
	// Actions contains the actions taken during recovery
	Actions []string `json:"actions,omitempty"`
	
	// NewError is any new error that occurred during recovery
	NewError error `json:"-"`
}

// RecoveryAction represents a recovery action.
type RecoveryAction interface {
	// Execute executes the recovery action
	Execute(ctx context.Context, err error, context *ErrorContext) (*RecoveryResult, error)
	
	// CanExecute returns true if the action can be executed
	CanExecute(err error, context *ErrorContext) bool
	
	// GetStrategy returns the recovery strategy
	GetStrategy() RecoveryStrategy
	
	// GetDescription returns a description of the action
	GetDescription() string
}

// ErrorFilter defines criteria for filtering errors.
type ErrorFilter struct {
	// ErrorTypes filters by error types
	ErrorTypes []ErrorType `json:"error_types,omitempty"`
	
	// Severities filters by error severities
	Severities []ErrorSeverity `json:"severities,omitempty"`
	
	// Categories filters by error categories
	Categories []ErrorCategory `json:"categories,omitempty"`
	
	// Components filters by components
	Components []string `json:"components,omitempty"`
	
	// Operations filters by operations
	Operations []string `json:"operations,omitempty"`
	
	// TimeRange filters by time range
	TimeRange *TimeRange `json:"time_range,omitempty"`
	
	// Limit limits the number of results
	Limit int `json:"limit,omitempty"`
	
	// Offset is the result offset
	Offset int `json:"offset,omitempty"`
}

// TimeRange represents a time range for filtering.
type TimeRange struct {
	// Start is the start time
	Start time.Time `json:"start"`
	
	// End is the end time
	End time.Time `json:"end"`
}

// ErrorStats contains error statistics.
type ErrorStats struct {
	// TotalErrors is the total number of errors
	TotalErrors int64 `json:"total_errors"`
	
	// ErrorsByType breaks down errors by type
	ErrorsByType map[ErrorType]int64 `json:"errors_by_type"`
	
	// ErrorsBySeverity breaks down errors by severity
	ErrorsBySeverity map[ErrorSeverity]int64 `json:"errors_by_severity"`
	
	// ErrorsByCategory breaks down errors by category
	ErrorsByCategory map[ErrorCategory]int64 `json:"errors_by_category"`
	
	// ErrorsByComponent breaks down errors by component
	ErrorsByComponent map[string]int64 `json:"errors_by_component"`
	
	// ErrorRate is the error rate (errors per time unit)
	ErrorRate float64 `json:"error_rate"`
	
	// RecoveryRate is the recovery success rate
	RecoveryRate float64 `json:"recovery_rate"`
	
	// AverageResolutionTime is the average time to resolve errors
	AverageResolutionTime time.Duration `json:"average_resolution_time"`
	
	// TimeRange is the time range for these statistics
	TimeRange *TimeRange `json:"time_range"`
}

// ReportingDestination defines where error reports should be sent.
type ReportingDestination struct {
	// Type is the destination type
	Type DestinationType `json:"type"`
	
	// Config contains destination-specific configuration
	Config map[string]interface{} `json:"config"`
	
	// Enabled indicates if this destination is enabled
	Enabled bool `json:"enabled"`
	
	// MinSeverity is the minimum severity for this destination
	MinSeverity ErrorSeverity `json:"min_severity,omitempty"`
	
	// Filters contains additional filters
	Filters *ErrorFilter `json:"filters,omitempty"`
}

// DestinationType represents the type of reporting destination.
type DestinationType string

const (
	DestinationLog      DestinationType = "log"      // Log to file/stdout
	DestinationEmail    DestinationType = "email"    // Send email notifications
	DestinationSlack    DestinationType = "slack"    // Send Slack notifications
	DestinationWebhook  DestinationType = "webhook"  // Send webhook notifications
	DestinationMetrics  DestinationType = "metrics"  // Send to metrics system
	DestinationSentry   DestinationType = "sentry"   // Send to Sentry
	DestinationDatadog  DestinationType = "datadog"  // Send to Datadog
)

// NotificationChannel represents a notification channel.
type NotificationChannel string

const (
	ChannelEmail   NotificationChannel = "email"
	ChannelSlack   NotificationChannel = "slack"
	ChannelWebhook NotificationChannel = "webhook"
	ChannelSMS     NotificationChannel = "sms"
)

// NotificationThrottle defines notification throttling behavior.
type NotificationThrottle struct {
	// MaxPerHour is the maximum notifications per hour
	MaxPerHour int `json:"max_per_hour,omitempty"`
	
	// MinInterval is the minimum interval between notifications
	MinInterval time.Duration `json:"min_interval,omitempty"`
	
	// BurstLimit is the burst notification limit
	BurstLimit int `json:"burst_limit,omitempty"`
}

// EnvironmentInfo contains information about the execution environment.
type EnvironmentInfo struct {
	// Version is the application version
	Version string `json:"version,omitempty"`
	
	// Commit is the git commit hash
	Commit string `json:"commit,omitempty"`
	
	// BuildTime is when the application was built
	BuildTime string `json:"build_time,omitempty"`
	
	// Runtime is the runtime environment
	Runtime string `json:"runtime,omitempty"`
	
	// OS is the operating system
	OS string `json:"os,omitempty"`
	
	// Architecture is the system architecture
	Architecture string `json:"architecture,omitempty"`
	
	// Hostname is the system hostname
	Hostname string `json:"hostname,omitempty"`
	
	// ContainerID is the container identifier (if running in container)
	ContainerID string `json:"container_id,omitempty"`
	
	// KubernetesInfo contains Kubernetes-specific information
	KubernetesInfo *KubernetesInfo `json:"kubernetes_info,omitempty"`
}

// KubernetesInfo contains Kubernetes-specific environment information.
type KubernetesInfo struct {
	// Namespace is the Kubernetes namespace
	Namespace string `json:"namespace,omitempty"`
	
	// PodName is the pod name
	PodName string `json:"pod_name,omitempty"`
	
	// NodeName is the node name
	NodeName string `json:"node_name,omitempty"`
	
	// ClusterName is the cluster name
	ClusterName string `json:"cluster_name,omitempty"`
}
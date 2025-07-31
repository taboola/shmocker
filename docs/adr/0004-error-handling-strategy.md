# ADR-0004: Error Handling Strategy

## Status
Accepted

## Context

Container image building involves multiple complex systems (filesystem, network, registry, cryptography) with various failure modes. The error handling strategy must:

1. **Provide clear feedback** to users about what went wrong and how to fix it
2. **Enable robust recovery** from transient failures
3. **Maintain system stability** despite component failures
4. **Support debugging** with detailed error context
5. **Classify errors** appropriately for different handling strategies

Error handling challenges include:

### Complexity of Error Sources
- Network failures (registry unavailable, DNS issues)
- Filesystem errors (permission denied, disk full)
- Authentication/authorization failures
- Resource exhaustion (memory, disk space, file handles)
- Parsing errors (invalid Dockerfile syntax)
- Build execution failures (command failures, missing dependencies)
- Cache corruption or inconsistency

### User Experience Requirements
- Clear, actionable error messages
- Suggestions for resolution
- Progress preservation (don't restart entire build on transient failures)
- Consistent error formatting across components

### System Reliability Requirements
- Graceful degradation when components fail
- Retry logic for transient failures
- Resource cleanup on errors
- Proper error propagation through the workflow

## Decision

We will implement a **comprehensive multi-layered error handling system** with:

1. **Error Classification**: Automatic categorization of errors by type, severity, and recoverability
2. **Recovery Strategies**: Automated recovery attempts for transient failures
3. **Context Preservation**: Rich error context for debugging and user feedback
4. **Workflow Integration**: Error handling integrated into the workflow orchestration
5. **User-Friendly Reporting**: Clear, actionable error messages with suggestions

## Architecture Design

### Error Manager Interface

```go
type ErrorManager interface {
    // Core error handling
    HandleError(ctx context.Context, err error, context *ErrorContext) (*ErrorResult, error)
    RecordError(ctx context.Context, err error, context *ErrorContext) error
    
    // Analysis and reporting
    GetErrorHistory(ctx context.Context, filter *ErrorFilter) ([]*ErrorRecord, error)
    GetErrorStats(ctx context.Context, timeRange TimeRange) (*ErrorStats, error)
    
    // Configuration
    RegisterErrorHandler(errorType ErrorType, handler ErrorHandler) error
    SetErrorPolicy(policies []*ErrorPolicy) error
}
```

### Error Classification System

```go
type ErrorClassifier interface {
    Classify(err error) *ErrorClassification
    GetErrorType(err error) ErrorType
    GetSeverity(err error) ErrorSeverity
    IsRetryable(err error) bool
    GetCategory(err error) ErrorCategory
}

// Comprehensive error classification
type ErrorClassification struct {
    Type        ErrorType     `json:"type"`
    Severity    ErrorSeverity `json:"severity"`
    Category    ErrorCategory `json:"category"`
    Retryable   bool         `json:"retryable"`
    Recoverable bool         `json:"recoverable"`
    UserFacing  bool         `json:"user_facing"`
    Confidence  float64      `json:"confidence"`
    Tags        []string     `json:"tags,omitempty"`
}

// Error types covering all major failure modes
const (
    ErrorTypeValidation     ErrorType = "validation"      // Input validation errors
    ErrorTypeNetwork        ErrorType = "network"         // Network connectivity errors
    ErrorTypeRegistry       ErrorType = "registry"        // Registry operation errors
    ErrorTypeFileSystem     ErrorType = "filesystem"      // File system errors
    ErrorTypePermission     ErrorType = "permission"      // Permission errors
    ErrorTypeResource       ErrorType = "resource"        // Resource exhaustion errors
    ErrorTypeTimeout        ErrorType = "timeout"         // Timeout errors
    ErrorTypeDependency     ErrorType = "dependency"      // Dependency errors
    ErrorTypeConfiguration ErrorType = "configuration"   // Configuration errors
    ErrorTypeAuthentication ErrorType = "authentication" // Authentication errors
    ErrorTypeAuthorization  ErrorType = "authorization"  // Authorization errors
    ErrorTypeBuild          ErrorType = "build"          // Build execution errors
    ErrorTypeParsing        ErrorType = "parsing"        // Parsing errors
    ErrorTypeInternal       ErrorType = "internal"       // Internal system errors
    ErrorTypeExternal       ErrorType = "external"       // External service errors
)
```

### Error Context and Rich Information

```go
type ErrorContext struct {
    // Location information
    Component   string    `json:"component"`
    Operation   string    `json:"operation"`
    Stage       string    `json:"stage,omitempty"`
    WorkflowID  string    `json:"workflow_id,omitempty"`
    BuildID     string    `json:"build_id,omitempty"`
    
    // Context data
    ImageRef    string    `json:"image_ref,omitempty"`
    UserID      string    `json:"user_id,omitempty"`
    RequestID   string    `json:"request_id,omitempty"`
    Timestamp   time.Time `json:"timestamp"`
    
    // Technical details
    Metadata    map[string]interface{} `json:"metadata,omitempty"`
    StackTrace  []string              `json:"stack_trace,omitempty"`
    Environment *EnvironmentInfo      `json:"environment,omitempty"`
}

// Enhanced error result with recovery information
type ErrorResult struct {
    Action           ErrorAction              `json:"action"`
    Recovered        bool                     `json:"recovered"`
    RetryAfter       time.Duration           `json:"retry_after,omitempty"`
    Message          string                  `json:"message,omitempty"`
    Suggestions      []string                `json:"suggestions,omitempty"`
    RecoveryActions  []string                `json:"recovery_actions,omitempty"`
    AdditionalData   map[string]interface{}  `json:"additional_data,omitempty"`
}
```

## Error Recovery Strategies

### Automatic Recovery Framework

```go
type ErrorRecovery interface {
    AttemptRecovery(ctx context.Context, err error, context *ErrorContext) (*RecoveryResult, error)
    CanRecover(err error) bool
    GetRecoveryStrategies(err error) []RecoveryStrategy
    RegisterRecoveryAction(errorType ErrorType, action RecoveryAction) error
}

// Recovery strategies for different error types
const (
    RecoveryRetry        RecoveryStrategy = "retry"         // Simple retry
    RecoveryFallback     RecoveryStrategy = "fallback"      // Use alternative method
    RecoverySkip         RecoveryStrategy = "skip"          // Skip non-critical operation
    RecoveryClearCache   RecoveryStrategy = "clear_cache"   // Clear cache and retry
    RecoveryRestart      RecoveryStrategy = "restart"       // Restart component
    RecoveryRollback     RecoveryStrategy = "rollback"      // Rollback changes
    RecoveryAlternative  RecoveryStrategy = "alternative"   // Use alternative approach
)
```

### Network Error Recovery

```go
type NetworkErrorHandler struct {
    maxRetries    int
    backoffConfig *BackoffConfig
    dnsResolver   *DNSResolver
}

func (h *NetworkErrorHandler) Handle(ctx context.Context, err error, context *ErrorContext) (*ErrorResult, error) {
    if !h.CanHandle(err) {
        return nil, ErrCannotHandle
    }
    
    netErr := err.(*NetworkError)
    
    switch netErr.Type {
    case NetworkErrorTypeDNS:
        return h.handleDNSError(ctx, netErr, context)
    case NetworkErrorTypeConnection:
        return h.handleConnectionError(ctx, netErr, context)  
    case NetworkErrorTypeTimeout:
        return h.handleTimeoutError(ctx, netErr, context)
    default:
        return h.handleGenericNetworkError(ctx, netErr, context)
    }
}

func (h *NetworkErrorHandler) handleDNSError(ctx context.Context, err *NetworkError, context *ErrorContext) (*ErrorResult, error) {
    // Try alternative DNS servers
    if alternativeServers := h.dnsResolver.GetAlternatives(); len(alternativeServers) > 0 {
        for _, server := range alternativeServers {
            if h.dnsResolver.TestServer(server) {
                h.dnsResolver.SetPrimary(server)
                return &ErrorResult{
                    Action:     ActionRetry,
                    Recovered:  true,
                    RetryAfter: 1 * time.Second,
                    Message:    "Switched to alternative DNS server",
                    RecoveryActions: []string{"dns_server_switch"},
                }, nil
            }
        }
    }
    
    // Suggest manual resolution
    return &ErrorResult{
        Action:  ActionFail,
        Message: fmt.Sprintf("DNS resolution failed for %s", err.Host),
        Suggestions: []string{
            "Check your internet connectivity",
            "Verify the hostname is correct", 
            "Try using a different DNS server",
            "Check if a VPN is interfering with DNS resolution",
        },
    }, nil
}
```

### Registry Error Recovery

```go
type RegistryErrorHandler struct {
    authProvider   AuthProvider
    registryConfig *RegistryConfig
}

func (h *RegistryErrorHandler) handleAuthError(ctx context.Context, err *RegistryError, context *ErrorContext) (*ErrorResult, error) {
    // Try to refresh authentication token
    if creds, refreshErr := h.authProvider.RefreshToken(ctx, err.Registry); refreshErr == nil {
        return &ErrorResult{
            Action:     ActionRetry,
            Recovered:  true,
            RetryAfter: 500 * time.Millisecond,
            Message:    "Refreshed authentication token",
            RecoveryActions: []string{"auth_token_refresh"},
        }, nil
    }
    
    // Clear cached credentials and retry
    h.authProvider.ClearCredentials(err.Registry)
    
    return &ErrorResult{
        Action:  ActionRetry,
        Message: "Authentication failed, cleared cached credentials",
        Suggestions: []string{
            "Verify your registry credentials are correct",
            "Check if your authentication token has expired",
            "Ensure you have permission to access this registry",
        },
        RecoveryActions: []string{"auth_cache_clear"},
    }, nil
}

func (h *RegistryErrorHandler) handleRateLimitError(ctx context.Context, err *RegistryError, context *ErrorContext) (*ErrorResult, error) {
    // Parse rate limit headers to determine retry delay
    retryAfter := h.parseRetryAfter(err.Headers)
    if retryAfter == 0 {
        retryAfter = h.calculateBackoffDelay(context.Metadata["attempt_count"].(int))
    }
    
    return &ErrorResult{
        Action:     ActionRetry,
        RetryAfter: retryAfter,
        Message:    fmt.Sprintf("Registry rate limit exceeded, retrying in %v", retryAfter),
        Suggestions: []string{
            "Consider using a different registry",
            "Implement request throttling in your build pipeline",
            "Check if you have exceeded your registry plan limits",
        },
    }, nil
}
```

### Build Error Recovery

```go
type BuildErrorHandler struct {
    cacheManager cache.Manager
    buildConfig  *BuildConfig
}

func (h *BuildErrorHandler) handleCacheCorruption(ctx context.Context, err *BuildError, context *ErrorContext) (*ErrorResult, error) {
    // Clear corrupted cache entries
    if cacheKey := context.Metadata["cache_key"]; cacheKey != nil {
        if clearErr := h.cacheManager.Delete(ctx, cacheKey.(string)); clearErr != nil {
            log.Warnf("Failed to clear corrupted cache entry: %v", clearErr)
        }
    }
    
    return &ErrorResult{
        Action:     ActionRetry,
        Recovered:  true,
        Message:    "Cleared corrupted cache entry and retrying build",
        RecoveryActions: []string{"cache_clear"},
    }, nil
}

func (h *BuildErrorHandler) handleResourceExhaustion(ctx context.Context, err *BuildError, context *ErrorContext) (*ErrorResult, error) {
    resourceType := err.ResourceType
    
    switch resourceType {
    case "memory":
        return &ErrorResult{
            Action:  ActionFail,
            Message: "Insufficient memory for build operation",
            Suggestions: []string{
                "Increase available memory for the build process",
                "Use multi-stage builds to reduce memory usage",
                "Consider using a machine with more RAM",
                "Split large operations into smaller steps",
            },
        }, nil
    case "disk":
        // Try to free up disk space
        if pruned := h.cacheManager.Prune(ctx, &cache.PruneOptions{Strategy: cache.PruneStrategyLRU}); pruned.RemovedSize > 0 {
            return &ErrorResult{
                Action:     ActionRetry,
                Recovered:  true,
                Message:    fmt.Sprintf("Freed %d bytes of cache space", pruned.RemovedSize),
                RecoveryActions: []string{"cache_prune"},
            }, nil
        }
        
        return &ErrorResult{
            Action:  ActionFail,
            Message: "Insufficient disk space for build operation",
            Suggestions: []string{
                "Free up disk space on the build machine",
                "Use a build cache with size limits",
                "Consider using remote cache storage",
                "Clean up old build artifacts",
            },
        }, nil
    default:
        return nil, ErrCannotHandle
    }
}
```

## Error Policy Configuration

### Policy-Based Error Handling

```go
type ErrorPolicy struct {
    Name         string        `json:"name"`
    ErrorTypes   []ErrorType   `json:"error_types"`
    Conditions   []string      `json:"conditions,omitempty"`
    Action       ErrorAction   `json:"action"`
    MaxRetries   int           `json:"max_retries,omitempty"`
    RetryDelay   time.Duration `json:"retry_delay,omitempty"`
    BackoffMultiplier float64  `json:"backoff_multiplier,omitempty"`
    Timeout      time.Duration `json:"timeout,omitempty"`
    Notification *NotificationPolicy `json:"notification,omitempty"`
    Recovery     *RecoveryPolicy     `json:"recovery,omitempty"`
}

// Example policies for different environments
var DefaultPolicies = []*ErrorPolicy{
    {
        Name:       "network-retry",
        ErrorTypes: []ErrorType{ErrorTypeNetwork, ErrorTypeTimeout},
        Action:     ActionRetry,
        MaxRetries: 3,
        RetryDelay: 2 * time.Second,
        BackoffMultiplier: 2.0,
        Recovery: &RecoveryPolicy{
            Enabled:    true,
            Strategies: []RecoveryStrategy{RecoveryRetry, RecoveryFallback},
        },
    },
    {
        Name:       "auth-refresh",
        ErrorTypes: []ErrorType{ErrorTypeAuthentication},
        Action:     ActionRecover,
        MaxRetries: 2,
        Recovery: &RecoveryPolicy{
            Enabled:    true,
            Strategies: []RecoveryStrategy{RecoveryRetry},
            MaxAttempts: 1,
        },
    },
    {
        Name:       "validation-fail-fast",
        ErrorTypes: []ErrorType{ErrorTypeValidation, ErrorTypeParsing},
        Action:     ActionFail,
        MaxRetries: 0,
        Notification: &NotificationPolicy{
            Enabled:  true,
            Severity: SeverityMedium,
        },
    },
}
```

## User-Friendly Error Messages

### Error Message Templates

```go
type ErrorMessageGenerator interface {
    GenerateUserMessage(err error, classification *ErrorClassification, context *ErrorContext) string
    GenerateSuggestions(err error, classification *ErrorClassification, context *ErrorContext) []string
    FormatError(err error, context *ErrorContext) *FormattedError
}

type FormattedError struct {
    Title       string   `json:"title"`
    Description string   `json:"description"`
    Suggestions []string `json:"suggestions"`
    TechnicalDetails map[string]interface{} `json:"technical_details,omitempty"`
    HelpURL     string   `json:"help_url,omitempty"`
}

// Message templates for common errors
var ErrorMessageTemplates = map[ErrorType]string{
    ErrorTypeNetwork: `
Build failed due to network connectivity issues.

Problem: Unable to connect to {{.Host}}
Stage: {{.Stage}}
{{if .Suggestions}}
Suggestions:
{{range .Suggestions}}• {{.}}
{{end}}
{{end}}

For more help, see: https://docs.shmocker.dev/troubleshooting/network-errors
`,
    
    ErrorTypeRegistry: `
Failed to access container registry.

Registry: {{.Registry}}
Operation: {{.Operation}}
Status: {{.StatusCode}} {{.StatusText}}

{{if .Suggestions}}
Common solutions:
{{range .Suggestions}}• {{.}}
{{end}}
{{end}}

Registry documentation: https://docs.shmocker.dev/registries/
`,
    
    ErrorTypeBuild: `
Build step failed during execution.

Command: {{.Command}}
Exit Code: {{.ExitCode}}
Stage: {{.Stage}}

{{if .Stderr}}
Error output:
{{.Stderr}}
{{end}}

{{if .Suggestions}}
Try these solutions:
{{range .Suggestions}}• {{.}}
{{end}}
{{end}}
`,
}
```

### Contextual Help System

```go
type HelpSystem interface {
    GetHelpURL(errorType ErrorType, errorCode string) string
    GetDocumentationLinks(err error) []DocumentationLink
    SearchKnowledgeBase(query string) []KnowledgeBaseEntry
}

type DocumentationLink struct {
    Title string `json:"title"`
    URL   string `json:"url"`
    Type  string `json:"type"` // "guide", "reference", "troubleshooting"
}

type KnowledgeBaseEntry struct {
    Title       string `json:"title"`
    Summary     string `json:"summary"`
    Solution    string `json:"solution"`
    URL         string `json:"url"`
    Relevance   float64 `json:"relevance"`
}
```

## Integration with Workflow System

### Workflow Error Handling

```go
type WorkflowErrorHandler struct {
    errorManager ErrorManager
    policies     []*ErrorPolicy
}

func (h *WorkflowErrorHandler) HandleStageError(ctx context.Context, stage workflow.Stage, err error, input *workflow.StageInput) (*workflow.ErrorResult, error) {
    // Create rich error context
    errorContext := &ErrorContext{
        Component:  "workflow",
        Operation:  "stage_execution", 
        Stage:      string(stage),
        WorkflowID: input.WorkflowID,
        BuildID:    input.BuildRequest.ID,
        Timestamp:  time.Now(),
        Metadata: map[string]interface{}{
            "stage_input": input,
            "attempt":     input.Context["attempt_count"],
        },
    }
    
    // Handle the error through the error manager
    result, handleErr := h.errorManager.HandleError(ctx, err, errorContext)
    if handleErr != nil {
        return nil, handleErr
    }
    
    // Convert to workflow error result
    return &workflow.ErrorResult{
        Action:     workflow.ErrorAction(result.Action),
        RetryDelay: result.RetryAfter,
        Message:    result.Message,
        Context:    result.AdditionalData,
    }, nil
}
```

## Monitoring and Analytics

### Error Metrics Collection

```go
type ErrorMetrics interface {
    RecordError(errorType ErrorType, severity ErrorSeverity, component string)
    RecordRecovery(errorType ErrorType, strategy RecoveryStrategy, success bool)
    RecordRetry(errorType ErrorType, attempt int, success bool)
    
    // Analytics
    GetErrorRate(timeWindow time.Duration) float64
    GetRecoveryRate(errorType ErrorType) float64
    GetMostCommonErrors(limit int) []ErrorFrequency
}

type ErrorFrequency struct {
    ErrorType ErrorType `json:"error_type"`
    Count     int64     `json:"count"`
    Rate      float64   `json:"rate"`
    LastSeen  time.Time `json:"last_seen"`
}
```

## Consequences

### Positive

1. **Improved Reliability**: Automatic recovery from transient failures
2. **Better User Experience**: Clear, actionable error messages
3. **Faster Debugging**: Rich error context and history
4. **Reduced Support Burden**: Self-service error resolution
5. **System Stability**: Graceful handling of component failures

### Negative

1. **Implementation Complexity**: Comprehensive error handling is complex
2. **Performance Overhead**: Error classification and context collection
3. **Configuration Complexity**: Many policies and handlers to configure
4. **Testing Challenges**: Need to test all error scenarios

### Mitigation Strategies

1. **Gradual Implementation**: Start with core error types and expand
2. **Performance Optimization**: Lazy evaluation of error context
3. **Default Policies**: Sensible defaults requiring minimal configuration
4. **Automated Testing**: Comprehensive error injection testing

## Implementation Roadmap

### Phase 1: Core Error Framework
- Basic error classification system
- Error context collection
- Simple retry logic

### Phase 2: Recovery Strategies
- Automatic recovery mechanisms
- Policy-based error handling
- Network and registry error recovery

### Phase 3: User Experience
- User-friendly error messages
- Contextual help system
- Error reporting and analytics

### Phase 4: Advanced Features
- Machine learning for error classification
- Predictive error prevention
- Integration with external monitoring systems

## References

- [Go Error Handling Best Practices](https://blog.golang.org/error-handling-and-go)
- [Site Reliability Engineering - Error Budgets](https://sre.google/sre-book/embracing-risk/)
- [The Twelve-Factor App - Logs](https://12factor.net/logs)

## Related ADRs

- [ADR-0001: Embed BuildKit as Library](./0001-embed-buildkit-as-library.md)
- [ADR-0002: Rootless Execution Strategy](./0002-rootless-execution-strategy.md)
- [ADR-0003: Cache Architecture](./0003-cache-architecture.md)
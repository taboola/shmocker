// Package workflow defines interfaces for orchestrating the build workflow.
package workflow

import (
	"context"
	"time"

	"github.com/shmocker/shmocker/pkg/builder"
	"github.com/shmocker/shmocker/pkg/cache"
	"github.com/shmocker/shmocker/pkg/dockerfile"
	"github.com/shmocker/shmocker/pkg/registry"
	"github.com/shmocker/shmocker/pkg/sbom"
	"github.com/shmocker/shmocker/pkg/signing"
)

// Orchestrator provides the main interface for orchestrating the complete build workflow.
type Orchestrator interface {
	// Execute runs the complete build workflow
	Execute(ctx context.Context, req *WorkflowRequest) (*WorkflowResult, error)
	
	// ExecuteStage runs a specific workflow stage
	ExecuteStage(ctx context.Context, stage Stage, input *StageInput) (*StageOutput, error)
	
	// GetStages returns all stages in the workflow
	GetStages() []Stage
	
	// ValidateWorkflow validates a workflow configuration
	ValidateWorkflow(workflow *WorkflowConfig) error
	
	// GetProgress returns current workflow progress
	GetProgress(ctx context.Context, workflowID string) (*WorkflowProgress, error)
	
	// Cancel cancels a running workflow
	Cancel(ctx context.Context, workflowID string) error
}

// StageExecutor executes individual workflow stages.
type StageExecutor interface {
	// Execute executes a stage with the given input
	Execute(ctx context.Context, input *StageInput) (*StageOutput, error)
	
	// GetStage returns the stage this executor handles
	GetStage() Stage
	
	// Validate validates stage input
	Validate(input *StageInput) error
	
	// GetDependencies returns stage dependencies
	GetDependencies() []Stage
}

// ErrorHandler handles errors during workflow execution.
type ErrorHandler interface {
	// HandleError processes an error during workflow execution
	HandleError(ctx context.Context, err error, stage Stage, input *StageInput) (*ErrorResult, error)
	
	// ShouldRetry determines if a stage should be retried
	ShouldRetry(err error, stage Stage, attempt int) bool
	
	// GetRetryDelay returns the delay before retry
	GetRetryDelay(attempt int) time.Duration
}

// ProgressReporter reports workflow progress.
type ProgressReporter interface {
	// ReportStageStart reports the start of a stage
	ReportStageStart(ctx context.Context, stage Stage, input *StageInput)
	
	// ReportStageProgress reports progress within a stage
	ReportStageProgress(ctx context.Context, stage Stage, progress *StageProgress)
	
	// ReportStageComplete reports stage completion
	ReportStageComplete(ctx context.Context, stage Stage, output *StageOutput)
	
	// ReportStageError reports a stage error
	ReportStageError(ctx context.Context, stage Stage, err error)
	
	// ReportWorkflowComplete reports workflow completion
	ReportWorkflowComplete(ctx context.Context, result *WorkflowResult)
}

// WorkflowRequest represents a complete workflow execution request.
type WorkflowRequest struct {
	// ID is the unique workflow identifier
	ID string `json:"id"`
	
	// Config contains workflow configuration
	Config *WorkflowConfig `json:"config"`
	
	// BuildRequest contains the build request
	BuildRequest *builder.BuildRequest `json:"build_request"`
	
	// Options contains workflow options
	Options *WorkflowOptions `json:"options,omitempty"`
	
	// Context contains workflow context data
	Context map[string]interface{} `json:"context,omitempty"`
}

// WorkflowConfig defines the workflow configuration.
type WorkflowConfig struct {
	// Stages defines the stages to execute
	Stages []StageConfig `json:"stages"`
	
	// ErrorHandling defines error handling behavior
	ErrorHandling *ErrorHandlingConfig `json:"error_handling,omitempty"`
	
	// Parallelism defines parallelism settings
	Parallelism *ParallelismConfig `json:"parallelism,omitempty"`
	
	// Timeout defines workflow timeout
	Timeout time.Duration `json:"timeout,omitempty"`
	
	// Retries defines retry behavior
	Retries *RetryConfig `json:"retries,omitempty"`
}

// StageConfig defines configuration for a single stage.
type StageConfig struct {
	// Stage is the stage type
	Stage Stage `json:"stage"`
	
	// Enabled indicates if the stage is enabled
	Enabled bool `json:"enabled"`
	
	// Conditions defines conditions for stage execution
	Conditions []string `json:"conditions,omitempty"`
	
	// Timeout defines stage timeout
	Timeout time.Duration `json:"timeout,omitempty"`
	
	// Retries defines retry behavior for this stage
	Retries int `json:"retries,omitempty"`
	
	// Options contains stage-specific options
	Options map[string]interface{} `json:"options,omitempty"`
	
	// Dependencies defines stage dependencies
	Dependencies []Stage `json:"dependencies,omitempty"`
}

// ErrorHandlingConfig defines error handling configuration.
type ErrorHandlingConfig struct {
	// Strategy defines the error handling strategy
	Strategy ErrorStrategy `json:"strategy"`
	
	// FailFast stops execution on first error
	FailFast bool `json:"fail_fast,omitempty"`
	
	// IgnoreErrors defines errors to ignore
	IgnoreErrors []string `json:"ignore_errors,omitempty"`
	
	// MaxErrors defines maximum errors before stopping
	MaxErrors int `json:"max_errors,omitempty"`
}

// ParallelismConfig defines parallelism configuration.
type ParallelismConfig struct {
	// MaxConcurrency defines maximum concurrent stages
	MaxConcurrency int `json:"max_concurrency,omitempty"`
	
	// EnableParallelism enables parallel stage execution
	EnableParallelism bool `json:"enable_parallelism,omitempty"`
	
	// ParallelStages defines which stages can run in parallel
	ParallelStages [][]Stage `json:"parallel_stages,omitempty"`
}

// RetryConfig defines retry configuration.
type RetryConfig struct {
	// MaxRetries defines maximum retry attempts
	MaxRetries int `json:"max_retries,omitempty"`
	
	// InitialDelay defines initial retry delay
	InitialDelay time.Duration `json:"initial_delay,omitempty"`
	
	// MaxDelay defines maximum retry delay
	MaxDelay time.Duration `json:"max_delay,omitempty"`
	
	// BackoffMultiplier defines backoff multiplier
	BackoffMultiplier float64 `json:"backoff_multiplier,omitempty"`
	
	// RetryableErrors defines which errors are retryable
	RetryableErrors []string `json:"retryable_errors,omitempty"`
}

// WorkflowOptions contains workflow execution options.
type WorkflowOptions struct {
	// DryRun runs workflow without making changes
	DryRun bool `json:"dry_run,omitempty"`
	
	// Verbose enables verbose logging
	Verbose bool `json:"verbose,omitempty"`
	
	// ProgressReporting enables progress reporting
	ProgressReporting bool `json:"progress_reporting,omitempty"`
	
	// ContinueOnError continues execution on non-fatal errors
	ContinueOnError bool `json:"continue_on_error,omitempty"`
	
	// SkipStages defines stages to skip
	SkipStages []Stage `json:"skip_stages,omitempty"`
	
	// OnlyStages defines stages to run exclusively
	OnlyStages []Stage `json:"only_stages,omitempty"`
}

// WorkflowResult contains the result of workflow execution.
type WorkflowResult struct {
	// ID is the workflow identifier
	ID string `json:"id"`
	
	// Status is the workflow status
	Status WorkflowStatus `json:"status"`
	
	// StartTime is when the workflow started
	StartTime time.Time `json:"start_time"`
	
	// EndTime is when the workflow completed
	EndTime time.Time `json:"end_time"`
	
	// Duration is the total workflow duration
	Duration time.Duration `json:"duration"`
	
	// BuildResult contains the build result
	BuildResult *builder.BuildResult `json:"build_result,omitempty"`
	
	// StageResults contains results from each stage
	StageResults map[Stage]*StageOutput `json:"stage_results"`
	
	// Errors contains workflow errors
	Errors []WorkflowError `json:"errors,omitempty"`
	
	// Warnings contains workflow warnings
	Warnings []string `json:"warnings,omitempty"`
	
	// Artifacts contains generated artifacts
	Artifacts map[string]*Artifact `json:"artifacts,omitempty"`
	
	// Metadata contains workflow metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Stage represents a workflow stage.
type Stage string

const (
	// Core build stages
	StageValidation     Stage = "validation"      // Validate inputs and configuration
	StageContextPrep    Stage = "context_prep"    // Prepare build context
	StageDockerfileParse Stage = "dockerfile_parse" // Parse Dockerfile
	StageCacheResolve   Stage = "cache_resolve"   // Resolve cache dependencies
	StageBuild          Stage = "build"           // Execute build
	StageImageAssembly  Stage = "image_assembly"  // Assemble final image
	
	// Security and compliance stages
	StageSBOMGeneration Stage = "sbom_generation" // Generate SBOM
	StageSigning        Stage = "signing"         // Sign image
	StageAttestation    Stage = "attestation"     // Generate attestations
	StageVulnScan       Stage = "vuln_scan"       // Vulnerability scanning
	StagePolicyCheck    Stage = "policy_check"    // Policy validation
	
	// Output stages
	StageRegistryPush   Stage = "registry_push"   // Push to registry
	StageCacheExport    Stage = "cache_export"    // Export cache
	StageArtifactExport Stage = "artifact_export" // Export artifacts
	StageNotification   Stage = "notification"    // Send notifications
	
	// Cleanup stages
	StageCleanup        Stage = "cleanup"         // Cleanup resources
)

// StageInput contains input data for a stage.
type StageInput struct {
	// Stage is the current stage
	Stage Stage `json:"stage"`
	
	// WorkflowID is the workflow identifier
	WorkflowID string `json:"workflow_id"`
	
	// BuildRequest contains the build request
	BuildRequest *builder.BuildRequest `json:"build_request,omitempty"`
	
	// DockerfileAST contains the parsed Dockerfile
	DockerfileAST *dockerfile.AST `json:"dockerfile_ast,omitempty"`
	
	// BuildResult contains results from the build stage
	BuildResult *builder.BuildResult `json:"build_result,omitempty"`
	
	// SBOM contains the generated SBOM
	SBOM *sbom.SBOM `json:"sbom,omitempty"`
	
	// Signature contains image signature
	Signature *signing.SignResult `json:"signature,omitempty"`
	
	// CacheManager provides access to cache operations
	CacheManager cache.Manager `json:"-"`
	
	// RegistryClient provides access to registry operations
	RegistryClient registry.Client `json:"-"`
	
	// Context contains stage context data
	Context map[string]interface{} `json:"context,omitempty"`
	
	// PreviousOutputs contains outputs from previous stages
	PreviousOutputs map[Stage]*StageOutput `json:"previous_outputs,omitempty"`
}

// StageOutput contains output data from a stage.
type StageOutput struct {
	// Stage is the stage that produced this output
	Stage Stage `json:"stage"`
	
	// Status is the stage execution status
	Status StageStatus `json:"status"`
	
	// StartTime is when the stage started
	StartTime time.Time `json:"start_time"`
	
	// EndTime is when the stage completed
	EndTime time.Time `json:"end_time"`
	
	// Duration is the stage execution duration
	Duration time.Duration `json:"duration"`
	
	// Data contains stage-specific output data
	Data map[string]interface{} `json:"data,omitempty"`
	
	// Artifacts contains generated artifacts
	Artifacts []*Artifact `json:"artifacts,omitempty"`
	
	// Errors contains stage errors
	Errors []string `json:"errors,omitempty"`
	
	// Warnings contains stage warnings
	Warnings []string `json:"warnings,omitempty"`
	
	// Metadata contains stage metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// WorkflowStatus represents the status of workflow execution.
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "pending"   // Workflow is queued
	WorkflowStatusRunning   WorkflowStatus = "running"   // Workflow is executing
	WorkflowStatusSuccess   WorkflowStatus = "success"   // Workflow completed successfully
	WorkflowStatusFailed    WorkflowStatus = "failed"    // Workflow failed
	WorkflowStatusCanceled  WorkflowStatus = "canceled"  // Workflow was canceled
	WorkflowStatusSkipped   WorkflowStatus = "skipped"   // Workflow was skipped
)

// StageStatus represents the status of stage execution.
type StageStatus string

const (
	StageStatusPending   StageStatus = "pending"   // Stage is waiting to run
	StageStatusRunning   StageStatus = "running"   // Stage is executing
	StageStatusSuccess   StageStatus = "success"   // Stage completed successfully
	StageStatusFailed    StageStatus = "failed"    // Stage failed
	StageStatusSkipped   StageStatus = "skipped"   // Stage was skipped
	StageStatusCanceled  StageStatus = "canceled"  // Stage was canceled
)

// ErrorStrategy defines how to handle errors.
type ErrorStrategy string

const (
	ErrorStrategyFail     ErrorStrategy = "fail"     // Fail on any error
	ErrorStrategyContinue ErrorStrategy = "continue" // Continue on errors
	ErrorStrategyRetry    ErrorStrategy = "retry"    // Retry on retryable errors
)

// WorkflowError represents an error during workflow execution.
type WorkflowError struct {
	// Stage is the stage where the error occurred
	Stage Stage `json:"stage"`
	
	// Error is the error message
	Error string `json:"error"`
	
	// Code is the error code
	Code string `json:"code,omitempty"`
	
	// Timestamp is when the error occurred
	Timestamp time.Time `json:"timestamp"`
	
	// Recoverable indicates if the error is recoverable
	Recoverable bool `json:"recoverable,omitempty"`
	
	// Context contains error context
	Context map[string]interface{} `json:"context,omitempty"`
}

// ErrorResult contains the result of error handling.
type ErrorResult struct {
	// Action is the action to take
	Action ErrorAction `json:"action"`
	
	// RetryDelay is the delay before retry
	RetryDelay time.Duration `json:"retry_delay,omitempty"`
	
	// Message is an optional message
	Message string `json:"message,omitempty"`
	
	// Context contains additional context
	Context map[string]interface{} `json:"context,omitempty"`
}

// ErrorAction defines the action to take after an error.
type ErrorAction string

const (
	ErrorActionFail     ErrorAction = "fail"     // Fail the workflow
	ErrorActionContinue ErrorAction = "continue" // Continue execution
	ErrorActionRetry    ErrorAction = "retry"    // Retry the stage
	ErrorActionSkip     ErrorAction = "skip"     // Skip the stage
)

// WorkflowProgress represents workflow execution progress.
type WorkflowProgress struct {
	// WorkflowID is the workflow identifier
	WorkflowID string `json:"workflow_id"`
	
	// Status is the current workflow status
	Status WorkflowStatus `json:"status"`
	
	// CurrentStage is the currently executing stage
	CurrentStage Stage `json:"current_stage,omitempty"`
	
	// CompletedStages contains completed stages
	CompletedStages []Stage `json:"completed_stages"`
	
	// RemainingStages contains remaining stages
	RemainingStages []Stage `json:"remaining_stages"`
	
	// StageProgress contains progress for the current stage
	StageProgress *StageProgress `json:"stage_progress,omitempty"`
	
	// OverallProgress is the overall progress percentage
	OverallProgress float64 `json:"overall_progress"`
	
	// StartTime is when the workflow started
	StartTime time.Time `json:"start_time"`
	
	// ElapsedTime is the elapsed execution time
	ElapsedTime time.Duration `json:"elapsed_time"`
	
	// EstimatedRemaining is the estimated remaining time
	EstimatedRemaining time.Duration `json:"estimated_remaining,omitempty"`
}

// StageProgress represents progress within a stage.
type StageProgress struct {
	// Stage is the stage
	Stage Stage `json:"stage"`
	
	// Status is the stage status
	Status StageStatus `json:"status"`
	
	// Progress is the stage progress percentage
	Progress float64 `json:"progress"`
	
	// Message is a progress message
	Message string `json:"message,omitempty"`
	
	// Details contains detailed progress information
	Details map[string]interface{} `json:"details,omitempty"`
	
	// StartTime is when the stage started
	StartTime time.Time `json:"start_time"`
	
	// ElapsedTime is the elapsed stage time
	ElapsedTime time.Duration `json:"elapsed_time"`
}

// Artifact represents a generated artifact.
type Artifact struct {
	// ID is the artifact identifier
	ID string `json:"id"`
	
	// Name is the artifact name
	Name string `json:"name"`
	
	// Type is the artifact type
	Type ArtifactType `json:"type"`
	
	// Path is the artifact path
	Path string `json:"path,omitempty"`
	
	// URL is the artifact URL
	URL string `json:"url,omitempty"`
	
	// Size is the artifact size
	Size int64 `json:"size,omitempty"`
	
	// Checksum is the artifact checksum
	Checksum string `json:"checksum,omitempty"`
	
	// MediaType is the artifact media type
	MediaType string `json:"media_type,omitempty"`
	
	// CreatedAt is when the artifact was created
	CreatedAt time.Time `json:"created_at"`
	
	// Metadata contains artifact metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ArtifactType represents the type of artifact.
type ArtifactType string

const (
	ArtifactTypeImage       ArtifactType = "image"       // Container image
	ArtifactTypeManifest    ArtifactType = "manifest"    // Image manifest
	ArtifactTypeSBOM        ArtifactType = "sbom"        // Software Bill of Materials
	ArtifactTypeSignature   ArtifactType = "signature"   // Image signature
	ArtifactTypeAttestation ArtifactType = "attestation" // Attestation
	ArtifactTypeReport      ArtifactType = "report"      // Report (vulnerability, etc.)
	ArtifactTypeLog         ArtifactType = "log"         // Log file
	ArtifactTypeCache       ArtifactType = "cache"       // Cache export
	ArtifactTypeArchive     ArtifactType = "archive"     // Archive file
)

// DataFlow represents the flow of data between workflow stages.
type DataFlow struct {
	// Stages maps each stage to its data requirements and outputs
	Stages map[Stage]*StageDataFlow `json:"stages"`
	
	// Dependencies maps stage dependencies
	Dependencies map[Stage][]Stage `json:"dependencies"`
	
	// DataTypes defines the data types that flow between stages
	DataTypes map[string]*DataType `json:"data_types"`
}

// StageDataFlow defines data flow for a single stage.
type StageDataFlow struct {
	// Inputs defines required input data
	Inputs []*DataRequirement `json:"inputs"`
	
	// Outputs defines produced output data
	Outputs []*DataOutput `json:"outputs"`
	
	// OptionalInputs defines optional input data
	OptionalInputs []*DataRequirement `json:"optional_inputs,omitempty"`
}

// DataRequirement defines a data requirement for a stage.
type DataRequirement struct {
	// Name is the data name
	Name string `json:"name"`
	
	// Type is the data type
	Type string `json:"type"`
	
	// Source is the source stage
	Source Stage `json:"source,omitempty"`
	
	// Required indicates if this data is required
	Required bool `json:"required"`
	
	// Validation defines data validation rules
	Validation map[string]interface{} `json:"validation,omitempty"`
}

// DataOutput defines data output from a stage.
type DataOutput struct {
	// Name is the data name
	Name string `json:"name"`
	
	// Type is the data type
	Type string `json:"type"`
	
	// Description is a description of the data
	Description string `json:"description,omitempty"`
	
	// Schema defines the data schema
	Schema map[string]interface{} `json:"schema,omitempty"`
}

// DataType defines a data type used in the workflow.
type DataType struct {
	// Name is the type name
	Name string `json:"name"`
	
	// Description is the type description
	Description string `json:"description"`
	
	// Schema defines the type schema
	Schema map[string]interface{} `json:"schema"`
	
	// Validation defines validation rules
	Validation map[string]interface{} `json:"validation,omitempty"`
}
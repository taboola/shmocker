// Package dockerfile defines interfaces for Dockerfile parsing and AST representation.
package dockerfile

import (
	"io"
	"time"
)

// Parser provides the interface for parsing Dockerfiles into an AST representation.
type Parser interface {
	// Parse parses a Dockerfile from the given reader and returns an AST
	Parse(reader io.Reader) (*AST, error)
	
	// ParseFile parses a Dockerfile from the specified file path
	ParseFile(path string) (*AST, error)
	
	// ParseBytes parses a Dockerfile from byte content
	ParseBytes(content []byte) (*AST, error)
	
	// Validate performs syntax validation on a Dockerfile AST
	Validate(ast *AST) error
}

// LLBConverter converts Dockerfile AST to BuildKit's Low Level Builder (LLB) format.
type LLBConverter interface {
	// Convert transforms a Dockerfile AST into LLB definition
	Convert(ast *AST, opts *ConvertOptions) (*LLBDefinition, error)
	
	// ConvertStage converts a single build stage to LLB
	ConvertStage(stage *Stage, opts *ConvertOptions) (*LLBState, error)
	
	// ResolveBaseImage resolves the base image reference for a stage
	ResolveBaseImage(from *FromInstruction) (*ImageReference, error)
}

// AST represents the Abstract Syntax Tree of a parsed Dockerfile.
type AST struct {
	// Stages contains all build stages in the Dockerfile
	Stages []*Stage `json:"stages"`
	
	// Directives contains parser directives (escape, syntax)
	Directives []*Directive `json:"directives,omitempty"`
	
	// Comments contains preserved comments
	Comments []*Comment `json:"comments,omitempty"`
	
	// Metadata contains parsing metadata
	Metadata *ParseMetadata `json:"metadata"`
}

// Stage represents a single build stage in a multi-stage Dockerfile.
type Stage struct {
	// Name is the optional stage name (from AS clause)
	Name string `json:"name,omitempty"`
	
	// Index is the zero-based index of this stage
	Index int `json:"index"`
	
	// From is the FROM instruction that starts this stage
	From *FromInstruction `json:"from"`
	
	// Instructions contains all instructions in this stage
	Instructions []Instruction `json:"instructions"`
	
	// Platform is the target platform for this stage
	Platform string `json:"platform,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Instruction represents a single Dockerfile instruction.
type Instruction interface {
	// GetCmd returns the instruction command (FROM, RUN, COPY, etc.)
	GetCmd() string
	
	// GetArgs returns the instruction arguments
	GetArgs() []string
	
	// GetFlags returns instruction flags
	GetFlags() map[string]string
	
	// GetLocation returns source location information
	GetLocation() *SourceLocation
	
	// String returns the string representation of the instruction
	String() string
	
	// Validate performs instruction-specific validation
	Validate() error
}

// FromInstruction represents a FROM instruction.
type FromInstruction struct {
	// Image is the base image reference
	Image string `json:"image"`
	
	// Tag is the image tag
	Tag string `json:"tag,omitempty"`
	
	// Digest is the image digest
	Digest string `json:"digest,omitempty"`
	
	// Stage is the source stage name (for multi-stage builds)
	Stage string `json:"stage,omitempty"`
	
	// As is the stage alias
	As string `json:"as,omitempty"`
	
	// Platform is the target platform
	Platform string `json:"platform,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for FromInstruction
func (f *FromInstruction) GetCmd() string { return "FROM" }
func (f *FromInstruction) GetArgs() []string {
	args := []string{f.Image}
	if f.Tag != "" {
		args[0] += ":" + f.Tag
	}
	if f.Digest != "" {
		args[0] += "@" + f.Digest
	}
	if f.As != "" {
		args = append(args, "AS", f.As)
	}
	return args
}
func (f *FromInstruction) GetFlags() map[string]string {
	flags := make(map[string]string)
	if f.Platform != "" {
		flags["platform"] = f.Platform
	}
	return flags
}
func (f *FromInstruction) GetLocation() *SourceLocation { return f.Location }
func (f *FromInstruction) String() string {
	return "FROM " + f.Image
}
func (f *FromInstruction) Validate() error { return nil }

// RunInstruction represents a RUN instruction.
type RunInstruction struct {
	// Commands contains the command to execute
	Commands []string `json:"commands"`
	
	// Shell indicates if this is a shell form command
	Shell bool `json:"shell"`
	
	// Mounts contains mount instructions
	Mounts []*MountInstruction `json:"mounts,omitempty"`
	
	// Network specifies network mode
	Network string `json:"network,omitempty"`
	
	// Security specifies security mode
	Security string `json:"security,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for RunInstruction
func (r *RunInstruction) GetCmd() string { return "RUN" }
func (r *RunInstruction) GetArgs() []string { return r.Commands }
func (r *RunInstruction) GetFlags() map[string]string {
	flags := make(map[string]string)
	if r.Network != "" {
		flags["network"] = r.Network
	}
	if r.Security != "" {
		flags["security"] = r.Security
	}
	return flags
}
func (r *RunInstruction) GetLocation() *SourceLocation { return r.Location }
func (r *RunInstruction) String() string {
	return "RUN " + r.Commands[0]
}
func (r *RunInstruction) Validate() error { return nil }

// CopyInstruction represents a COPY instruction.
type CopyInstruction struct {
	// Sources contains source paths
	Sources []string `json:"sources"`
	
	// Destination is the destination path
	Destination string `json:"destination"`
	
	// From specifies the source stage or image
	From string `json:"from,omitempty"`
	
	// Chown specifies ownership
	Chown string `json:"chown,omitempty"`
	
	// Chmod specifies permissions
	Chmod string `json:"chmod,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for CopyInstruction
func (c *CopyInstruction) GetCmd() string { return "COPY" }
func (c *CopyInstruction) GetArgs() []string {
	args := make([]string, 0, len(c.Sources)+1)
	args = append(args, c.Sources...)
	args = append(args, c.Destination)
	return args
}
func (c *CopyInstruction) GetFlags() map[string]string {
	flags := make(map[string]string)
	if c.From != "" {
		flags["from"] = c.From
	}
	if c.Chown != "" {
		flags["chown"] = c.Chown
	}
	if c.Chmod != "" {
		flags["chmod"] = c.Chmod
	}
	return flags
}
func (c *CopyInstruction) GetLocation() *SourceLocation { return c.Location }
func (c *CopyInstruction) String() string {
	return "COPY " + c.Sources[0] + " " + c.Destination
}
func (c *CopyInstruction) Validate() error { return nil }

// AddInstruction represents an ADD instruction.
type AddInstruction struct {
	// Sources contains source paths or URLs
	Sources []string `json:"sources"`
	
	// Destination is the destination path
	Destination string `json:"destination"`
	
	// Chown specifies ownership
	Chown string `json:"chown,omitempty"`
	
	// Chmod specifies permissions
	Chmod string `json:"chmod,omitempty"`
	
	// Checksum specifies expected checksum
	Checksum string `json:"checksum,omitempty"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for AddInstruction
func (a *AddInstruction) GetCmd() string { return "ADD" }
func (a *AddInstruction) GetArgs() []string {
	args := make([]string, 0, len(a.Sources)+1)
	args = append(args, a.Sources...)
	args = append(args, a.Destination)
	return args
}
func (a *AddInstruction) GetFlags() map[string]string {
	flags := make(map[string]string)
	if a.Chown != "" {
		flags["chown"] = a.Chown
	}
	if a.Chmod != "" {
		flags["chmod"] = a.Chmod
	}
	if a.Checksum != "" {
		flags["checksum"] = a.Checksum
	}
	return flags
}
func (a *AddInstruction) GetLocation() *SourceLocation { return a.Location }
func (a *AddInstruction) String() string {
	return "ADD " + a.Sources[0] + " " + a.Destination
}
func (a *AddInstruction) Validate() error { return nil }

// EnvInstruction represents an ENV instruction.
type EnvInstruction struct {
	// Variables contains environment variables
	Variables map[string]string `json:"variables"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Implement Instruction interface for EnvInstruction
func (e *EnvInstruction) GetCmd() string { return "ENV" }
func (e *EnvInstruction) GetArgs() []string {
	args := make([]string, 0, len(e.Variables)*2)
	for k, v := range e.Variables {
		args = append(args, k, v)
	}
	return args
}
func (e *EnvInstruction) GetFlags() map[string]string { return nil }
func (e *EnvInstruction) GetLocation() *SourceLocation { return e.Location }
func (e *EnvInstruction) String() string {
	for k, v := range e.Variables {
		return "ENV " + k + "=" + v
	}
	return "ENV"
}
func (e *EnvInstruction) Validate() error { return nil }

// MountInstruction represents a mount in a RUN instruction.
type MountInstruction struct {
	// Type is the mount type (bind, cache, tmpfs, secret, ssh)
	Type string `json:"type"`
	
	// Source is the source path
	Source string `json:"source,omitempty"`
	
	// Target is the target path
	Target string `json:"target"`
	
	// Options contains mount options
	Options map[string]string `json:"options,omitempty"`
}

// Directive represents a parser directive.
type Directive struct {
	// Name is the directive name (escape, syntax)
	Name string `json:"name"`
	
	// Value is the directive value
	Value string `json:"value"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// Comment represents a preserved comment.
type Comment struct {
	// Text is the comment text
	Text string `json:"text"`
	
	// Location contains source location information
	Location *SourceLocation `json:"location"`
}

// SourceLocation contains source location information for instructions.
type SourceLocation struct {
	// Line is the line number (1-based)
	Line int `json:"line"`
	
	// Column is the column number (1-based)
	Column int `json:"column"`
	
	// StartLine is the starting line for multi-line instructions
	StartLine int `json:"start_line,omitempty"`
	
	// EndLine is the ending line for multi-line instructions
	EndLine int `json:"end_line,omitempty"`
}

// ParseMetadata contains metadata about the parsing process.
type ParseMetadata struct {
	// Filename is the source filename
	Filename string `json:"filename,omitempty"`
	
	// ParseTime is when the parsing occurred
	ParseTime time.Time `json:"parse_time"`
	
	// ParserVersion is the version of the parser used
	ParserVersion string `json:"parser_version"`
	
	// Warnings contains parsing warnings
	Warnings []*ParseWarning `json:"warnings,omitempty"`
	
	// Syntax is the Dockerfile syntax version
	Syntax string `json:"syntax,omitempty"`
}

// ParseWarning represents a parsing warning.
type ParseWarning struct {
	// Message is the warning message
	Message string `json:"message"`
	
	// Location is where the warning occurred
	Location *SourceLocation `json:"location,omitempty"`
	
	// Severity is the warning severity
	Severity WarningSeverity `json:"severity"`
}

// WarningSeverity represents the severity of a parsing warning.
type WarningSeverity string

const (
	WarningInfo    WarningSeverity = "info"
	WarningWarning WarningSeverity = "warning"
	WarningError   WarningSeverity = "error"
)

// ConvertOptions contains options for converting Dockerfile AST to LLB.
type ConvertOptions struct {
	// BuildArgs contains build-time variables
	BuildArgs map[string]string `json:"build_args,omitempty"`
	
	// Platform is the target platform
	Platform string `json:"platform,omitempty"`
	
	// Target is the target stage
	Target string `json:"target,omitempty"`
	
	// Labels contains image labels
	Labels map[string]string `json:"labels,omitempty"`
	
	// NetworkMode specifies network mode
	NetworkMode string `json:"network_mode,omitempty"`
}

// LLBDefinition represents a BuildKit LLB definition.
type LLBDefinition struct {
	// Definition is the serialized LLB definition
	Definition []byte `json:"definition"`
	
	// Metadata contains LLB metadata
	Metadata map[string][]byte `json:"metadata,omitempty"`
}

// LLBState represents a BuildKit LLB state.
type LLBState struct {
	// State is the LLB state
	State interface{} `json:"state"`
	
	// Metadata contains state metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ImageReference represents a container image reference.
type ImageReference struct {
	// Registry is the registry hostname
	Registry string `json:"registry,omitempty"`
	
	// Namespace is the image namespace
	Namespace string `json:"namespace,omitempty"`
	
	// Repository is the repository name
	Repository string `json:"repository"`
	
	// Tag is the image tag
	Tag string `json:"tag,omitempty"`
	
	// Digest is the image digest
	Digest string `json:"digest,omitempty"`
}

// String returns the full image reference string.
func (r *ImageReference) String() string {
	ref := ""
	if r.Registry != "" {
		ref += r.Registry + "/"
	}
	if r.Namespace != "" {
		ref += r.Namespace + "/"
	}
	ref += r.Repository
	if r.Tag != "" {
		ref += ":" + r.Tag
	}
	if r.Digest != "" {
		ref += "@" + r.Digest
	}
	return ref
}
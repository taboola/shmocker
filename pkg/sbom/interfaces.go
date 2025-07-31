// Package sbom defines interfaces for Software Bill of Materials generation.
package sbom

import (
	"context"
	"io"
	"time"
)

// Generator provides the main interface for generating SBOMs from container images.
type Generator interface {
	// Generate creates an SBOM for the given image
	Generate(ctx context.Context, req *GenerateRequest) (*SBOM, error)
	
	// GenerateFromFilesystem creates an SBOM from a filesystem path
	GenerateFromFilesystem(ctx context.Context, path string, opts *GenerateOptions) (*SBOM, error)
	
	// GenerateFromLayers creates an SBOM from individual image layers
	GenerateFromLayers(ctx context.Context, layers []*LayerInfo, opts *GenerateOptions) (*SBOM, error)
	
	// Merge combines multiple SBOMs into one
	Merge(ctx context.Context, sboms []*SBOM) (*SBOM, error)
}

// Scanner provides the interface for scanning individual components or files.
type Scanner interface {
	// ScanFile scans a single file for package information
	ScanFile(ctx context.Context, path string) ([]*Package, error)
	
	// ScanDirectory scans a directory recursively
	ScanDirectory(ctx context.Context, path string, opts *ScanOptions) ([]*Package, error)
	
	// ScanLayer scans a single image layer
	ScanLayer(ctx context.Context, layer *LayerInfo) ([]*Package, error)
	
	// GetSupportedTypes returns the package types this scanner supports
	GetSupportedTypes() []PackageType
	
	// GetName returns the scanner name
	GetName() string
}

// Serializer provides interfaces for serializing SBOMs to different formats.
type Serializer interface {
	// Serialize converts an SBOM to the specified format
	Serialize(sbom *SBOM, format Format) ([]byte, error)
	
	// SerializeToWriter writes an SBOM to a writer in the specified format
	SerializeToWriter(sbom *SBOM, format Format, writer io.Writer) error
	
	// Deserialize converts serialized data back to an SBOM
	Deserialize(data []byte, format Format) (*SBOM, error)
	
	// GetSupportedFormats returns the formats this serializer supports
	GetSupportedFormats() []Format
}

// Validator provides validation capabilities for SBOMs.
type Validator interface {
	// Validate checks if an SBOM is valid according to its format specification
	Validate(sbom *SBOM) (*ValidationResult, error)
	
	// ValidateData validates serialized SBOM data
	ValidateData(data []byte, format Format) (*ValidationResult, error)
	
	// GetSchema returns the schema for the specified format
	GetSchema(format Format) ([]byte, error)
}

// AttestationGenerator creates attestations for SBOMs.
type AttestationGenerator interface {
	// GenerateAttestation creates a signed attestation for an SBOM
	GenerateAttestation(ctx context.Context, sbom *SBOM, opts *AttestationOptions) (*Attestation, error)
	
	// VerifyAttestation verifies an SBOM attestation
	VerifyAttestation(ctx context.Context, attestation *Attestation) (*VerificationResult, error)
	
	// AttachToImage attaches an SBOM attestation to a container image
	AttachToImage(ctx context.Context, imageRef string, attestation *Attestation) error
}

// GenerateRequest represents a request to generate an SBOM.
type GenerateRequest struct {
	// ImageRef is the container image reference
	ImageRef string `json:"image_ref"`
	
	// ImageDigest is the specific image digest
	ImageDigest string `json:"image_digest,omitempty"`
	
	// Platform specifies the target platform
	Platform string `json:"platform,omitempty"`
	
	// Options contains generation options
	Options *GenerateOptions `json:"options,omitempty"`
	
	// Auth provides registry authentication
	Auth *RegistryAuth `json:"auth,omitempty"`
}

// GenerateOptions contains options for SBOM generation.
type GenerateOptions struct {
	// Format specifies the output format
	Format Format `json:"format"`
	
	// IncludeFiles includes file listings in the SBOM
	IncludeFiles bool `json:"include_files,omitempty"`
	
	// IncludeSecrets includes secret scanning results
	IncludeSecrets bool `json:"include_secrets,omitempty"`
	
	// IncludeVulnerabilities includes vulnerability information
	IncludeVulnerabilities bool `json:"include_vulnerabilities,omitempty"`
	
	// ScannerTypes specifies which scanners to use
	ScannerTypes []PackageType `json:"scanner_types,omitempty"`
	
	// Depth controls scanning depth
	Depth int `json:"depth,omitempty"`
	
	// ExcludePaths contains paths to exclude from scanning
	ExcludePaths []string `json:"exclude_paths,omitempty"`
	
	// IncludePaths contains paths to specifically include
	IncludePaths []string `json:"include_paths,omitempty"`
	
	// Metadata contains additional metadata to include
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ScanOptions contains options for package scanning.
type ScanOptions struct {
	// Recursive enables recursive directory scanning
	Recursive bool `json:"recursive,omitempty"`
	
	// FollowSymlinks enables following symbolic links
	FollowSymlinks bool `json:"follow_symlinks,omitempty"`
	
	// MaxDepth limits scanning depth
	MaxDepth int `json:"max_depth,omitempty"`
	
	// ExcludePatterns contains file patterns to exclude
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`
	
	// IncludePatterns contains file patterns to include
	IncludePatterns []string `json:"include_patterns,omitempty"`
}

// SBOM represents a Software Bill of Materials.
type SBOM struct {
	// Metadata contains SBOM metadata
	Metadata *Metadata `json:"metadata"`
	
	// Packages contains discovered packages
	Packages []*Package `json:"packages"`
	
	// Files contains file information (if included)
	Files []*File `json:"files,omitempty"`
	
	// Relationships describes relationships between components
	Relationships []*Relationship `json:"relationships,omitempty"`
	
	// Vulnerabilities contains vulnerability information (if included)
	Vulnerabilities []*Vulnerability `json:"vulnerabilities,omitempty"`
	
	// Secrets contains discovered secrets (if included)
	Secrets []*Secret `json:"secrets,omitempty"`
	
	// Annotations contains arbitrary metadata
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Metadata contains SBOM metadata information.
type Metadata struct {
	// ID is the unique SBOM identifier
	ID string `json:"id"`
	
	// Name is the SBOM name
	Name string `json:"name"`
	
	// Version is the SBOM version
	Version string `json:"version"`
	
	// Format is the SBOM format
	Format Format `json:"format"`
	
	// Timestamp is when the SBOM was generated
	Timestamp time.Time `json:"timestamp"`
	
	// Generator contains information about the SBOM generator
	Generator *GeneratorInfo `json:"generator"`
	
	// Subject contains information about what was scanned
	Subject *Subject `json:"subject"`
	
	// Supplier contains supplier information
	Supplier *Entity `json:"supplier,omitempty"`
	
	// Author contains author information
	Author *Entity `json:"author,omitempty"`
	
	// CreatedBy contains creation tool information
	CreatedBy []*Entity `json:"created_by,omitempty"`
	
	// Namespace is the SBOM namespace
	Namespace string `json:"namespace,omitempty"`
	
	// DocumentRef is the document reference
	DocumentRef string `json:"document_ref,omitempty"`
}

// GeneratorInfo contains information about the SBOM generator.
type GeneratorInfo struct {
	// Name is the generator name
	Name string `json:"name"`
	
	// Version is the generator version
	Version string `json:"version"`
	
	// Configuration contains generator configuration
	Configuration map[string]interface{} `json:"configuration,omitempty"`
}

// Subject contains information about what was scanned.
type Subject struct {
	// Type is the subject type (image, directory, etc.)
	Type string `json:"type"`
	
	// Name is the subject name or reference
	Name string `json:"name"`
	
	// Digest is the subject digest (for images)
	Digest string `json:"digest,omitempty"`
	
	// Size is the subject size
	Size int64 `json:"size,omitempty"`
	
	// Metadata contains additional subject metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Entity represents an entity (person, organization, or tool).
type Entity struct {
	// Name is the entity name
	Name string `json:"name"`
	
	// Email is the entity email
	Email string `json:"email,omitempty"`
	
	// URL is the entity URL
	URL string `json:"url,omitempty"`
	
	// Type is the entity type
	Type EntityType `json:"type,omitempty"`
}

// EntityType represents the type of entity.
type EntityType string

const (
	EntityTypePerson       EntityType = "person"
	EntityTypeOrganization EntityType = "organization"
	EntityTypeTool         EntityType = "tool"
)

// Package represents a discovered software package.
type Package struct {
	// ID is the unique package identifier
	ID string `json:"id"`
	
	// Name is the package name
	Name string `json:"name"`
	
	// Version is the package version
	Version string `json:"version"`
	
	// Type is the package type
	Type PackageType `json:"type"`
	
	// PURL is the Package URL identifier
	PURL string `json:"purl,omitempty"`
	
	// CPE is the Common Platform Enumeration identifier
	CPE string `json:"cpe,omitempty"`
	
	// Description is the package description
	Description string `json:"description,omitempty"`
	
	// Homepage is the package homepage URL
	Homepage string `json:"homepage,omitempty"`
	
	// SourceInfo contains source information
	SourceInfo *SourceInfo `json:"source_info,omitempty"`
	
	// Supplier contains supplier information
	Supplier *Entity `json:"supplier,omitempty"`
	
	// Originator contains originator information
	Originator *Entity `json:"originator,omitempty"`
	
	// Licenses contains license information
	Licenses []*License `json:"licenses,omitempty"`
	
	// Files contains associated files
	Files []*File `json:"files,omitempty"`
	
	// Dependencies contains package dependencies
	Dependencies []string `json:"dependencies,omitempty"`
	
	// Metadata contains additional package metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// PackageType represents the type of package.
type PackageType string

const (
	PackageTypeUnknown      PackageType = "unknown"
	PackageTypeApk          PackageType = "apk"
	PackageTypeDeb          PackageType = "deb"
	PackageTypeRpm          PackageType = "rpm"
	PackageTypeNPM          PackageType = "npm"
	PackageTypePyPI         PackageType = "pypi"
	PackageTypeGem          PackageType = "gem"
	PackageTypeGo           PackageType = "go"
	PackageTypeCargo        PackageType = "cargo"
	PackageTypeComposer     PackageType = "composer"
	PackageTypeNuGet        PackageType = "nuget"
	PackageTypeMaven        PackageType = "maven"
	PackageTypeGradle       PackageType = "gradle"
	PackageTypeConan        PackageType = "conan"
	PackageTypeCocoapods    PackageType = "cocoapods"
	PackageTypeSwiftPM      PackageType = "swiftpm"
	PackageTypePub          PackageType = "pub"
	PackageTypeHex          PackageType = "hex"
	PackageTypeCPAN         PackageType = "cpan"
	PackageTypeHackage      PackageType = "hackage"
)

// SourceInfo contains package source information.
type SourceInfo struct {
	// Repository is the source repository URL
	Repository string `json:"repository,omitempty"`
	
	// Revision is the source revision
	Revision string `json:"revision,omitempty"`
	
	// Branch is the source branch
	Branch string `json:"branch,omitempty"`
	
	// Tag is the source tag
	Tag string `json:"tag,omitempty"`
	
	// Path is the path within the repository
	Path string `json:"path,omitempty"`
}

// License contains license information.
type License struct {
	// ID is the SPDX license identifier
	ID string `json:"id,omitempty"`
	
	// Name is the license name
	Name string `json:"name,omitempty"`
	
	// Text is the license text
	Text string `json:"text,omitempty"`
	
	// URL is the license URL
	URL string `json:"url,omitempty"`
}

// File represents a file in the SBOM.
type File struct {
	// ID is the unique file identifier
	ID string `json:"id"`
	
	// Path is the file path
	Path string `json:"path"`
	
	// Size is the file size
	Size int64 `json:"size"`
	
	// Checksums contains file checksums
	Checksums []*Checksum `json:"checksums,omitempty"`
	
	// MimeType is the file MIME type
	MimeType string `json:"mime_type,omitempty"`
	
	// IsExecutable indicates if the file is executable
	IsExecutable bool `json:"is_executable,omitempty"`
	
	// Metadata contains additional file metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Checksum represents a file checksum.
type Checksum struct {
	// Algorithm is the checksum algorithm
	Algorithm string `json:"algorithm"`
	
	// Value is the checksum value
	Value string `json:"value"`
}

// Relationship describes a relationship between SBOM components.
type Relationship struct {
	// Subject is the relationship subject
	Subject string `json:"subject"`
	
	// Type is the relationship type
	Type RelationshipType `json:"type"`
	
	// Object is the relationship object
	Object string `json:"object"`
	
	// Comment provides additional context
	Comment string `json:"comment,omitempty"`
}

// RelationshipType represents the type of relationship.
type RelationshipType string

const (
	RelationshipContains        RelationshipType = "contains"
	RelationshipDependsOn       RelationshipType = "depends_on"
	RelationshipDescribes       RelationshipType = "describes"
	RelationshipGeneratedFrom   RelationshipType = "generated_from"
	RelationshipAncestorOf      RelationshipType = "ancestor_of"
	RelationshipDescendantOf    RelationshipType = "descendant_of"
	RelationshipVariantOf       RelationshipType = "variant_of"
)

// Vulnerability represents a security vulnerability.
type Vulnerability struct {
	// ID is the vulnerability identifier
	ID string `json:"id"`
	
	// Source is the vulnerability source
	Source string `json:"source"`
	
	// Severity is the vulnerability severity
	Severity VulnerabilitySeverity `json:"severity"`
	
	// Score is the vulnerability score
	Score float64 `json:"score,omitempty"`
	
	// Description is the vulnerability description
	Description string `json:"description,omitempty"`
	
	// References contains vulnerability references
	References []string `json:"references,omitempty"`
	
	// AffectedPackages lists affected packages
	AffectedPackages []string `json:"affected_packages,omitempty"`
	
	// FixAvailable indicates if a fix is available
	FixAvailable bool `json:"fix_available,omitempty"`
	
	// FixedVersion is the version that fixes the vulnerability
	FixedVersion string `json:"fixed_version,omitempty"`
}

// VulnerabilitySeverity represents vulnerability severity.
type VulnerabilitySeverity string

const (
	SeverityUnknown  VulnerabilitySeverity = "unknown"
	SeverityNegligible VulnerabilitySeverity = "negligible"
	SeverityLow      VulnerabilitySeverity = "low"
	SeverityMedium   VulnerabilitySeverity = "medium"
	SeverityHigh     VulnerabilitySeverity = "high"
	SeverityCritical VulnerabilitySeverity = "critical"
)

// Secret represents a discovered secret.
type Secret struct {
	// Type is the secret type
	Type string `json:"type"`
	
	// Description is the secret description
	Description string `json:"description"`
	
	// File is the file containing the secret
	File string `json:"file"`
	
	// Line is the line number where the secret was found
	Line int `json:"line,omitempty"`
	
	// Column is the column where the secret was found
	Column int `json:"column,omitempty"`
	
	// Confidence is the detection confidence
	Confidence float64 `json:"confidence,omitempty"`
}

// Format represents an SBOM format.
type Format string

const (
	FormatSPDXJSON    Format = "spdx-json"
	FormatSPDXXML     Format = "spdx-xml"
	FormatSPDXTagValue Format = "spdx-tagvalue"
	FormatCycloneDXJSON Format = "cyclonedx-json"
	FormatCycloneDXXML  Format = "cyclonedx-xml"
	FormatSYFTJSON     Format = "syft-json"
)

// LayerInfo contains information about an image layer.
type LayerInfo struct {
	// Digest is the layer digest
	Digest string `json:"digest"`
	
	// Size is the layer size
	Size int64 `json:"size"`
	
	// MediaType is the layer media type
	MediaType string `json:"media_type"`
	
	// Content provides access to the layer content
	Content io.ReadCloser `json:"-"`
	
	// Metadata contains additional layer metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RegistryAuth contains registry authentication information.
type RegistryAuth struct {
	// Username for basic authentication
	Username string `json:"username,omitempty"`
	
	// Password for basic authentication
	Password string `json:"password,omitempty"`
	
	// Token for bearer authentication
	Token string `json:"token,omitempty"`
}

// ValidationResult contains SBOM validation results.
type ValidationResult struct {
	// Valid indicates if the SBOM is valid
	Valid bool `json:"valid"`
	
	// Errors contains validation errors
	Errors []string `json:"errors,omitempty"`
	
	// Warnings contains validation warnings
	Warnings []string `json:"warnings,omitempty"`
	
	// Schema is the schema that was used for validation
	Schema string `json:"schema,omitempty"`
}

// AttestationOptions contains options for attestation generation.
type AttestationOptions struct {
	// KeyPath is the path to the signing key
	KeyPath string `json:"key_path,omitempty"`
	
	// KeyID is the key identifier
	KeyID string `json:"key_id,omitempty"`
	
	// Issuer is the attestation issuer
	Issuer string `json:"issuer,omitempty"`
	
	// Subject is the attestation subject
	Subject string `json:"subject,omitempty"`
	
	// PredicateType is the predicate type
	PredicateType string `json:"predicate_type,omitempty"`
}

// Attestation represents an SBOM attestation.
type Attestation struct {
	// Format is the attestation format
	Format string `json:"format"`
	
	// Data is the attestation data
	Data []byte `json:"data"`
	
	// Signature is the attestation signature
	Signature []byte `json:"signature"`
	
	// Certificate is the signing certificate
	Certificate []byte `json:"certificate,omitempty"`
	
	// Bundle contains additional attestation data
	Bundle map[string]interface{} `json:"bundle,omitempty"`
}

// VerificationResult contains attestation verification results.
type VerificationResult struct {
	// Verified indicates if the attestation is valid
	Verified bool `json:"verified"`
	
	// Signer is the attestation signer
	Signer string `json:"signer,omitempty"`
	
	// Timestamp is when the attestation was created
	Timestamp *time.Time `json:"timestamp,omitempty"`
	
	// Errors contains verification errors
	Errors []string `json:"errors,omitempty"`
}
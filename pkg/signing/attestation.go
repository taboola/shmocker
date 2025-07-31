// Package signing provides attestation generation for SLSA compliance.
package signing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// SLSAAttestationGenerator implements AttestationGenerator for SLSA compliance.
type SLSAAttestationGenerator struct {
	signer      Signer
	keyProvider KeyProvider
}

// NewSLSAAttestationGenerator creates a new SLSA attestation generator.
func NewSLSAAttestationGenerator(signer Signer, keyProvider KeyProvider) *SLSAAttestationGenerator {
	return &SLSAAttestationGenerator{
		signer:      signer,
		keyProvider: keyProvider,
	}
}

// GenerateAttestation creates an in-toto attestation for the specified subject and predicate.
func (g *SLSAAttestationGenerator) GenerateAttestation(ctx context.Context, req *AttestationRequest) (*Attestation, error) {
	if req == nil {
		return nil, fmt.Errorf("attestation request cannot be nil")
	}

	if req.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	if req.PredicateType == "" {
		return nil, fmt.Errorf("predicate type cannot be empty")
	}

	if req.Predicate == nil {
		return nil, fmt.Errorf("predicate cannot be nil")
	}

	// Create attestation subject
	subjects := []*AttestationSubject{
		{
			Name: req.Subject,
			Digest: map[string]string{
				"sha256": g.calculateSubjectDigest(req.Subject),
			},
		},
	}

	// Create the attestation
	attestation := &Attestation{
		Type:          "https://in-toto.io/Statement/v0.1",
		Subject:       subjects,
		PredicateType: req.PredicateType,
		Predicate:     req.Predicate,
	}

	// Sign the attestation if key reference is provided
	if req.KeyRef != "" {
		signature, err := g.signAttestation(ctx, attestation, req.KeyRef, req.Options)
		if err != nil {
			return nil, errors.Wrap(err, "failed to sign attestation")
		}
		attestation.Signature = signature
	}

	return attestation, nil
}

// AttachAttestation attaches an attestation to a container image.
func (g *SLSAAttestationGenerator) AttachAttestation(ctx context.Context, imageRef string, attestation *Attestation) error {
	if imageRef == "" {
		return fmt.Errorf("image reference cannot be empty")
	}

	if attestation == nil {
		return fmt.Errorf("attestation cannot be nil")
	}

	// TODO: Implement actual attestation attachment to registry
	// In a real implementation, this would:
	// 1. Serialize the attestation to JSON
	// 2. Create an OCI artifact with the attestation as content
	// 3. Push the artifact to the registry with the appropriate media type
	// 4. Associate it with the image using the subject digest

	return fmt.Errorf("attestation attachment not yet implemented")
}

// GetAttestations retrieves all attestations for a container image.
func (g *SLSAAttestationGenerator) GetAttestations(ctx context.Context, imageRef string) ([]*Attestation, error) {
	if imageRef == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}

	// TODO: Implement actual attestation retrieval from registry
	// In a real implementation, this would:
	// 1. Query the registry for associated artifacts
	// 2. Filter for attestation media types
	// 3. Download and parse attestation content
	// 4. Verify attestation signatures

	// For now, return empty slice
	return []*Attestation{}, nil
}

// SBOMAttestationGenerator creates SBOM-specific attestations.
type SBOMAttestationGenerator struct {
	*SLSAAttestationGenerator
}

// NewSBOMAttestationGenerator creates a new SBOM attestation generator.
func NewSBOMAttestationGenerator(signer Signer, keyProvider KeyProvider) *SBOMAttestationGenerator {
	return &SBOMAttestationGenerator{
		SLSAAttestationGenerator: NewSLSAAttestationGenerator(signer, keyProvider),
	}
}

// GenerateSBOMAttestation creates an attestation for an SBOM.
func (g *SBOMAttestationGenerator) GenerateSBOMAttestation(ctx context.Context, imageRef string, sbomData []byte, keyRef string) (*Attestation, error) {
	if imageRef == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}

	if len(sbomData) == 0 {
		return nil, fmt.Errorf("SBOM data cannot be empty")
	}

	if keyRef == "" {
		return nil, fmt.Errorf("key reference cannot be empty")
	}

	// Create SBOM predicate
	predicate := &SBOMPredicate{
		Format:    "application/spdx+json",
		Content:   sbomData,
		CreatedAt: time.Now(),
		Generator: GeneratorInfo{
			Name:    "shmocker",
			Version: "1.0.0",
		},
	}

	// Create attestation request
	request := &AttestationRequest{
		Subject:       imageRef,
		PredicateType: PredicateTypes.SBOM,
		Predicate:     predicate,
		KeyRef:        keyRef,
		Options: &AttestationOptions{
			Timestamp: true,
		},
	}

	return g.GenerateAttestation(ctx, request)
}

// ProvenanceAttestationGenerator creates SLSA provenance attestations.
type ProvenanceAttestationGenerator struct {
	*SLSAAttestationGenerator
}

// NewProvenanceAttestationGenerator creates a new provenance attestation generator.
func NewProvenanceAttestationGenerator(signer Signer, keyProvider KeyProvider) *ProvenanceAttestationGenerator {
	return &ProvenanceAttestationGenerator{
		SLSAAttestationGenerator: NewSLSAAttestationGenerator(signer, keyProvider),
	}
}

// GenerateProvenanceAttestation creates a provenance attestation for a build.
func (g *ProvenanceAttestationGenerator) GenerateProvenanceAttestation(ctx context.Context, buildInfo *BuildInfo, keyRef string) (*Attestation, error) {
	if buildInfo == nil {
		return nil, fmt.Errorf("build info cannot be nil")
	}

	if keyRef == "" {
		return nil, fmt.Errorf("key reference cannot be empty")
	}

	// Create SLSA provenance predicate
	predicate := &SLSAProvenance{
		Builder: SLSABuilder{
			ID: "https://github.com/shmocker/shmocker",
		},
		BuildType: "https://github.com/shmocker/shmocker/build@v1",
		Invocation: SLSAInvocation{
			ConfigSource: SLSAConfigSource{
				URI:      buildInfo.Source.Repository,
				Digest:   map[string]string{"sha1": buildInfo.Source.Commit},
				EntryPoint: buildInfo.Source.Path,
			},
			Parameters: buildInfo.Parameters,
			Environment: buildInfo.Environment,
		},
		BuildConfig: buildInfo.Config,
		Metadata: SLSAMetadata{
			BuildInvocationID: buildInfo.InvocationID,
			BuildStartedOn:    buildInfo.StartedAt,
			BuildFinishedOn:   buildInfo.FinishedAt,
			Completeness: SLSACompleteness{
				Parameters:  true,
				Environment: true,
				Materials:   true,
			},
			Reproducible: buildInfo.Reproducible,
		},
		Materials: g.convertMaterials(buildInfo.Materials),
	}

	request := &AttestationRequest{
		Subject:       buildInfo.Subject,
		PredicateType: PredicateTypes.SLSA,
		Predicate:     predicate,
		KeyRef:        keyRef,
		Options: &AttestationOptions{
			Timestamp: true,
		},
	}

	return g.GenerateAttestation(ctx, request)
}

// Helper methods

func (g *SLSAAttestationGenerator) calculateSubjectDigest(subject string) string {
	hash := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(hash[:])
}

func (g *SLSAAttestationGenerator) signAttestation(ctx context.Context, attestation *Attestation, keyRef string, opts *AttestationOptions) (*Signature, error) {
	// Serialize attestation for signing
	data, err := json.Marshal(attestation)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal attestation")
	}

	// Sign using the provided signer
	signOpts := &SignOptions{
		KeyRef: keyRef,
	}

	if opts != nil && opts.Timestamp {
		signOpts.Timestamp = true
		if opts.TSAServer != "" {
			signOpts.TSAServer = opts.TSAServer
		}
	}

	signature, err := g.signer.SignBlob(ctx, data, signOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign attestation")
	}

	return signature, nil
}

func (g *ProvenanceAttestationGenerator) convertMaterials(materials []BuildMaterial) []SLSAMaterial {
	result := make([]SLSAMaterial, len(materials))
	for i, material := range materials {
		result[i] = SLSAMaterial{
			URI:    material.URI,
			Digest: material.Digest,
		}
	}
	return result
}

// Predicate types and structures

// SBOMPredicate represents an SBOM attestation predicate.
type SBOMPredicate struct {
	Format    string          `json:"format"`
	Content   []byte          `json:"content"`
	CreatedAt time.Time       `json:"createdAt"`
	Generator GeneratorInfo   `json:"generator"`
}

// GeneratorInfo contains information about the tool that generated the artifact.
type GeneratorInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// SLSAProvenance represents a SLSA provenance attestation predicate.
type SLSAProvenance struct {
	Builder     SLSABuilder     `json:"builder"`
	BuildType   string          `json:"buildType"`
	Invocation  SLSAInvocation  `json:"invocation"`
	BuildConfig interface{}     `json:"buildConfig,omitempty"`
	Metadata    SLSAMetadata    `json:"metadata"`
	Materials   []SLSAMaterial  `json:"materials"`
}

// SLSABuilder represents the builder in SLSA provenance.
type SLSABuilder struct {
	ID string `json:"id"`
}

// SLSAInvocation represents the build invocation in SLSA provenance.
type SLSAInvocation struct {
	ConfigSource SLSAConfigSource `json:"configSource"`
	Parameters   interface{}      `json:"parameters,omitempty"`
	Environment  interface{}      `json:"environment,omitempty"`
}

// SLSAConfigSource represents the configuration source.
type SLSAConfigSource struct {
	URI        string            `json:"uri"`
	Digest     map[string]string `json:"digest"`
	EntryPoint string            `json:"entryPoint,omitempty"`
}

// SLSAMetadata represents metadata about the build.
type SLSAMetadata struct {
	BuildInvocationID string            `json:"buildInvocationId"`
	BuildStartedOn    *time.Time        `json:"buildStartedOn,omitempty"`
	BuildFinishedOn   *time.Time        `json:"buildFinishedOn,omitempty"`
	Completeness      SLSACompleteness  `json:"completeness"`
	Reproducible      bool              `json:"reproducible"`
}

// SLSACompleteness represents the completeness guarantees of SLSA provenance.
type SLSACompleteness struct {
	Parameters  bool `json:"parameters"`
	Environment bool `json:"environment"`
	Materials   bool `json:"materials"`
}

// SLSAMaterial represents a material used in the build.
type SLSAMaterial struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

// BuildInfo contains information about a build for provenance generation.
type BuildInfo struct {
	Subject      string                 `json:"subject"`
	InvocationID string                 `json:"invocationId"`
	StartedAt    *time.Time             `json:"startedAt,omitempty"`
	FinishedAt   *time.Time             `json:"finishedAt,omitempty"`
	Source       BuildSource            `json:"source"`
	Parameters   interface{}            `json:"parameters,omitempty"`
	Environment  interface{}            `json:"environment,omitempty"`
	Config       interface{}            `json:"config,omitempty"`
	Materials    []BuildMaterial        `json:"materials"`
	Reproducible bool                   `json:"reproducible"`
}

// BuildSource represents the source of a build.
type BuildSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Path       string `json:"path,omitempty"`
}

// BuildMaterial represents a material used in a build.
type BuildMaterial struct {
	URI    string            `json:"uri"`
	Digest map[string]string `json:"digest"`
}

// AttestationValidator provides validation for attestations.
type AttestationValidator struct {
	verifier Verifier
}

// NewAttestationValidator creates a new attestation validator.
func NewAttestationValidator(verifier Verifier) *AttestationValidator {
	return &AttestationValidator{
		verifier: verifier,
	}
}

// ValidateAttestation validates an attestation signature and structure.
func (v *AttestationValidator) ValidateAttestation(ctx context.Context, attestation *Attestation, publicKey interface{}) (*AttestationResult, error) {
	if attestation == nil {
		return nil, fmt.Errorf("attestation cannot be nil")
	}

	if publicKey == nil {
		return nil, fmt.Errorf("public key cannot be nil")
	}

	// Validate attestation structure
	if err := v.validateAttestationStructure(attestation); err != nil {
		return &AttestationResult{
			Verified: false,
			Errors:   []string{err.Error()},
		}, nil
	}

	// Verify signature if present
	if attestation.Signature != nil {
		// Serialize attestation for verification (excluding signature)
		tempAttestation := *attestation
		tempAttestation.Signature = nil

		data, err := json.Marshal(&tempAttestation)
		if err != nil {
			return &AttestationResult{
				Verified: false,
				Errors:   []string{fmt.Sprintf("failed to marshal attestation: %v", err)},
			}, nil
		}

		err = v.verifier.VerifyBlob(ctx, data, attestation.Signature, publicKey)
		if err != nil {
			return &AttestationResult{
				Verified: false,
				Errors:   []string{fmt.Sprintf("signature verification failed: %v", err)},
			}, nil
		}
	}

	// Return successful validation result
	result := &AttestationResult{
		Verified:      true,
		Subject:       attestation.Subject,
		PredicateType: attestation.PredicateType,
		Predicate:     attestation.Predicate,
	}

	return result, nil
}

// validateAttestationStructure validates the basic structure of an attestation.
func (v *AttestationValidator) validateAttestationStructure(attestation *Attestation) error {
	if attestation.Type != "https://in-toto.io/Statement/v0.1" {
		return fmt.Errorf("invalid attestation type: %s", attestation.Type)
	}

	if len(attestation.Subject) == 0 {
		return fmt.Errorf("attestation must have at least one subject")
	}

	for i, subject := range attestation.Subject {
		if subject.Name == "" {
			return fmt.Errorf("subject[%d] name cannot be empty", i)
		}

		if len(subject.Digest) == 0 {
			return fmt.Errorf("subject[%d] must have at least one digest", i)
		}
	}

	if attestation.PredicateType == "" {
		return fmt.Errorf("predicate type cannot be empty")
	}

	if attestation.Predicate == nil {
		return fmt.Errorf("predicate cannot be nil")
	}

	return nil
}

// PolicyEnforcer enforces signing policies for attestations.
type PolicyEnforcer struct {
	verifier Verifier
}

// NewPolicyEnforcer creates a new policy enforcer.
func NewPolicyEnforcer(verifier Verifier) *PolicyEnforcer {
	return &PolicyEnforcer{
		verifier: verifier,
	}
}

// EnforcePolicy enforces a signing policy against attestations.
func (e *PolicyEnforcer) EnforcePolicy(ctx context.Context, imageRef string, policy *AttestationPolicy) (*PolicyEnforcementResult, error) {
	if imageRef == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}

	if policy == nil {
		return nil, fmt.Errorf("policy cannot be nil")
	}

	// TODO: Implement policy enforcement
	// In a real implementation, this would:
	// 1. Retrieve attestations for the image
	// 2. Validate required attestation types are present
	// 3. Verify signatures against allowed signers
	// 4. Check attestation content against policy rules
	// 5. Return enforcement result

	return &PolicyEnforcementResult{
		Allowed: true,
		Reason:  "policy enforcement not yet implemented",
	}, nil
}

// AttestationPolicy represents a policy for attestation validation.
type AttestationPolicy struct {
	RequiredAttestations []string            `json:"requiredAttestations"`
	AllowedSigners       []string            `json:"allowedSigners"`
	Rules                []AttestationRule   `json:"rules"`
}

// AttestationRule represents a rule for attestation validation.
type AttestationRule struct {
	PredicateType string      `json:"predicateType"`
	Conditions    []Condition `json:"conditions"`
}

// Condition represents a condition for attestation validation.
type Condition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// PolicyEnforcementResult represents the result of policy enforcement.
type PolicyEnforcementResult struct {
	Allowed bool     `json:"allowed"`
	Reason  string   `json:"reason,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}
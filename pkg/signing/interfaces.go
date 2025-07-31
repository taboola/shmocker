// Package signing defines interfaces for container image signing and verification.
package signing

import (
	"context"
	"crypto"
	"io"
	"time"
)

// Signer provides the main interface for signing container images and artifacts.
type Signer interface {
	// Sign signs a container image with the specified key
	Sign(ctx context.Context, req *SignRequest) (*SignResult, error)
	
	// SignBlob signs arbitrary blob data
	SignBlob(ctx context.Context, data []byte, opts *SignOptions) (*Signature, error)
	
	// GenerateKeyPair generates a new signing key pair
	GenerateKeyPair(ctx context.Context, opts *KeyGenOptions) (*KeyPair, error)
	
	// GetPublicKey returns the public key for verification
	GetPublicKey(ctx context.Context, keyRef string) (crypto.PublicKey, error)
}

// Verifier provides the interface for verifying signed images and artifacts.
type Verifier interface {
	// Verify verifies a container image signature
	Verify(ctx context.Context, req *VerifyRequest) (*VerifyResult, error)
	
	// VerifyBlob verifies a blob signature
	VerifyBlob(ctx context.Context, data []byte, sig *Signature, key crypto.PublicKey) error
	
	// VerifyAttestation verifies an in-toto attestation
	VerifyAttestation(ctx context.Context, attestation *Attestation, key crypto.PublicKey) (*AttestationResult, error)
	
	// GetSignatures retrieves all signatures for an image
	GetSignatures(ctx context.Context, imageRef string) ([]*Signature, error)
}

// KeyProvider provides access to signing keys from various sources.
type KeyProvider interface {
	// GetPrivateKey retrieves a private key
	GetPrivateKey(ctx context.Context, keyRef string) (crypto.PrivateKey, error)
	
	// GetPublicKey retrieves a public key
	GetPublicKey(ctx context.Context, keyRef string) (crypto.PublicKey, error)
	
	// ListKeys lists available keys
	ListKeys(ctx context.Context) ([]*KeyInfo, error)
	
	// StoreKey stores a key
	StoreKey(ctx context.Context, keyRef string, key crypto.PrivateKey) error
	
	// DeleteKey deletes a key
	DeleteKey(ctx context.Context, keyRef string) error
}

// AttestationGenerator creates in-toto attestations for images and SBOMs.
type AttestationGenerator interface {
	// GenerateAttestation creates an in-toto attestation
	GenerateAttestation(ctx context.Context, req *AttestationRequest) (*Attestation, error)
	
	// AttachAttestation attaches an attestation to an image
	AttachAttestation(ctx context.Context, imageRef string, attestation *Attestation) error
	
	// GetAttestations retrieves attestations for an image
	GetAttestations(ctx context.Context, imageRef string) ([]*Attestation, error)
}

// PolicyVerifier verifies images against signing policies.
type PolicyVerifier interface {
	// VerifyPolicy verifies an image against a signing policy
	VerifyPolicy(ctx context.Context, imageRef string, policy *Policy) (*PolicyResult, error)
	
	// LoadPolicy loads a policy from various sources
	LoadPolicy(ctx context.Context, source string) (*Policy, error)
	
	// ValidatePolicy validates a policy
	ValidatePolicy(policy *Policy) error
}

// CertificateAuthority provides certificate management for signing.
type CertificateAuthority interface {
	// IssueCertificate issues a new certificate
	IssueCertificate(ctx context.Context, req *CertificateRequest) (*Certificate, error)
	
	// RevokeCertificate revokes a certificate
	RevokeCertificate(ctx context.Context, serialNumber string) error
	
	// VerifyCertificate verifies a certificate
	VerifyCertificate(ctx context.Context, cert *Certificate) (*CertificateResult, error)
	
	// GetCRL returns the certificate revocation list
	GetCRL(ctx context.Context) (*CRL, error)
}

// SignRequest represents a request to sign an image.
type SignRequest struct {
	// ImageRef is the image reference to sign
	ImageRef string `json:"image_ref"`
	
	// ImageDigest is the specific image digest to sign
	ImageDigest string `json:"image_digest"`
	
	// KeyRef is the reference to the signing key
	KeyRef string `json:"key_ref,omitempty"`
	
	// Options contains signing options
	Options *SignOptions `json:"options,omitempty"`
	
	// Auth provides registry authentication
	Auth *RegistryAuth `json:"auth,omitempty"`
	
	// Annotations contains arbitrary annotations to include
	Annotations map[string]string `json:"annotations,omitempty"`
}

// SignOptions contains options for signing operations.
type SignOptions struct {
	// KeyType specifies the key type to use
	KeyType KeyType `json:"key_type,omitempty"`
	
	// Algorithm specifies the signing algorithm
	Algorithm SigningAlgorithm `json:"algorithm,omitempty"`
	
	// Recursive signs all images in a multi-arch manifest
	Recursive bool `json:"recursive,omitempty"`
	
	// Attestations contains attestations to include
	Attestations []*Attestation `json:"attestations,omitempty"`
	
	// Certificate contains the signing certificate
	Certificate *Certificate `json:"certificate,omitempty"`
	
	// CertificateChain contains the certificate chain
	CertificateChain []*Certificate `json:"certificate_chain,omitempty"`
	
	// Timestamp enables timestamping
	Timestamp bool `json:"timestamp,omitempty"`
	
	// TSAServer is the timestamp authority server
	TSAServer string `json:"tsa_server,omitempty"`
	
	// FulcioURL is the Fulcio CA URL
	FulcioURL string `json:"fulcio_url,omitempty"`
	
	// RekorURL is the Rekor transparency log URL
	RekorURL string `json:"rekor_url,omitempty"`
}

// SignResult contains the result of a signing operation.
type SignResult struct {
	// ImageRef is the signed image reference
	ImageRef string `json:"image_ref"`
	
	// ImageDigest is the signed image digest
	ImageDigest string `json:"image_digest"`
	
	// Signature contains the image signature
	Signature *Signature `json:"signature"`
	
	// Attestations contains generated attestations
	Attestations []*Attestation `json:"attestations,omitempty"`
	
	// Certificate contains the signing certificate
	Certificate *Certificate `json:"certificate,omitempty"`
	
	// TransparencyLog contains transparency log entries
	TransparencyLog []*LogEntry `json:"transparency_log,omitempty"`
	
	// Bundle contains the complete signature bundle
	Bundle *SignatureBundle `json:"bundle,omitempty"`
}

// VerifyRequest represents a request to verify a signature.
type VerifyRequest struct {
	// ImageRef is the image reference to verify
	ImageRef string `json:"image_ref"`
	
	// ImageDigest is the specific image digest to verify
	ImageDigest string `json:"image_digest,omitempty"`
	
	// KeyRef is the reference to the verification key
	KeyRef string `json:"key_ref,omitempty"`
	
	// PublicKey is the public key for verification
	PublicKey crypto.PublicKey `json:"-"`
	
	// Options contains verification options
	Options *VerifyOptions `json:"options,omitempty"`
	
	// Auth provides registry authentication
	Auth *RegistryAuth `json:"auth,omitempty"`
}

// VerifyOptions contains options for verification operations.
type VerifyOptions struct {
	// RequireCertificate requires a valid certificate
	RequireCertificate bool `json:"require_certificate,omitempty"`
	
	// RequireTransparencyLog requires transparency log verification
	RequireTransparencyLog bool `json:"require_transparency_log,omitempty"`
	
	// CertificateIdentity specifies required certificate identity
	CertificateIdentity string `json:"certificate_identity,omitempty"`
	
	// CertificateOIDCIssuer specifies required OIDC issuer
	CertificateOIDCIssuer string `json:"certificate_oidc_issuer,omitempty"`
	
	// AnnotationsVerifier verifies annotations
	AnnotationsVerifier func(map[string]string) error `json:"-"`
	
	// RekorURL is the Rekor transparency log URL
	RekorURL string `json:"rekor_url,omitempty"`
	
	// MaxAge is the maximum signature age
	MaxAge time.Duration `json:"max_age,omitempty"`
}

// VerifyResult contains the result of a verification operation.
type VerifyResult struct {
	// Verified indicates if the signature is valid
	Verified bool `json:"verified"`
	
	// Signatures contains verified signatures
	Signatures []*Signature `json:"signatures"`
	
	// Attestations contains verified attestations
	Attestations []*Attestation `json:"attestations,omitempty"`
	
	// Certificate contains the signing certificate
	Certificate *Certificate `json:"certificate,omitempty"`
	
	// CertificateChain contains the certificate chain
	CertificateChain []*Certificate `json:"certificate_chain,omitempty"`
	
	// TransparencyLog contains transparency log verification
	TransparencyLog []*LogEntry `json:"transparency_log,omitempty"`
	
	// Errors contains verification errors
	Errors []string `json:"errors,omitempty"`
	
	// Warnings contains verification warnings
	Warnings []string `json:"warnings,omitempty"`
}

// Signature represents a digital signature.
type Signature struct {
	// KeyID is the key identifier
	KeyID string `json:"key_id,omitempty"`
	
	// Algorithm is the signing algorithm
	Algorithm SigningAlgorithm `json:"algorithm"`
	
	// Signature is the signature bytes
	Signature []byte `json:"signature"`
	
	// Payload is the signed payload
	Payload []byte `json:"payload,omitempty"`
	
	// MediaType is the payload media type
	MediaType string `json:"media_type,omitempty"`
	
	// Annotations contains signature annotations
	Annotations map[string]string `json:"annotations,omitempty"`
	
	// Bundle contains the signature bundle
	Bundle *SignatureBundle `json:"bundle,omitempty"`
}

// KeyType represents the type of cryptographic key.
type KeyType string

const (
	KeyTypeRSA     KeyType = "rsa"
	KeyTypeECDSA   KeyType = "ecdsa"
	KeyTypeEd25519 KeyType = "ed25519"
)

// SigningAlgorithm represents a signing algorithm.
type SigningAlgorithm string

const (
	AlgorithmRSAPSS    SigningAlgorithm = "rsa-pss"
	AlgorithmRSAPKCS1  SigningAlgorithm = "rsa-pkcs1"
	AlgorithmECDSAP256 SigningAlgorithm = "ecdsa-p256"
	AlgorithmECDSAP384 SigningAlgorithm = "ecdsa-p384"
	AlgorithmEd25519   SigningAlgorithm = "ed25519"
)

// KeyPair represents a cryptographic key pair.
type KeyPair struct {
	// PrivateKey is the private key
	PrivateKey crypto.PrivateKey `json:"-"`
	
	// PublicKey is the public key
	PublicKey crypto.PublicKey `json:"-"`
	
	// KeyID is the key identifier
	KeyID string `json:"key_id"`
	
	// KeyType is the key type
	KeyType KeyType `json:"key_type"`
	
	// Algorithm is the signing algorithm
	Algorithm SigningAlgorithm `json:"algorithm"`
	
	// CreatedAt is when the key was created
	CreatedAt time.Time `json:"created_at"`
}

// KeyInfo provides information about a key.
type KeyInfo struct {
	// KeyID is the key identifier
	KeyID string `json:"key_id"`
	
	// KeyType is the key type
	KeyType KeyType `json:"key_type"`
	
	// Algorithm is the signing algorithm
	Algorithm SigningAlgorithm `json:"algorithm"`
	
	// PublicKey is the public key (for display)
	PublicKey string `json:"public_key,omitempty"`
	
	// CreatedAt is when the key was created
	CreatedAt time.Time `json:"created_at"`
	
	// ExpiresAt is when the key expires
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	
	// Description is a human-readable description
	Description string `json:"description,omitempty"`
}

// KeyGenOptions contains options for key generation.
type KeyGenOptions struct {
	// KeyType specifies the key type
	KeyType KeyType `json:"key_type"`
	
	// Algorithm specifies the signing algorithm
	Algorithm SigningAlgorithm `json:"algorithm"`
	
	// KeySize specifies the key size (for RSA)
	KeySize int `json:"key_size,omitempty"`
	
	// Curve specifies the elliptic curve (for ECDSA)
	Curve string `json:"curve,omitempty"`
	
	// Description is a human-readable description
	Description string `json:"description,omitempty"`
	
	// ExpiresAt is when the key should expire
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AttestationRequest represents a request to generate an attestation.
type AttestationRequest struct {
	// Subject is the attestation subject
	Subject string `json:"subject"`
	
	// PredicateType is the predicate type
	PredicateType string `json:"predicate_type"`
	
	// Predicate is the attestation predicate
	Predicate interface{} `json:"predicate"`
	
	// KeyRef is the reference to the signing key
	KeyRef string `json:"key_ref,omitempty"`
	
	// Options contains attestation options
	Options *AttestationOptions `json:"options,omitempty"`
}

// AttestationOptions contains options for attestation generation.
type AttestationOptions struct {
	// Timestamp enables timestamping
	Timestamp bool `json:"timestamp,omitempty"`
	
	// TSAServer is the timestamp authority server
	TSAServer string `json:"tsa_server,omitempty"`
	
	// Certificate contains the signing certificate
	Certificate *Certificate `json:"certificate,omitempty"`
	
	// CertificateChain contains the certificate chain
	CertificateChain []*Certificate `json:"certificate_chain,omitempty"`
}

// Attestation represents an in-toto attestation.
type Attestation struct {
	// Type is the attestation type
	Type string `json:"_type"`
	
	// Subject contains the attestation subjects
	Subject []*AttestationSubject `json:"subject"`
	
	// PredicateType is the predicate type
	PredicateType string `json:"predicateType"`
	
	// Predicate contains the attestation predicate
	Predicate interface{} `json:"predicate"`
	
	// Signature contains the attestation signature
	Signature *Signature `json:"signature,omitempty"`
}

// AttestationSubject represents an attestation subject.
type AttestationSubject struct {
	// Name is the subject name
	Name string `json:"name"`
	
	// Digest contains the subject digests
	Digest map[string]string `json:"digest"`
}

// AttestationResult contains the result of an attestation verification.
type AttestationResult struct {
	// Verified indicates if the attestation is valid
	Verified bool `json:"verified"`
	
	// Subject contains the verified subjects
	Subject []*AttestationSubject `json:"subject"`
	
	// PredicateType is the predicate type
	PredicateType string `json:"predicate_type"`
	
	// Predicate contains the verified predicate
	Predicate interface{} `json:"predicate"`
	
	// Certificate contains the signing certificate
	Certificate *Certificate `json:"certificate,omitempty"`
	
	// Errors contains verification errors
	Errors []string `json:"errors,omitempty"`
}

// Policy represents a signing policy.
type Policy struct {
	// Version is the policy version
	Version string `json:"version"`
	
	// Rules contains policy rules
	Rules []*PolicyRule `json:"rules"`
	
	// Default is the default policy action
	Default PolicyAction `json:"default"`
	
	// Metadata contains policy metadata
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PolicyRule represents a single policy rule.
type PolicyRule struct {
	// Pattern is the image pattern to match
	Pattern string `json:"pattern"`
	
	// Action is the action to take
	Action PolicyAction `json:"action"`
	
	// Requirements contains signing requirements
	Requirements *PolicyRequirements `json:"requirements,omitempty"`
	
	// Description is a human-readable description
	Description string `json:"description,omitempty"`
}

// PolicyAction represents a policy action.
type PolicyAction string

const (
	PolicyActionAllow  PolicyAction = "allow"
	PolicyActionDeny   PolicyAction = "deny"
	PolicyActionVerify PolicyAction = "verify"
)

// PolicyRequirements contains signing requirements.
type PolicyRequirements struct {
	// RequireSignature requires a valid signature
	RequireSignature bool `json:"require_signature,omitempty"`
	
	// RequireCertificate requires a valid certificate
	RequireCertificate bool `json:"require_certificate,omitempty"`
	
	// RequireAttestation requires specific attestations
	RequireAttestation []string `json:"require_attestation,omitempty"`
	
	// AllowedSigners contains allowed signer identities
	AllowedSigners []string `json:"allowed_signers,omitempty"`
	
	// AllowedIssuers contains allowed certificate issuers
	AllowedIssuers []string `json:"allowed_issuers,omitempty"`
	
	// MaxAge is the maximum signature age
	MaxAge time.Duration `json:"max_age,omitempty"`
}

// PolicyResult contains the result of a policy verification.
type PolicyResult struct {
	// Allowed indicates if the image is allowed
	Allowed bool `json:"allowed"`
	
	// Action is the policy action taken
	Action PolicyAction `json:"action"`
	
	// Rule is the matching rule
	Rule *PolicyRule `json:"rule,omitempty"`
	
	// Reason is the reason for the decision
	Reason string `json:"reason,omitempty"`
	
	// Errors contains policy errors
	Errors []string `json:"errors,omitempty"`
}

// Certificate represents an X.509 certificate.
type Certificate struct {
	// Raw is the raw certificate bytes
	Raw []byte `json:"raw"`
	
	// Subject is the certificate subject
	Subject string `json:"subject"`
	
	// Issuer is the certificate issuer
	Issuer string `json:"issuer"`
	
	// SerialNumber is the certificate serial number
	SerialNumber string `json:"serial_number"`
	
	// NotBefore is when the certificate becomes valid
	NotBefore time.Time `json:"not_before"`
	
	// NotAfter is when the certificate expires
	NotAfter time.Time `json:"not_after"`
	
	// PublicKey is the certificate public key
	PublicKey crypto.PublicKey `json:"-"`
	
	// Extensions contains certificate extensions
	Extensions map[string]string `json:"extensions,omitempty"`
}

// CertificateRequest represents a certificate signing request.
type CertificateRequest struct {
	// Subject is the certificate subject
	Subject string `json:"subject"`
	
	// PublicKey is the public key to certify
	PublicKey crypto.PublicKey `json:"-"`
	
	// Extensions contains requested extensions
	Extensions map[string]string `json:"extensions,omitempty"`
	
	// ValidityPeriod is the requested validity period
	ValidityPeriod time.Duration `json:"validity_period,omitempty"`
}

// CertificateResult contains certificate verification results.
type CertificateResult struct {
	// Valid indicates if the certificate is valid
	Valid bool `json:"valid"`
	
	// Chain contains the verified certificate chain
	Chain []*Certificate `json:"chain,omitempty"`
	
	// Revoked indicates if the certificate is revoked
	Revoked bool `json:"revoked,omitempty"`
	
	// Errors contains verification errors
	Errors []string `json:"errors,omitempty"`
}

// CRL represents a certificate revocation list.
type CRL struct {
	// Issuer is the CRL issuer
	Issuer string `json:"issuer"`
	
	// ThisUpdate is when the CRL was issued
	ThisUpdate time.Time `json:"this_update"`
	
	// NextUpdate is when the next CRL will be issued
	NextUpdate time.Time `json:"next_update"`
	
	// RevokedCertificates contains revoked certificates
	RevokedCertificates []*RevokedCertificate `json:"revoked_certificates"`
}

// RevokedCertificate represents a revoked certificate.
type RevokedCertificate struct {
	// SerialNumber is the revoked certificate serial number
	SerialNumber string `json:"serial_number"`
	
	// RevocationTime is when the certificate was revoked
	RevocationTime time.Time `json:"revocation_time"`
	
	// Reason is the revocation reason
	Reason string `json:"reason,omitempty"`
}

// LogEntry represents a transparency log entry.
type LogEntry struct {
	// LogID is the log identifier
	LogID string `json:"log_id"`
	
	// Index is the entry index
	Index int64 `json:"index"`
	
	// IntegratedTime is when the entry was integrated
	IntegratedTime time.Time `json:"integrated_time"`
	
	// Body is the log entry body
	Body []byte `json:"body"`
	
	// Verification contains verification information
	Verification *LogVerification `json:"verification,omitempty"`
}

// LogVerification contains transparency log verification information.
type LogVerification struct {
	// SignedEntryTimestamp is the signed entry timestamp
	SignedEntryTimestamp []byte `json:"signed_entry_timestamp"`
	
	// InclusionProof is the inclusion proof
	InclusionProof *InclusionProof `json:"inclusion_proof,omitempty"`
}

// InclusionProof represents a Merkle tree inclusion proof.
type InclusionProof struct {
	// LogIndex is the log index
	LogIndex int64 `json:"log_index"`
	
	// RootHash is the root hash
	RootHash []byte `json:"root_hash"`
	
	// TreeSize is the tree size
	TreeSize int64 `json:"tree_size"`
	
	// Hashes contains the proof hashes
	Hashes [][]byte `json:"hashes"`
}

// SignatureBundle contains a complete signature bundle.
type SignatureBundle struct {
	// MediaType is the bundle media type
	MediaType string `json:"media_type"`
	
	// Content is the bundle content
	Content []byte `json:"content"`
	
	// Signature contains the bundle signature
	Signature *Signature `json:"signature,omitempty"`
	
	// Certificate contains the signing certificate
	Certificate *Certificate `json:"certificate,omitempty"`
	
	// TransparencyLog contains transparency log entries
	TransparencyLog []*LogEntry `json:"transparency_log,omitempty"`
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

// Common predicate types for attestations
var PredicateTypes = struct {
	SLSA        string
	SBOM        string
	Provenance  string
	Vulnerability string
}{
	SLSA:        "https://slsa.dev/provenance/v0.2",
	SBOM:        "https://spdx.dev/Document",
	Provenance:  "https://in-toto.io/Statement/v0.1",
	Vulnerability: "https://in-toto.io/attestation/vuln/v0.1",
}
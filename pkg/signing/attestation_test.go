// Package signing provides attestation generation for SLSA compliance.
package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSLSAAttestationGenerator_GenerateAttestation(t *testing.T) {
	// Create mock signer and key provider
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	generator := NewSLSAAttestationGenerator(signer, keyProvider)

	// Generate test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	keyRef := "test-key"
	if err := keyProvider.StoreKey(context.Background(), keyRef, privateKey); err != nil {
		t.Fatalf("Failed to store test key: %v", err)
	}

	// Test predicate
	testPredicate := map[string]interface{}{
		"test": "predicate",
		"data": 123,
	}

	tests := []struct {
		name    string
		req     *AttestationRequest
		wantErr bool
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "empty subject",
			req: &AttestationRequest{
				Subject: "",
			},
			wantErr: true,
		},
		{
			name: "empty predicate type",
			req: &AttestationRequest{
				Subject:       "test-subject",
				PredicateType: "",
			},
			wantErr: true,
		},
		{
			name: "nil predicate",
			req: &AttestationRequest{
				Subject:       "test-subject",
				PredicateType: "test-predicate-type",
				Predicate:     nil,
			},
			wantErr: true,
		},
		{
			name: "valid request without signing",
			req: &AttestationRequest{
				Subject:       "test-subject",
				PredicateType: "test-predicate-type",
				Predicate:     testPredicate,
			},
			wantErr: false,
		},
		{
			name: "valid request with signing",
			req: &AttestationRequest{
				Subject:       "test-subject",
				PredicateType: "test-predicate-type",
				Predicate:     testPredicate,
				KeyRef:        keyRef,
				Options: &AttestationOptions{
					Timestamp: true,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			attestation, err := generator.GenerateAttestation(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SLSAAttestationGenerator.GenerateAttestation() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SLSAAttestationGenerator.GenerateAttestation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if attestation == nil {
				t.Errorf("SLSAAttestationGenerator.GenerateAttestation() returned nil attestation")
				return
			}

			// Validate attestation structure
			if attestation.Type != "https://in-toto.io/Statement/v0.1" {
				t.Errorf("SLSAAttestationGenerator.GenerateAttestation() Type = %v, want %v", 
					attestation.Type, "https://in-toto.io/Statement/v0.1")
			}

			if len(attestation.Subject) != 1 {
				t.Errorf("SLSAAttestationGenerator.GenerateAttestation() Subject count = %d, want 1", 
					len(attestation.Subject))
			}

			if len(attestation.Subject) > 0 {
				if attestation.Subject[0].Name != tt.req.Subject {
					t.Errorf("SLSAAttestationGenerator.GenerateAttestation() Subject[0].Name = %v, want %v", 
						attestation.Subject[0].Name, tt.req.Subject)
				}

				if len(attestation.Subject[0].Digest) == 0 {
					t.Errorf("SLSAAttestationGenerator.GenerateAttestation() Subject[0].Digest is empty")
				}
			}

			if attestation.PredicateType != tt.req.PredicateType {
				t.Errorf("SLSAAttestationGenerator.GenerateAttestation() PredicateType = %v, want %v", 
					attestation.PredicateType, tt.req.PredicateType)
			}

			// Check if signature is present when key is provided
			if tt.req.KeyRef != "" {
				if attestation.Signature == nil {
					t.Errorf("SLSAAttestationGenerator.GenerateAttestation() Signature is nil when key provided")
				}
			} else {
				if attestation.Signature != nil {
					t.Errorf("SLSAAttestationGenerator.GenerateAttestation() Signature is not nil when no key provided")
				}
			}
		})
	}
}

func TestSBOMAttestationGenerator_GenerateSBOMAttestation(t *testing.T) {
	// Create mock signer and key provider
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	generator := NewSBOMAttestationGenerator(signer, keyProvider)

	// Generate test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	keyRef := "test-key"
	if err := keyProvider.StoreKey(context.Background(), keyRef, privateKey); err != nil {
		t.Fatalf("Failed to store test key: %v", err)
	}

	testSBOMData := []byte(`{"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0"}`)

	tests := []struct {
		name     string
		imageRef string
		sbomData []byte
		keyRef   string
		wantErr  bool
	}{
		{
			name:     "empty image reference",
			imageRef: "",
			sbomData: testSBOMData,
			keyRef:   keyRef,
			wantErr:  true,
		},
		{
			name:     "empty SBOM data",
			imageRef: "alpine:latest",
			sbomData: []byte{},
			keyRef:   keyRef,
			wantErr:  true,
		},
		{
			name:     "empty key reference",
			imageRef: "alpine:latest",
			sbomData: testSBOMData,
			keyRef:   "",
			wantErr:  true,
		},
		{
			name:     "valid SBOM attestation",
			imageRef: "alpine:latest",
			sbomData: testSBOMData,
			keyRef:   keyRef,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			attestation, err := generator.GenerateSBOMAttestation(ctx, tt.imageRef, tt.sbomData, tt.keyRef)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if attestation == nil {
				t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() returned nil attestation")
				return
			}

			// Validate SBOM-specific fields
			if attestation.PredicateType != PredicateTypes.SBOM {
				t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() PredicateType = %v, want %v", 
					attestation.PredicateType, PredicateTypes.SBOM)
			}

			// Validate predicate structure
			predicateBytes, err := json.Marshal(attestation.Predicate)
			if err != nil {
				t.Errorf("Failed to marshal predicate: %v", err)
				return
			}

			var predicate SBOMPredicate
			if err := json.Unmarshal(predicateBytes, &predicate); err != nil {
				t.Errorf("Failed to unmarshal SBOM predicate: %v", err)
				return
			}

			if predicate.Format != "application/spdx+json" {
				t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() Format = %v, want %v", 
					predicate.Format, "application/spdx+json")
			}

			if len(predicate.Content) != len(tt.sbomData) {
				t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() Content length = %d, want %d", 
					len(predicate.Content), len(tt.sbomData))
			}

			if predicate.Generator.Name != "shmocker" {
				t.Errorf("SBOMAttestationGenerator.GenerateSBOMAttestation() Generator.Name = %v, want %v", 
					predicate.Generator.Name, "shmocker")
			}
		})
	}
}

func TestProvenanceAttestationGenerator_GenerateProvenanceAttestation(t *testing.T) {
	// Create mock signer and key provider
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	generator := NewProvenanceAttestationGenerator(signer, keyProvider)

	// Generate test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	keyRef := "test-key"
	if err := keyProvider.StoreKey(context.Background(), keyRef, privateKey); err != nil {
		t.Fatalf("Failed to store test key: %v", err)
	}

	startTime := time.Now().Add(-time.Hour)
	endTime := time.Now()

	testBuildInfo := &BuildInfo{
		Subject:      "myapp:latest",
		InvocationID: uuid.New().String(),
		StartedAt:    &startTime,
		FinishedAt:   &endTime,
		Source: BuildSource{
			Repository: "https://github.com/user/repo",
			Commit:     "abc123def456",
			Path:       "Dockerfile",
		},
		Parameters: map[string]interface{}{
			"target": "production",
		},
		Environment: map[string]interface{}{
			"GOOS":   "linux",
			"GOARCH": "amd64",
		},
		Config: map[string]interface{}{
			"dockerfile": "Dockerfile",
		},
		Materials: []BuildMaterial{
			{
				URI: "git+https://github.com/user/repo@refs/heads/main",
				Digest: map[string]string{
					"sha1": "abc123def456",
				},
			},
		},
		Reproducible: true,
	}

	tests := []struct {
		name      string
		buildInfo *BuildInfo
		keyRef    string
		wantErr   bool
	}{
		{
			name:      "nil build info",
			buildInfo: nil,
			keyRef:    keyRef,
			wantErr:   true,
		},
		{
			name:      "empty key reference",
			buildInfo: testBuildInfo,
			keyRef:    "",
			wantErr:   true,
		},
		{
			name:      "valid provenance attestation",
			buildInfo: testBuildInfo,
			keyRef:    keyRef,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			attestation, err := generator.GenerateProvenanceAttestation(ctx, tt.buildInfo, tt.keyRef)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if attestation == nil {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() returned nil attestation")
				return
			}

			// Validate provenance-specific fields
			if attestation.PredicateType != PredicateTypes.SLSA {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() PredicateType = %v, want %v", 
					attestation.PredicateType, PredicateTypes.SLSA)
			}

			// Validate predicate structure
			predicateBytes, err := json.Marshal(attestation.Predicate)
			if err != nil {
				t.Errorf("Failed to marshal predicate: %v", err)
				return
			}

			var predicate SLSAProvenance
			if err := json.Unmarshal(predicateBytes, &predicate); err != nil {
				t.Errorf("Failed to unmarshal SLSA provenance predicate: %v", err)
				return
			}

			if predicate.Builder.ID != "https://github.com/shmocker/shmocker" {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() Builder.ID = %v, want %v", 
					predicate.Builder.ID, "https://github.com/shmocker/shmocker")
			}

			if predicate.BuildType != "https://github.com/shmocker/shmocker/build@v1" {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() BuildType = %v, want %v", 
					predicate.BuildType, "https://github.com/shmocker/shmocker/build@v1")
			}

			if predicate.Invocation.ConfigSource.URI != tt.buildInfo.Source.Repository {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() ConfigSource.URI = %v, want %v", 
					predicate.Invocation.ConfigSource.URI, tt.buildInfo.Source.Repository)
			}

			if predicate.Metadata.Reproducible != tt.buildInfo.Reproducible {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() Reproducible = %v, want %v", 
					predicate.Metadata.Reproducible, tt.buildInfo.Reproducible)
			}

			if len(predicate.Materials) != len(tt.buildInfo.Materials) {
				t.Errorf("ProvenanceAttestationGenerator.GenerateProvenanceAttestation() Materials count = %d, want %d", 
					len(predicate.Materials), len(tt.buildInfo.Materials))
			}
		})
	}
}

func TestAttestationValidator_ValidateAttestation(t *testing.T) {
	// Create mock verifier
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	verifier := NewCosignVerifier(keyProvider, nil)

	validator := NewAttestationValidator(verifier)

	// Generate test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// Create valid test attestation
	validAttestation := &Attestation{
		Type: "https://in-toto.io/Statement/v0.1",
		Subject: []*AttestationSubject{
			{
				Name: "test-subject",
				Digest: map[string]string{
					"sha256": "1234567890abcdef",
				},
			},
		},
		PredicateType: "test-predicate-type",
		Predicate:     map[string]interface{}{"test": "data"},
	}

	// Create attestation with invalid type
	invalidTypeAttestation := &Attestation{
		Type: "invalid-type",
		Subject: []*AttestationSubject{
			{
				Name: "test-subject",
				Digest: map[string]string{
					"sha256": "1234567890abcdef",
				},
			},
		},
		PredicateType: "test-predicate-type",
		Predicate:     map[string]interface{}{"test": "data"},
	}

	// Create attestation with empty subjects
	emptySubjectsAttestation := &Attestation{
		Type:          "https://in-toto.io/Statement/v0.1",
		Subject:       []*AttestationSubject{},
		PredicateType: "test-predicate-type",
		Predicate:     map[string]interface{}{"test": "data"},
	}

	tests := []struct {
		name        string
		attestation *Attestation
		publicKey   interface{}
		wantErr     bool
		wantValid   bool
	}{
		{
			name:        "nil attestation",
			attestation: nil,
			publicKey:   &privateKey.PublicKey,
			wantErr:     true,
		},
		{
			name:        "nil public key",
			attestation: validAttestation,
			publicKey:   nil,
			wantErr:     true,
		},
		{
			name:        "invalid attestation type",
			attestation: invalidTypeAttestation,
			publicKey:   &privateKey.PublicKey,
			wantErr:     false,
			wantValid:   false,
		},
		{
			name:        "empty subjects",
			attestation: emptySubjectsAttestation,
			publicKey:   &privateKey.PublicKey,
			wantErr:     false,
			wantValid:   false,
		},
		{
			name:        "valid attestation",
			attestation: validAttestation,
			publicKey:   &privateKey.PublicKey,
			wantErr:     false,
			wantValid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := validator.ValidateAttestation(ctx, tt.attestation, tt.publicKey)

			if tt.wantErr {
				if err == nil {
					t.Errorf("AttestationValidator.ValidateAttestation() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("AttestationValidator.ValidateAttestation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Errorf("AttestationValidator.ValidateAttestation() returned nil result")
				return
			}

			if result.Verified != tt.wantValid {
				t.Errorf("AttestationValidator.ValidateAttestation() Verified = %v, want %v", result.Verified, tt.wantValid)
			}

			if tt.wantValid {
				if len(result.Subject) != len(tt.attestation.Subject) {
					t.Errorf("AttestationValidator.ValidateAttestation() Subject count = %d, want %d", 
						len(result.Subject), len(tt.attestation.Subject))
				}

				if result.PredicateType != tt.attestation.PredicateType {
					t.Errorf("AttestationValidator.ValidateAttestation() PredicateType = %v, want %v", 
						result.PredicateType, tt.attestation.PredicateType)
				}
			} else {
				if len(result.Errors) == 0 {
					t.Errorf("AttestationValidator.ValidateAttestation() expected errors for invalid attestation")
				}
			}
		})
	}
}

func TestSLSAAttestationGenerator_AttachAttestation(t *testing.T) {
	generator := NewSLSAAttestationGenerator(nil, nil)

	attestation := &Attestation{
		Type: "https://in-toto.io/Statement/v0.1",
		Subject: []*AttestationSubject{
			{
				Name: "test-subject",
				Digest: map[string]string{
					"sha256": "1234567890abcdef",
				},
			},
		},
		PredicateType: "test-predicate-type",
		Predicate:     map[string]interface{}{"test": "data"},
	}

	tests := []struct {
		name        string
		imageRef    string
		attestation *Attestation
		wantErr     bool
	}{
		{
			name:        "empty image reference",
			imageRef:    "",
			attestation: attestation,
			wantErr:     true,
		},
		{
			name:        "nil attestation",
			imageRef:    "alpine:latest",
			attestation: nil,
			wantErr:     true,
		},
		{
			name:        "valid request - not implemented",
			imageRef:    "alpine:latest",
			attestation: attestation,
			wantErr:     true, // Currently not implemented
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := generator.AttachAttestation(ctx, tt.imageRef, tt.attestation)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SLSAAttestationGenerator.AttachAttestation() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SLSAAttestationGenerator.AttachAttestation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSLSAAttestationGenerator_GetAttestations(t *testing.T) {
	generator := NewSLSAAttestationGenerator(nil, nil)

	tests := []struct {
		name     string
		imageRef string
		wantErr  bool
	}{
		{
			name:     "empty image reference",
			imageRef: "",
			wantErr:  true,
		},
		{
			name:     "valid image reference",
			imageRef: "alpine:latest",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			attestations, err := generator.GetAttestations(ctx, tt.imageRef)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SLSAAttestationGenerator.GetAttestations() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SLSAAttestationGenerator.GetAttestations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if attestations == nil {
				t.Errorf("SLSAAttestationGenerator.GetAttestations() returned nil")
				return
			}

			// Currently returns empty slice
			if len(attestations) != 0 {
				t.Errorf("SLSAAttestationGenerator.GetAttestations() returned %d attestations, expected 0", len(attestations))
			}
		})
	}
}

func TestPolicyEnforcer_EnforcePolicy(t *testing.T) {
	// Create mock verifier
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	verifier := NewCosignVerifier(keyProvider, nil)

	enforcer := NewPolicyEnforcer(verifier)

	policy := &AttestationPolicy{
		RequiredAttestations: []string{"sbom", "provenance"},
		AllowedSigners:       []string{"trusted-signer"},
		Rules: []AttestationRule{
			{
				PredicateType: "sbom",
				Conditions: []Condition{
					{
						Field:    "format",
						Operator: "equals",
						Value:    "spdx",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		imageRef string
		policy   *AttestationPolicy
		wantErr  bool
	}{
		{
			name:     "empty image reference",
			imageRef: "",
			policy:   policy,
			wantErr:  true,
		},
		{
			name:     "nil policy",
			imageRef: "alpine:latest",
			policy:   nil,
			wantErr:  true,
		},
		{
			name:     "valid request",
			imageRef: "alpine:latest",
			policy:   policy,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := enforcer.EnforcePolicy(ctx, tt.imageRef, tt.policy)

			if tt.wantErr {
				if err == nil {
					t.Errorf("PolicyEnforcer.EnforcePolicy() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("PolicyEnforcer.EnforcePolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Errorf("PolicyEnforcer.EnforcePolicy() returned nil result")
				return
			}

			// Currently allows all (not implemented)
			if !result.Allowed {
				t.Errorf("PolicyEnforcer.EnforcePolicy() Allowed = false, expected true (not implemented)")
			}
		})
	}
}

func BenchmarkSLSAAttestationGenerator_GenerateAttestation(b *testing.B) {
	// Create mock signer and key provider
	tempDir := b.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	generator := NewSLSAAttestationGenerator(signer, keyProvider)

	req := &AttestationRequest{
		Subject:       "benchmark-subject",
		PredicateType: "benchmark-predicate-type",
		Predicate: map[string]interface{}{
			"benchmark": "data",
			"large":     make([]string, 1000),
		},
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := generator.GenerateAttestation(ctx, req)
		if err != nil {
			b.Fatalf("GenerateAttestation error: %v", err)
		}
	}
}

func BenchmarkAttestationValidator_ValidateAttestation(b *testing.B) {
	// Create mock verifier
	tempDir := b.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	verifier := NewCosignVerifier(keyProvider, nil)

	validator := NewAttestationValidator(verifier)

	// Generate test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		b.Fatalf("Failed to generate test key: %v", err)
	}

	attestation := &Attestation{
		Type: "https://in-toto.io/Statement/v0.1",
		Subject: []*AttestationSubject{
			{
				Name: "benchmark-subject",
				Digest: map[string]string{
					"sha256": "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				},
			},
		},
		PredicateType: "benchmark-predicate-type",
		Predicate: map[string]interface{}{
			"benchmark": "data",
			"large":     make([]string, 1000),
		},
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := validator.ValidateAttestation(ctx, attestation, &privateKey.PublicKey)
		if err != nil {
			b.Fatalf("ValidateAttestation error: %v", err)
		}
	}
}
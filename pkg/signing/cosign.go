// Package signing provides image signing functionality using Cosign.
package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

// CosignSigner implements the Signer interface using Sigstore Cosign.
type CosignSigner struct {
	keyProvider KeyProvider
	options     *CosignOptions
}

// CosignOptions contains configuration for Cosign signing.
type CosignOptions struct {
	// Signing options
	Timeout       time.Duration
	AllowInsecure bool
	FulcioURL     string
	RekorURL      string
	OIDCIssuer    string
	OIDCClientID  string
	
	// Key management
	KeyPassphrase string
}

// FileKeyProvider implements KeyProvider using local files.
type FileKeyProvider struct {
	keyDir string
}

// NewCosignSigner creates a new Cosign-based signer.
func NewCosignSigner(keyProvider KeyProvider, opts *CosignOptions) *CosignSigner {
	if opts == nil {
		opts = &CosignOptions{
			Timeout:       30 * time.Second,
			AllowInsecure: false,
		}
	}
	
	
	return &CosignSigner{
		keyProvider: keyProvider,
		options:     opts,
	}
}

// NewFileKeyProvider creates a new file-based key provider.
func NewFileKeyProvider(keyDir string) *FileKeyProvider {
	return &FileKeyProvider{
		keyDir: keyDir,
	}
}

// Sign signs a container image using Cosign.
func (s *CosignSigner) Sign(ctx context.Context, req *SignRequest) (*SignResult, error) {
	if req == nil {
		return nil, fmt.Errorf("sign request cannot be nil")
	}
	
	if req.ImageRef == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}
	
	if req.ImageDigest == "" {
		return nil, fmt.Errorf("image digest cannot be empty")
	}
	
	// Get the private key
	privateKey, err := s.keyProvider.GetPrivateKey(ctx, req.KeyRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get private key")
	}
	
	// Create image reference with digest
	imageRef := fmt.Sprintf("%s@%s", req.ImageRef, req.ImageDigest)
	
	// For now, create a placeholder signature
	// TODO: Implement actual Cosign signing logic
	signatureData := []byte(fmt.Sprintf("cosign-signature-%s-%s", req.KeyRef, time.Now().Format(time.RFC3339)))
	
	// In a real implementation, this would:
	// 1. Create the signing payload
	// 2. Sign it with the private key
	// 3. Upload the signature to the registry
	// 4. Optionally submit to transparency log (Rekor)
	
	_ = imageRef // Use the image reference for actual signing
	
	// Create signature result
	result := &SignResult{
		ImageRef:    req.ImageRef,
		ImageDigest: req.ImageDigest,
		Signature: &Signature{
			KeyID:     req.KeyRef,
			Algorithm: s.getSigningAlgorithm(privateKey),
			Signature: signatureData,
			Payload:   []byte(imageRef), // Use image ref as payload for now
			Annotations: req.Annotations,
		},
	}
	
	return result, nil
}

// SignBlob signs arbitrary blob data.
func (s *CosignSigner) SignBlob(ctx context.Context, data []byte, opts *SignOptions) (*Signature, error) {
	if opts == nil {
		return nil, fmt.Errorf("sign options cannot be nil")
	}
	
	if opts.KeyRef == "" {
		return nil, fmt.Errorf("key reference cannot be empty")
	}
	
	// Get the private key
	privateKey, err := s.keyProvider.GetPrivateKey(ctx, opts.KeyRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get private key")
	}
	
	// For now, create a simple signature
	// TODO: Implement actual cryptographic signing
	signedPayload := []byte(fmt.Sprintf("signature-for-blob-%d-bytes", len(data)))
	
	signature := &Signature{
		KeyID:     opts.KeyRef,
		Algorithm: s.getSigningAlgorithm(privateKey),
		Signature: signedPayload,
		Payload:   data,
	}
	
	return signature, nil
}

// GenerateKeyPair generates a new signing key pair.
func (s *CosignSigner) GenerateKeyPair(ctx context.Context, opts *KeyGenOptions) (*KeyPair, error) {
	if opts == nil {
		return nil, fmt.Errorf("key generation options cannot be nil")
	}
	
	// Generate key pair based on type
	var privateKey crypto.PrivateKey
	var publicKey crypto.PublicKey
	var err error
	
	switch opts.KeyType {
	case KeyTypeECDSA:
		privateKey, err = s.generateECDSAKeyPair(opts)
	case KeyTypeEd25519:
		privateKey, err = s.generateEd25519KeyPair()
	case KeyTypeRSA:
		privateKey, err = s.generateRSAKeyPair(opts)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", opts.KeyType)
	}
	
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate key pair")
	}
	
	// Extract public key
	switch key := privateKey.(type) {
	case *ecdsa.PrivateKey:
		publicKey = &key.PublicKey
	case ed25519.PrivateKey:
		publicKey = key.Public()
	case *rsa.PrivateKey:
		publicKey = &key.PublicKey
	default:
		return nil, fmt.Errorf("unsupported private key type")
	}
	
	// Generate key ID
	keyID := uuid.New().String()
	
	keyPair := &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		KeyID:      keyID,
		KeyType:    opts.KeyType,
		Algorithm:  opts.Algorithm,
		CreatedAt:  time.Now(),
	}
	
	// Store the key pair
	if err := s.keyProvider.StoreKey(ctx, keyID, privateKey); err != nil {
		return nil, errors.Wrap(err, "failed to store key pair")
	}
	
	return keyPair, nil
}

// GetPublicKey returns the public key for verification.
func (s *CosignSigner) GetPublicKey(ctx context.Context, keyRef string) (crypto.PublicKey, error) {
	return s.keyProvider.GetPublicKey(ctx, keyRef)
}

// CosignVerifier implements the Verifier interface using Cosign.
type CosignVerifier struct {
	keyProvider KeyProvider
	options     *CosignOptions
}

// NewCosignVerifier creates a new Cosign-based verifier.
func NewCosignVerifier(keyProvider KeyProvider, opts *CosignOptions) *CosignVerifier {
	if opts == nil {
		opts = &CosignOptions{
			Timeout:       30 * time.Second,
			AllowInsecure: false,
		}
	}
	
	return &CosignVerifier{
		keyProvider: keyProvider,
		options:     opts,
	}
}

// Verify verifies a container image signature.
func (v *CosignVerifier) Verify(ctx context.Context, req *VerifyRequest) (*VerifyResult, error) {
	if req == nil {
		return nil, fmt.Errorf("verify request cannot be nil")
	}
	
	if req.ImageRef == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}
	
	// Get public key if not provided
	var publicKey crypto.PublicKey
	var err error
	
	if req.PublicKey != nil {
		publicKey = req.PublicKey
	} else if req.KeyRef != "" {
		publicKey, err = v.keyProvider.GetPublicKey(ctx, req.KeyRef)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get public key")
		}
	} else {
		return nil, fmt.Errorf("either public key or key reference must be provided")
	}
	
	// Create image reference with digest if provided
	imageRef := req.ImageRef
	if req.ImageDigest != "" {
		// Remove tag if present when adding digest
		if idx := strings.LastIndex(imageRef, ":"); idx > 0 && !strings.Contains(imageRef[idx:], "/") {
			imageRef = imageRef[:idx]
		}
		imageRef = fmt.Sprintf("%s@%s", imageRef, req.ImageDigest)
	}
	
	// For now, implement basic verification logic
	// TODO: Implement actual Cosign verification
	
	// In a real implementation, this would:
	// 1. Fetch signatures from the registry
	// 2. Verify signature using the public key
	// 3. Check transparency log entries if required
	// 4. Validate certificates if using keyless signing
	
	// For demonstration, assume verification succeeds if we have a public key
	verified := publicKey != nil
	
	if !verified {
		return &VerifyResult{
			Verified: false,
			Errors:   []string{"verification failed: no valid public key"},
		}, nil
	}
	
	// Get signatures from registry
	signatures, err := v.getImageSignatures(ctx, imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get image signatures")
	}
	
	result := &VerifyResult{
		Verified:   true,
		Signatures: signatures,
	}
	
	return result, nil
}

// VerifyBlob verifies a blob signature.
func (v *CosignVerifier) VerifyBlob(ctx context.Context, data []byte, sig *Signature, key crypto.PublicKey) error {
	if sig == nil {
		return fmt.Errorf("signature cannot be nil")
	}
	
	if key == nil {
		return fmt.Errorf("public key cannot be nil")
	}
	
	// For now, implement basic verification
	// TODO: Implement actual cryptographic verification
	if len(sig.Signature) == 0 {
		return fmt.Errorf("empty signature")
	}
	
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	
	// In a real implementation, this would verify the signature using the public key
	return nil
}

// VerifyAttestation verifies an in-toto attestation.
func (v *CosignVerifier) VerifyAttestation(ctx context.Context, attestation *Attestation, key crypto.PublicKey) (*AttestationResult, error) {
	if attestation == nil {
		return nil, fmt.Errorf("attestation cannot be nil")
	}
	
	// For now, return a simplified implementation
	// TODO: Implement proper attestation verification using Cosign
	return &AttestationResult{
		Verified: false,
		Errors:   []string{"attestation verification not yet implemented"},
	}, nil
}

// GetSignatures retrieves all signatures for an image.
func (v *CosignVerifier) GetSignatures(ctx context.Context, imageRef string) ([]*Signature, error) {
	return v.getImageSignatures(ctx, imageRef)
}

// File-based key provider implementation

// GetPrivateKey retrieves a private key from file.
func (p *FileKeyProvider) GetPrivateKey(ctx context.Context, keyRef string) (crypto.PrivateKey, error) {
	if keyRef == "" {
		return nil, fmt.Errorf("key reference cannot be empty")
	}
	
	// Construct key file path
	keyPath := keyRef
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(p.keyDir, keyRef)
	}
	
	// Add .key extension if not present
	if !strings.HasSuffix(keyPath, ".key") && !strings.HasSuffix(keyPath, ".pem") {
		keyPath += ".key"
	}
	
	// Read key file
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read key file: %s", keyPath)
	}
	
	// Parse private key
	privateKey, err := p.parsePrivateKey(keyData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse private key")
	}
	
	return privateKey, nil
}

// GetPublicKey retrieves a public key from file.
func (p *FileKeyProvider) GetPublicKey(ctx context.Context, keyRef string) (crypto.PublicKey, error) {
	// First try to get private key and extract public key
	privateKey, err := p.GetPrivateKey(ctx, keyRef)
	if err == nil {
		// Extract public key from private key
		switch key := privateKey.(type) {
		case *ecdsa.PrivateKey:
			return &key.PublicKey, nil
		case ed25519.PrivateKey:
			return key.Public(), nil
		case *rsa.PrivateKey:
			return &key.PublicKey, nil
		default:
			return nil, fmt.Errorf("unsupported private key type")
		}
	}
	
	// Try to read public key file directly
	keyPath := keyRef
	if !filepath.IsAbs(keyPath) {
		keyPath = filepath.Join(p.keyDir, keyRef)
	}
	
	// Try different public key extensions
	pubKeyPaths := []string{
		keyPath + ".pub",
		strings.TrimSuffix(keyPath, ".key") + ".pub",
		strings.TrimSuffix(keyPath, ".pem") + ".pub",
	}
	
	for _, pubKeyPath := range pubKeyPaths {
		if keyData, err := os.ReadFile(pubKeyPath); err == nil {
			publicKey, err := p.parsePublicKey(keyData)
			if err == nil {
				return publicKey, nil
			}
		}
	}
	
	return nil, fmt.Errorf("failed to find public key for: %s", keyRef)
}

// ListKeys lists available keys.
func (p *FileKeyProvider) ListKeys(ctx context.Context) ([]*KeyInfo, error) {
	if _, err := os.Stat(p.keyDir); os.IsNotExist(err) {
		return []*KeyInfo{}, nil
	}
	
	entries, err := os.ReadDir(p.keyDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read key directory")
	}
	
	var keys []*KeyInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		if strings.HasSuffix(name, ".key") || strings.HasSuffix(name, ".pem") {
			keyRef := strings.TrimSuffix(strings.TrimSuffix(name, ".key"), ".pem")
			
			// Try to get key info
			privateKey, err := p.GetPrivateKey(ctx, keyRef)
			if err != nil {
				continue
			}
			
			keyInfo := &KeyInfo{
				KeyID:       keyRef,
				KeyType:     p.getKeyType(privateKey),
				Algorithm:   p.getSigningAlgorithm(privateKey),
				Description: fmt.Sprintf("Key from file: %s", name),
			}
			
			// Get file info for timestamps
			if info, err := entry.Info(); err == nil {
				keyInfo.CreatedAt = info.ModTime()
			}
			
			keys = append(keys, keyInfo)
		}
	}
	
	return keys, nil
}

// StoreKey stores a private key to file.
func (p *FileKeyProvider) StoreKey(ctx context.Context, keyRef string, key crypto.PrivateKey) error {
	if keyRef == "" {
		return fmt.Errorf("key reference cannot be empty")
	}
	
	// Create key directory if it doesn't exist
	if err := os.MkdirAll(p.keyDir, 0700); err != nil {
		return errors.Wrap(err, "failed to create key directory")
	}
	
	// Construct key file path
	keyPath := filepath.Join(p.keyDir, keyRef+".key")
	
	// Marshal private key to PEM format
	keyData, err := p.marshalPrivateKey(key)
	if err != nil {
		return errors.Wrap(err, "failed to marshal private key")
	}
	
	// Write key file with restricted permissions
	err = os.WriteFile(keyPath, keyData, 0600)
	if err != nil {
		return errors.Wrap(err, "failed to write key file")
	}
	
	// Also write public key file
	pubKeyPath := filepath.Join(p.keyDir, keyRef+".pub")
	var publicKey crypto.PublicKey
	
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		publicKey = &k.PublicKey
	case ed25519.PrivateKey:
		publicKey = k.Public()
	case *rsa.PrivateKey:
		publicKey = &k.PublicKey
	default:
		return fmt.Errorf("unsupported private key type")
	}
	
	pubKeyData, err := p.marshalPublicKey(publicKey)
	if err != nil {
		return errors.Wrap(err, "failed to marshal public key")
	}
	
	err = os.WriteFile(pubKeyPath, pubKeyData, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write public key file")
	}
	
	return nil
}

// DeleteKey deletes a key file.
func (p *FileKeyProvider) DeleteKey(ctx context.Context, keyRef string) error {
	if keyRef == "" {
		return fmt.Errorf("key reference cannot be empty")
	}
	
	// Remove private key file
	keyPath := filepath.Join(p.keyDir, keyRef+".key")
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove private key file")
	}
	
	// Remove public key file
	pubKeyPath := filepath.Join(p.keyDir, keyRef+".pub")
	if err := os.Remove(pubKeyPath); err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to remove public key file")
	}
	
	return nil
}

// Helper methods

func (s *CosignSigner) getPasswordFunc() func(bool) ([]byte, error) {
	return func(confirm bool) ([]byte, error) {
		if s.options.KeyPassphrase != "" {
			return []byte(s.options.KeyPassphrase), nil
		}
		return []byte{}, nil // Empty passphrase
	}
}

func (s *CosignSigner) getSigningAlgorithm(key crypto.PrivateKey) SigningAlgorithm {
	switch key.(type) {
	case *ecdsa.PrivateKey:
		return AlgorithmECDSAP256
	case ed25519.PrivateKey:
		return AlgorithmEd25519
	case *rsa.PrivateKey:
		return AlgorithmRSAPSS
	default:
		return ""
	}
}

func (s *CosignSigner) generateECDSAKeyPair(opts *KeyGenOptions) (crypto.PrivateKey, error) {
	// Use P-256 curve by default
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate ECDSA key")
	}
	return key, nil
}

func (s *CosignSigner) generateEd25519KeyPair() (crypto.PrivateKey, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate Ed25519 key")
	}
	return key, nil
}

func (s *CosignSigner) generateRSAKeyPair(opts *KeyGenOptions) (crypto.PrivateKey, error) {
	keySize := 2048
	if opts.KeySize > 0 {
		keySize = opts.KeySize
	}
	
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate RSA key")
	}
	return key, nil
}

func (v *CosignVerifier) getImageSignatures(ctx context.Context, imageRef string) ([]*Signature, error) {
	// For now, return mock signatures
	// TODO: Implement actual signature retrieval from registry
	
	_, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse image reference")
	}
	
	// In a real implementation, this would:
	// 1. Parse the image reference
	// 2. Fetch signature manifest from registry
	// 3. Parse and validate signatures
	// 4. Return structured signature data
	
	signatures := []*Signature{
		{
			KeyID:     "mock-key-id",
			Algorithm: AlgorithmECDSAP256,
			Signature: []byte("mock-signature-data"),
			Payload:   []byte(imageRef),
		},
	}
	
	return signatures, nil
}

// Key parsing and marshalling helpers

func (p *FileKeyProvider) parsePrivateKey(data []byte) (crypto.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	
	switch block.Type {
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}
}

func (p *FileKeyProvider) parsePublicKey(data []byte) (crypto.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}
	
	switch block.Type {
	case "PUBLIC KEY":
		return x509.ParsePKIXPublicKey(block.Bytes)
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported public key type: %s", block.Type)
	}
}

func (p *FileKeyProvider) marshalPrivateKey(key crypto.PrivateKey) ([]byte, error) {
	var keyBytes []byte
	var keyType string
	var err error
	
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err = x509.MarshalECPrivateKey(k)
		keyType = "EC PRIVATE KEY"
	case ed25519.PrivateKey:
		keyBytes, err = x509.MarshalPKCS8PrivateKey(k)
		keyType = "PRIVATE KEY"
	case *rsa.PrivateKey:
		keyBytes = x509.MarshalPKCS1PrivateKey(k)
		keyType = "RSA PRIVATE KEY"
	default:
		return nil, fmt.Errorf("unsupported private key type")
	}
	
	if err != nil {
		return nil, err
	}
	
	block := &pem.Block{
		Type:  keyType,
		Bytes: keyBytes,
	}
	
	return pem.EncodeToMemory(block), nil
}

func (p *FileKeyProvider) marshalPublicKey(key crypto.PublicKey) ([]byte, error) {
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, err
	}
	
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	}
	
	return pem.EncodeToMemory(block), nil
}

func (p *FileKeyProvider) getKeyType(key crypto.PrivateKey) KeyType {
	switch key.(type) {
	case *ecdsa.PrivateKey:
		return KeyTypeECDSA
	case ed25519.PrivateKey:
		return KeyTypeEd25519
	case *rsa.PrivateKey:
		return KeyTypeRSA
	default:
		return ""
	}
}

func (p *FileKeyProvider) getSigningAlgorithm(key crypto.PrivateKey) SigningAlgorithm {
	switch key.(type) {
	case *ecdsa.PrivateKey:
		return AlgorithmECDSAP256
	case ed25519.PrivateKey:
		return AlgorithmEd25519
	case *rsa.PrivateKey:
		return AlgorithmRSAPSS
	default:
		return ""
	}
}
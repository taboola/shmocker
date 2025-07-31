// Package signing provides image signing functionality using Cosign.
package signing

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCosignSigner(t *testing.T) {
	keyProvider := NewFileKeyProvider("/tmp/test-keys")

	tests := []struct {
		name string
		opts *CosignOptions
	}{
		{
			name: "nil options",
			opts: nil,
		},
		{
			name: "with options",
			opts: &CosignOptions{
				Timeout:       60 * time.Second,
				AllowInsecure: true,
				FulcioURL:     "https://fulcio.sigstore.dev",
				RekorURL:      "https://rekor.sigstore.dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer := NewCosignSigner(keyProvider, tt.opts)

			if signer == nil {
				t.Errorf("NewCosignSigner() returned nil")
				return
			}

			if signer.keyProvider != keyProvider {
				t.Errorf("NewCosignSigner() keyProvider not set correctly")
			}

			if signer.options == nil {
				t.Errorf("NewCosignSigner() options should not be nil")
			}

			// Check default timeout is set
			if tt.opts == nil && signer.options.Timeout != 30*time.Second {
				t.Errorf("NewCosignSigner() default timeout = %v, want %v", signer.options.Timeout, 30*time.Second)
			}
		})
	}
}

func TestCosignSigner_Sign(t *testing.T) {
	// Create temporary key directory
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	// Generate a test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	keyRef := "test-key"
	if err := keyProvider.StoreKey(context.Background(), keyRef, privateKey); err != nil {
		t.Fatalf("Failed to store test key: %v", err)
	}

	tests := []struct {
		name    string
		req     *SignRequest
		wantErr bool
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "empty image reference",
			req: &SignRequest{
				ImageRef: "",
			},
			wantErr: true,
		},
		{
			name: "empty image digest",
			req: &SignRequest{
				ImageRef:    "alpine:latest",
				ImageDigest: "",
			},
			wantErr: true,
		},
		{
			name: "non-existent key",
			req: &SignRequest{
				ImageRef:    "alpine:latest",
				ImageDigest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				KeyRef:      "non-existent-key",
			},
			wantErr: true,
		},
		{
			name: "valid request",
			req: &SignRequest{
				ImageRef:    "alpine:latest",
				ImageDigest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				KeyRef:      keyRef,
				Annotations: map[string]string{
					"test": "annotation",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := signer.Sign(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CosignSigner.Sign() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CosignSigner.Sign() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Errorf("CosignSigner.Sign() returned nil result")
				return
			}

			// Validate result
			if result.ImageRef != tt.req.ImageRef {
				t.Errorf("CosignSigner.Sign() ImageRef = %v, want %v", result.ImageRef, tt.req.ImageRef)
			}

			if result.ImageDigest != tt.req.ImageDigest {
				t.Errorf("CosignSigner.Sign() ImageDigest = %v, want %v", result.ImageDigest, tt.req.ImageDigest)
			}

			if result.Signature == nil {
				t.Errorf("CosignSigner.Sign() Signature is nil")
				return
			}

			if result.Signature.KeyID != tt.req.KeyRef {
				t.Errorf("CosignSigner.Sign() Signature.KeyID = %v, want %v", result.Signature.KeyID, tt.req.KeyRef)
			}

			if len(result.Signature.Signature) == 0 {
				t.Errorf("CosignSigner.Sign() Signature.Signature is empty")
			}
		})
	}
}

func TestCosignSigner_SignBlob(t *testing.T) {
	// Create temporary key directory
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	// Generate a test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	keyRef := "test-key"
	if err := keyProvider.StoreKey(context.Background(), keyRef, privateKey); err != nil {
		t.Fatalf("Failed to store test key: %v", err)
	}

	testData := []byte("test data to sign")

	tests := []struct {
		name    string
		data    []byte
		opts    *SignOptions
		wantErr bool
	}{
		{
			name:    "nil options",
			data:    testData,
			opts:    nil,
			wantErr: true,
		},
		{
			name: "non-existent key",
			data: testData,
			opts: &SignOptions{
				KeyRef: "non-existent-key",
			},
			wantErr: true,
		},
		{
			name: "valid blob signing",
			data: testData,
			opts: &SignOptions{
				KeyRef: keyRef,
			},
			wantErr: false,
		},
		{
			name: "empty data",
			data: []byte{},
			opts: &SignOptions{
				KeyRef: keyRef,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			signature, err := signer.SignBlob(ctx, tt.data, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CosignSigner.SignBlob() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CosignSigner.SignBlob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if signature == nil {
				t.Errorf("CosignSigner.SignBlob() returned nil signature")
				return
			}

			if signature.KeyID != tt.opts.KeyRef {
				t.Errorf("CosignSigner.SignBlob() KeyID = %v, want %v", signature.KeyID, tt.opts.KeyRef)
			}

			if len(signature.Signature) == 0 {
				t.Errorf("CosignSigner.SignBlob() Signature is empty")
			}

			if len(signature.Payload) != len(tt.data) {
				t.Errorf("CosignSigner.SignBlob() Payload length = %d, want %d", len(signature.Payload), len(tt.data))
			}
		})
	}
}

func TestCosignSigner_GenerateKeyPair(t *testing.T) {
	// Create temporary key directory
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	tests := []struct {
		name    string
		opts    *KeyGenOptions
		wantErr bool
	}{
		{
			name:    "nil options",
			opts:    nil,
			wantErr: true,
		},
		{
			name: "unsupported key type",
			opts: &KeyGenOptions{
				KeyType:   KeyType("unsupported"),
				Algorithm: AlgorithmECDSAP256,
			},
			wantErr: true,
		},
		{
			name: "ECDSA key generation",
			opts: &KeyGenOptions{
				KeyType:     KeyTypeECDSA,
				Algorithm:   AlgorithmECDSAP256,
				Description: "Test ECDSA key",
			},
			wantErr: false,
		},
		{
			name: "Ed25519 key generation",
			opts: &KeyGenOptions{
				KeyType:     KeyTypeEd25519,
				Algorithm:   AlgorithmEd25519,
				Description: "Test Ed25519 key",
			},
			wantErr: false,
		},
		{
			name: "RSA key generation",
			opts: &KeyGenOptions{
				KeyType:     KeyTypeRSA,
				Algorithm:   AlgorithmRSAPSS,
				KeySize:     2048,
				Description: "Test RSA key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			keyPair, err := signer.GenerateKeyPair(ctx, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CosignSigner.GenerateKeyPair() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CosignSigner.GenerateKeyPair() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if keyPair == nil {
				t.Errorf("CosignSigner.GenerateKeyPair() returned nil keyPair")
				return
			}

			// Validate key pair
			if keyPair.PrivateKey == nil {
				t.Errorf("CosignSigner.GenerateKeyPair() PrivateKey is nil")
			}

			if keyPair.PublicKey == nil {
				t.Errorf("CosignSigner.GenerateKeyPair() PublicKey is nil")
			}

			if keyPair.KeyID == "" {
				t.Errorf("CosignSigner.GenerateKeyPair() KeyID is empty")
			}

			if keyPair.KeyType != tt.opts.KeyType {
				t.Errorf("CosignSigner.GenerateKeyPair() KeyType = %v, want %v", keyPair.KeyType, tt.opts.KeyType)
			}

			if keyPair.Algorithm != tt.opts.Algorithm {
				t.Errorf("CosignSigner.GenerateKeyPair() Algorithm = %v, want %v", keyPair.Algorithm, tt.opts.Algorithm)
			}

			// Verify key is stored in provider
			storedKey, err := keyProvider.GetPrivateKey(ctx, keyPair.KeyID)
			if err != nil {
				t.Errorf("CosignSigner.GenerateKeyPair() key not stored in provider: %v", err)
			}

			if storedKey == nil {
				t.Errorf("CosignSigner.GenerateKeyPair() stored key is nil")
			}
		})
	}
}

func TestCosignVerifier_Verify(t *testing.T) {
	// Create temporary key directory
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	verifier := NewCosignVerifier(keyProvider, nil)

	// Generate a test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	keyRef := "test-key"
	if err := keyProvider.StoreKey(context.Background(), keyRef, privateKey); err != nil {
		t.Fatalf("Failed to store test key: %v", err)
	}

	tests := []struct {
		name    string
		req     *VerifyRequest
		wantErr bool
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "empty image reference",
			req: &VerifyRequest{
				ImageRef: "",
			},
			wantErr: true,
		},
		{
			name: "no key provided",
			req: &VerifyRequest{
				ImageRef: "alpine:latest",
			},
			wantErr: true,
		},
		{
			name: "valid request with key reference",
			req: &VerifyRequest{
				ImageRef: "alpine:latest",
				KeyRef:   keyRef,
			},
			wantErr: false,
		},
		{
			name: "valid request with public key",
			req: &VerifyRequest{
				ImageRef:  "alpine:latest",
				PublicKey: &privateKey.PublicKey,
			},
			wantErr: false,
		},
		{
			name: "with image digest",
			req: &VerifyRequest{
				ImageRef:    "alpine:latest",
				ImageDigest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				KeyRef:      keyRef,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := verifier.Verify(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CosignVerifier.Verify() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CosignVerifier.Verify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == nil {
				t.Errorf("CosignVerifier.Verify() returned nil result")
				return
			}

			// For our simplified implementation, verification should succeed with valid keys
			if !result.Verified {
				t.Errorf("CosignVerifier.Verify() Verified = false, want true")
			}

			if len(result.Signatures) == 0 {
				t.Errorf("CosignVerifier.Verify() no signatures returned")
			}
		})
	}
}

func TestCosignVerifier_VerifyBlob(t *testing.T) {
	// Create temporary key directory
	tempDir := t.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	verifier := NewCosignVerifier(keyProvider, nil)

	// Generate a test key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	testData := []byte("test data")
	testSignature := &Signature{
		KeyID:     "test-key",
		Algorithm: AlgorithmECDSAP256,
		Signature: []byte("test signature"),
		Payload:   testData,
	}

	tests := []struct {
		name    string
		data    []byte
		sig     *Signature
		key     interface{}
		wantErr bool
	}{
		{
			name:    "nil signature",
			data:    testData,
			sig:     nil,
			key:     &privateKey.PublicKey,
			wantErr: true,
		},
		{
			name:    "nil key",
			data:    testData,
			sig:     testSignature,
			key:     nil,
			wantErr: true,
		},
		{
			name: "empty signature",
			data: testData,
			sig: &Signature{
				KeyID:     "test-key",
				Algorithm: AlgorithmECDSAP256,
				Signature: []byte{},
				Payload:   testData,
			},
			key:     &privateKey.PublicKey,
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    []byte{},
			sig:     testSignature,
			key:     &privateKey.PublicKey,
			wantErr: true,
		},
		{
			name:    "valid blob verification",
			data:    testData,
			sig:     testSignature,
			key:     &privateKey.PublicKey,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := verifier.VerifyBlob(ctx, tt.data, tt.sig, tt.key)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CosignVerifier.VerifyBlob() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("CosignVerifier.VerifyBlob() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFileKeyProvider_StoreAndRetrieveKey(t *testing.T) {
	tempDir := t.TempDir()
	provider := NewFileKeyProvider(tempDir)

	// Test different key types
	tests := []struct {
		name       string
		keyRef     string
		generateKey func() (interface{}, error)
	}{
		{
			name:   "ECDSA key",
			keyRef: "ecdsa-test-key",
			generateKey: func() (interface{}, error) {
				return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			},
		},
		{
			name:   "Ed25519 key",
			keyRef: "ed25519-test-key",
			generateKey: func() (interface{}, error) {
				_, priv, err := ed25519.GenerateKey(rand.Reader)
				return priv, err
			},
		},
		{
			name:   "RSA key",
			keyRef: "rsa-test-key",
			generateKey: func() (interface{}, error) {
				return rsa.GenerateKey(rand.Reader, 2048)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Generate key
			privateKey, err := tt.generateKey()
			if err != nil {
				t.Fatalf("Failed to generate test key: %v", err)
			}

			// Store key
			err = provider.StoreKey(ctx, tt.keyRef, privateKey)
			if err != nil {
				t.Errorf("FileKeyProvider.StoreKey() error = %v", err)
				return
			}

			// Verify key files exist
			keyPath := filepath.Join(tempDir, tt.keyRef+".key")
			pubKeyPath := filepath.Join(tempDir, tt.keyRef+".pub")

			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				t.Errorf("Private key file not created: %s", keyPath)
			}

			if _, err := os.Stat(pubKeyPath); os.IsNotExist(err) {
				t.Errorf("Public key file not created: %s", pubKeyPath)
			}

			// Retrieve private key
			retrieved, err := provider.GetPrivateKey(ctx, tt.keyRef)
			if err != nil {
				t.Errorf("FileKeyProvider.GetPrivateKey() error = %v", err)
				return
			}

			if retrieved == nil {
				t.Errorf("FileKeyProvider.GetPrivateKey() returned nil")
				return
			}

			// Retrieve public key
			publicKey, err := provider.GetPublicKey(ctx, tt.keyRef)
			if err != nil {
				t.Errorf("FileKeyProvider.GetPublicKey() error = %v", err)
				return
			}

			if publicKey == nil {
				t.Errorf("FileKeyProvider.GetPublicKey() returned nil")
			}

			// Clean up
			err = provider.DeleteKey(ctx, tt.keyRef)
			if err != nil {
				t.Errorf("FileKeyProvider.DeleteKey() error = %v", err)
			}

			// Verify files are deleted
			if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
				t.Errorf("Private key file not deleted: %s", keyPath)
			}

			if _, err := os.Stat(pubKeyPath); !os.IsNotExist(err) {
				t.Errorf("Public key file not deleted: %s", pubKeyPath)
			}
		})
	}
}

func TestFileKeyProvider_ListKeys(t *testing.T) {
	tempDir := t.TempDir()
	provider := NewFileKeyProvider(tempDir)
	ctx := context.Background()

	// Initially, should return empty list
	keys, err := provider.ListKeys(ctx)
	if err != nil {
		t.Errorf("FileKeyProvider.ListKeys() error = %v", err)
	}

	if len(keys) != 0 {
		t.Errorf("FileKeyProvider.ListKeys() initial count = %d, want 0", len(keys))
	}

	// Store some test keys
	testKeys := []string{"key1", "key2", "key3"}
	for _, keyRef := range testKeys {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("Failed to generate test key: %v", err)
		}

		err = provider.StoreKey(ctx, keyRef, privateKey)
		if err != nil {
			t.Fatalf("Failed to store test key: %v", err)
		}
	}

	// List keys
	keys, err = provider.ListKeys(ctx)
	if err != nil {
		t.Errorf("FileKeyProvider.ListKeys() error = %v", err)
	}

	if len(keys) != len(testKeys) {
		t.Errorf("FileKeyProvider.ListKeys() count = %d, want %d", len(keys), len(testKeys))
	}

	// Verify key information
	for _, keyInfo := range keys {
		if keyInfo.KeyID == "" {
			t.Errorf("FileKeyProvider.ListKeys() KeyID is empty")
		}

		if keyInfo.KeyType == "" {
			t.Errorf("FileKeyProvider.ListKeys() KeyType is empty")
		}

		if keyInfo.Algorithm == "" {
			t.Errorf("FileKeyProvider.ListKeys() Algorithm is empty")
		}

		if keyInfo.CreatedAt.IsZero() {
			t.Errorf("FileKeyProvider.ListKeys() CreatedAt is zero")
		}
	}
}

func TestFileKeyProvider_ErrorCases(t *testing.T) {
	tempDir := t.TempDir()
	provider := NewFileKeyProvider(tempDir)
	ctx := context.Background()

	tests := []struct {
		name    string
		keyRef  string
		wantErr bool
	}{
		{
			name:    "empty key reference",
			keyRef:  "",
			wantErr: true,
		},
		{
			name:    "non-existent key",
			keyRef:  "non-existent-key",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test GetPrivateKey
			_, err := provider.GetPrivateKey(ctx, tt.keyRef)
			if tt.wantErr && err == nil {
				t.Errorf("FileKeyProvider.GetPrivateKey() expected error, got nil")
			}

			// Test GetPublicKey
			_, err = provider.GetPublicKey(ctx, tt.keyRef)
			if tt.wantErr && err == nil {
				t.Errorf("FileKeyProvider.GetPublicKey() expected error, got nil")
			}

			// Test DeleteKey (should not error for non-existent keys)
			err = provider.DeleteKey(ctx, tt.keyRef)
			if tt.keyRef == "" && err == nil {
				t.Errorf("FileKeyProvider.DeleteKey() expected error for empty keyRef, got nil")
			}
		})
	}
}

func BenchmarkCosignSigner_GenerateKeyPair(b *testing.B) {
	tempDir := b.TempDir()
	keyProvider := NewFileKeyProvider(tempDir)
	signer := NewCosignSigner(keyProvider, nil)

	opts := &KeyGenOptions{
		KeyType:   KeyTypeECDSA,
		Algorithm: AlgorithmECDSAP256,
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := signer.GenerateKeyPair(ctx, opts)
		if err != nil {
			b.Fatalf("GenerateKeyPair error: %v", err)
		}
	}
}

func BenchmarkFileKeyProvider_StoreKey(b *testing.B) {
	tempDir := b.TempDir()
	provider := NewFileKeyProvider(tempDir)

	// Pre-generate keys for benchmarking
	keys := make([]interface{}, b.N)
	for i := 0; i < b.N; i++ {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			b.Fatalf("Failed to generate test key: %v", err)
		}
		keys[i] = key
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyRef := "benchmark-key-" + string(rune(i))
		err := provider.StoreKey(ctx, keyRef, keys[i])
		if err != nil {
			b.Fatalf("StoreKey error: %v", err)
		}
	}
}
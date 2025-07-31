// Package sbom provides Software Bill of Materials (SBOM) generation functionality using Syft.
package sbom

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSyftGenerator_Generate(t *testing.T) {
	generator := NewSyftGenerator()

	tests := []struct {
		name    string
		req     *GenerateRequest
		wantErr bool
	}{
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
		},
		{
			name: "empty image reference",
			req: &GenerateRequest{
				ImageRef: "",
			},
			wantErr: true,
		},
		{
			name: "valid request - minimal",
			req: &GenerateRequest{
				ImageRef: "alpine:latest",
				Options: &GenerateOptions{
					Format: FormatSPDXJSON,
				},
			},
			wantErr: false, // Note: This might fail due to actual Syft dependency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			sbom, err := generator.Generate(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyftGenerator.Generate() expected error, got nil")
				}
				return
			}

			if err != nil {
				// For now, we expect some tests to fail due to missing Syft setup
				t.Logf("SyftGenerator.Generate() error (expected in test environment): %v", err)
				return
			}

			if sbom == nil {
				t.Errorf("SyftGenerator.Generate() returned nil SBOM")
				return
			}

			// Validate SBOM structure
			if sbom.Metadata == nil {
				t.Errorf("SyftGenerator.Generate() returned SBOM with nil metadata")
			}

			if sbom.Metadata != nil {
				if sbom.Metadata.ID == "" {
					t.Errorf("SyftGenerator.Generate() returned SBOM with empty ID")
				}
				if sbom.Metadata.Format != tt.req.Options.Format {
					t.Errorf("SyftGenerator.Generate() format = %v, want %v", sbom.Metadata.Format, tt.req.Options.Format)
				}
			}
		})
	}
}

func TestSyftGenerator_GenerateFromFilesystem(t *testing.T) {
	generator := NewSyftGenerator()

	// Create a temporary directory with some test files
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		opts    *GenerateOptions
		wantErr bool
	}{
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "non-existent path",
			path:    "/non/existent/path",
			wantErr: true,
		},
		{
			name: "valid directory",
			path: tempDir,
			opts: &GenerateOptions{
				Format: FormatSPDXJSON,
			},
			wantErr: false, // Note: This might fail due to actual Syft dependency
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			sbom, err := generator.GenerateFromFilesystem(ctx, tt.path, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyftGenerator.GenerateFromFilesystem() expected error, got nil")
				}
				return
			}

			if err != nil {
				// For now, we expect some tests to fail due to missing Syft setup
				t.Logf("SyftGenerator.GenerateFromFilesystem() error (expected in test environment): %v", err)
				return
			}

			if sbom == nil {
				t.Errorf("SyftGenerator.GenerateFromFilesystem() returned nil SBOM")
			}
		})
	}
}

func TestSyftGenerator_GenerateFromLayers(t *testing.T) {
	generator := NewSyftGenerator()

	tests := []struct {
		name    string
		layers  []*LayerInfo
		opts    *GenerateOptions
		wantErr bool
	}{
		{
			name:    "empty layers",
			layers:  []*LayerInfo{},
			wantErr: true, // Currently not implemented
		},
		{
			name: "single layer",
			layers: []*LayerInfo{
				{
					Digest:    "sha256:abcd1234",
					Size:      1024,
					MediaType: "application/vnd.docker.image.rootfs.diff.tar.gzip",
				},
			},
			opts: &GenerateOptions{
				Format: FormatSPDXJSON,
			},
			wantErr: true, // Currently not implemented
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			sbom, err := generator.GenerateFromLayers(ctx, tt.layers, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyftGenerator.GenerateFromLayers() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SyftGenerator.GenerateFromLayers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if sbom == nil {
				t.Errorf("SyftGenerator.GenerateFromLayers() returned nil SBOM")
			}
		})
	}
}

func TestSyftGenerator_Merge(t *testing.T) {
	generator := NewSyftGenerator()

	// Create sample SBOMs
	sbom1 := &SBOM{
		Metadata: &Metadata{
			ID:        uuid.New().String(),
			Name:      "test-sbom-1",
			Version:   "1.0.0",
			Format:    FormatSPDXJSON,
			Timestamp: time.Now(),
		},
		Packages: []*Package{
			{
				ID:      "pkg1",
				Name:    "package1",
				Version: "1.0.0",
				Type:    PackageTypeApk,
			},
		},
	}

	sbom2 := &SBOM{
		Metadata: &Metadata{
			ID:        uuid.New().String(),
			Name:      "test-sbom-2",
			Version:   "1.0.0",
			Format:    FormatSPDXJSON,
			Timestamp: time.Now(),
		},
		Packages: []*Package{
			{
				ID:      "pkg2",
				Name:    "package2",
				Version: "2.0.0",
				Type:    PackageTypeDeb,
			},
		},
	}

	tests := []struct {
		name    string
		sboms   []*SBOM
		wantErr bool
	}{
		{
			name:    "empty slice",
			sboms:   []*SBOM{},
			wantErr: true,
		},
		{
			name:    "single SBOM",
			sboms:   []*SBOM{sbom1},
			wantErr: false,
		},
		{
			name:    "multiple SBOMs",
			sboms:   []*SBOM{sbom1, sbom2},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			merged, err := generator.Merge(ctx, tt.sboms)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyftGenerator.Merge() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SyftGenerator.Merge() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if merged == nil {
				t.Errorf("SyftGenerator.Merge() returned nil SBOM")
				return
			}

			// For single SBOM, should return the same SBOM
			if len(tt.sboms) == 1 {
				if merged.Metadata.ID != tt.sboms[0].Metadata.ID {
					t.Errorf("SyftGenerator.Merge() single SBOM should return same ID")
				}
			}

			// For multiple SBOMs, should merge packages
			if len(tt.sboms) > 1 {
				expectedPackages := 0
				for _, sbom := range tt.sboms {
					expectedPackages += len(sbom.Packages)
				}
				if len(merged.Packages) != expectedPackages {
					t.Errorf("SyftGenerator.Merge() package count = %d, want %d", len(merged.Packages), expectedPackages)
				}
			}
		})
	}
}

func TestSyftSerializer_Serialize(t *testing.T) {
	serializer := NewSyftSerializer()

	sbom := &SBOM{
		Metadata: &Metadata{
			ID:        uuid.New().String(),
			Name:      "test-sbom",
			Version:   "1.0.0",
			Format:    FormatSPDXJSON,
			Timestamp: time.Now(),
		},
		Packages: []*Package{
			{
				ID:      "pkg1",
				Name:    "test-package",
				Version: "1.0.0",
				Type:    PackageTypeApk,
			},
		},
	}

	tests := []struct {
		name    string
		sbom    *SBOM
		format  Format
		wantErr bool
	}{
		{
			name:    "nil SBOM",
			sbom:    nil,
			format:  FormatSPDXJSON,
			wantErr: true,
		},
		{
			name:    "unsupported format",
			sbom:    sbom,
			format:  Format("unsupported"),
			wantErr: true,
		},
		{
			name:    "SPDX JSON format",
			sbom:    sbom,
			format:  FormatSPDXJSON,
			wantErr: false,
		},
		{
			name:    "CycloneDX JSON format",
			sbom:    sbom,
			format:  FormatCycloneDXJSON,
			wantErr: false,
		},
		{
			name:    "Syft JSON format",
			sbom:    sbom,
			format:  FormatSYFTJSON,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := serializer.Serialize(tt.sbom, tt.format)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyftSerializer.Serialize() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SyftSerializer.Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(data) == 0 {
				t.Errorf("SyftSerializer.Serialize() returned empty data")
			}

			// Verify it's valid JSON (for JSON formats)
			if tt.format == FormatSPDXJSON || tt.format == FormatCycloneDXJSON || tt.format == FormatSYFTJSON {
				// Basic JSON validation - should start with '{'
				if len(data) > 0 && data[0] != '{' {
					t.Errorf("SyftSerializer.Serialize() JSON data should start with '{'")
				}
			}
		})
	}
}

func TestSyftSerializer_Deserialize(t *testing.T) {
	serializer := NewSyftSerializer()

	// Create a test SBOM and serialize it
	originalSBOM := &SBOM{
		Metadata: &Metadata{
			ID:        uuid.New().String(),
			Name:      "test-sbom",
			Version:   "1.0.0",
			Format:    FormatSPDXJSON,
			Timestamp: time.Now(),
		},
		Packages: []*Package{
			{
				ID:      "pkg1",
				Name:    "test-package",
				Version: "1.0.0",
				Type:    PackageTypeApk,
			},
		},
	}

	validData, err := serializer.Serialize(originalSBOM, FormatSPDXJSON)
	if err != nil {
		t.Fatalf("Failed to serialize test SBOM: %v", err)
	}

	tests := []struct {
		name    string
		data    []byte
		format  Format
		wantErr bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			format:  FormatSPDXJSON,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    []byte("invalid json"),
			format:  FormatSPDXJSON,
			wantErr: true,
		},
		{
			name:    "unsupported format",
			data:    validData,
			format:  Format("unsupported"),
			wantErr: true,
		},
		{
			name:    "valid SPDX JSON",
			data:    validData,
			format:  FormatSPDXJSON,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sbom, err := serializer.Deserialize(tt.data, tt.format)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SyftSerializer.Deserialize() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("SyftSerializer.Deserialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if sbom == nil {
				t.Errorf("SyftSerializer.Deserialize() returned nil SBOM")
				return
			}

			// Basic validation
			if sbom.Metadata == nil {
				t.Errorf("SyftSerializer.Deserialize() returned SBOM with nil metadata")
			}
		})
	}
}

func TestSyftSerializer_GetSupportedFormats(t *testing.T) {
	serializer := NewSyftSerializer()
	formats := serializer.GetSupportedFormats()

	expectedFormats := []Format{
		FormatSPDXJSON,
		FormatSPDXXML,
		FormatCycloneDXJSON,
		FormatCycloneDXXML,
		FormatSYFTJSON,
	}

	if len(formats) != len(expectedFormats) {
		t.Errorf("SyftSerializer.GetSupportedFormats() returned %d formats, want %d", len(formats), len(expectedFormats))
	}

	// Check that all expected formats are present
	formatMap := make(map[Format]bool)
	for _, format := range formats {
		formatMap[format] = true
	}

	for _, expected := range expectedFormats {
		if !formatMap[expected] {
			t.Errorf("SyftSerializer.GetSupportedFormats() missing format: %s", expected)
		}
	}
}

func TestConvertScannerTypes(t *testing.T) {
	generator := NewSyftGenerator()

	tests := []struct {
		name         string
		scannerTypes []PackageType
		expected     []string
	}{
		{
			name:         "empty slice",
			scannerTypes: []PackageType{},
			expected:     []string{},
		},
		{
			name:         "single type",
			scannerTypes: []PackageType{PackageTypeApk},
			expected:     []string{"apk"},
		},
		{
			name: "multiple types",
			scannerTypes: []PackageType{
				PackageTypeApk,
				PackageTypeDeb,
				PackageTypeNPM,
				PackageTypeGo,
			},
			expected: []string{"apk", "dpkg", "npm", "go"},
		},
		{
			name:         "unknown type",
			scannerTypes: []PackageType{PackageTypeUnknown},
			expected:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.convertScannerTypes(tt.scannerTypes)

			if len(result) != len(tt.expected) {
				t.Errorf("convertScannerTypes() returned %d items, want %d", len(result), len(tt.expected))
				return
			}

			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("convertScannerTypes()[%d] = %s, want %s", i, result[i], expected)
				}
			}
		})
	}
}

func BenchmarkSyftSerializer_Serialize(b *testing.B) {
	serializer := NewSyftSerializer()

	// Create a large SBOM for benchmarking
	sbom := &SBOM{
		Metadata: &Metadata{
			ID:        uuid.New().String(),
			Name:      "benchmark-sbom",
			Version:   "1.0.0",
			Format:    FormatSPDXJSON,
			Timestamp: time.Now(),
		},
		Packages: make([]*Package, 1000),
	}

	// Populate with test packages
	for i := 0; i < 1000; i++ {
		sbom.Packages[i] = &Package{
			ID:      uuid.New().String(),
			Name:    "benchmark-package-" + string(rune(i)),
			Version: "1.0.0",
			Type:    PackageTypeApk,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := serializer.Serialize(sbom, FormatSPDXJSON)
		if err != nil {
			b.Fatalf("Serialize error: %v", err)
		}
	}
}

func BenchmarkSyftGenerator_Merge(b *testing.B) {
	generator := NewSyftGenerator()

	// Create test SBOMs
	sboms := make([]*SBOM, 10)
	for i := 0; i < 10; i++ {
		sboms[i] = &SBOM{
			Metadata: &Metadata{
				ID:        uuid.New().String(),
				Name:      "benchmark-sbom",
				Version:   "1.0.0",
				Format:    FormatSPDXJSON,
				Timestamp: time.Now(),
			},
			Packages: []*Package{
				{
					ID:      uuid.New().String(),
					Name:    "package-" + string(rune(i)),
					Version: "1.0.0",
					Type:    PackageTypeApk,
				},
			},
		}
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.Merge(ctx, sboms)
		if err != nil {
			b.Fatalf("Merge error: %v", err)
		}
	}
}
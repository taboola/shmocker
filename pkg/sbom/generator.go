// Package sbom provides Software Bill of Materials (SBOM) generation functionality using Syft.
package sbom

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
	"path/filepath"
	"strings"
	"bytes"
	"io"

	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/artifact"
	"github.com/anchore/syft/syft/cataloging"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
	"github.com/google/uuid"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// SyftGenerator implements the Generator interface using Anchore Syft.
type SyftGenerator struct {
	configuration map[string]interface{}
}

// NewSyftGenerator creates a new Syft-based SBOM generator.
func NewSyftGenerator() *SyftGenerator {
	return &SyftGenerator{
		configuration: make(map[string]interface{}),
	}
}

// Generate creates an SBOM for the given image using Syft.
func (g *SyftGenerator) Generate(ctx context.Context, req *GenerateRequest) (*SBOM, error) {
	if req == nil {
		return nil, fmt.Errorf("generate request cannot be nil")
	}

	if req.ImageRef == "" {
		return nil, fmt.Errorf("image reference cannot be empty")
	}

	// Create source from image reference
	src, err := source.NewFromRegistry(req.ImageRef, source.DefaultRegistryOptions())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create source from image")
	}

	// Create cataloging configuration
	catalogConfig := cataloging.DefaultConfig()
	if req.Options != nil {
		// Apply custom options
		if len(req.Options.ScannerTypes) > 0 {
			// Convert our PackageType to Syft's cataloger configuration
			// This is a simplified mapping - in practice you'd want more sophisticated logic
			catalogConfig.Catalogers = g.convertScannerTypes(req.Options.ScannerTypes)
		}
	}

	// Create SBOM using Syft
	syftSBOM := syft.CreateSBOM(ctx, src, catalogConfig)
	if syftSBOM == nil {
		return nil, fmt.Errorf("failed to create SBOM")
	}

	// Convert Syft SBOM to our SBOM format
	result, err := g.convertSyftSBOM(syftSBOM, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert Syft SBOM")
	}

	return result, nil
}

// GenerateFromFilesystem creates an SBOM from a filesystem path.
func (g *SyftGenerator) GenerateFromFilesystem(ctx context.Context, path string, opts *GenerateOptions) (*SBOM, error) {
	if path == "" {
		return nil, fmt.Errorf("filesystem path cannot be empty")
	}

	// Create source from filesystem path
	src, err := source.NewFromDirectory(source.DirectoryConfig{
		Path: path,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create source from filesystem")
	}

	// Create cataloging configuration
	catalogConfig := cataloging.DefaultConfig()
	if opts != nil && len(opts.ScannerTypes) > 0 {
		catalogConfig.Catalogers = g.convertScannerTypes(opts.ScannerTypes)
	}

	// Create SBOM using Syft
	syftSBOM := syft.CreateSBOM(ctx, src, catalogConfig)
	if syftSBOM == nil {
		return nil, fmt.Errorf("failed to create SBOM from filesystem")
	}

	// Convert to our format
	request := &GenerateRequest{
		ImageRef: fmt.Sprintf("file://%s", path),
		Options:  opts,
	}

	result, err := g.convertSyftSBOM(syftSBOM, request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert filesystem SBOM")
	}

	return result, nil
}

// GenerateFromLayers creates an SBOM from individual image layers.
func (g *SyftGenerator) GenerateFromLayers(ctx context.Context, layers []*LayerInfo, opts *GenerateOptions) (*SBOM, error) {
	// This is a simplified implementation - in practice you'd want to 
	// reconstruct the image from layers and scan it
	return nil, fmt.Errorf("GenerateFromLayers not yet implemented for Syft generator")
}

// Merge combines multiple SBOMs into one.
func (g *SyftGenerator) Merge(ctx context.Context, sboms []*SBOM) (*SBOM, error) {
	if len(sboms) == 0 {
		return nil, fmt.Errorf("no SBOMs provided for merging")
	}

	if len(sboms) == 1 {
		return sboms[0], nil
	}

	// Create a merged SBOM
	merged := &SBOM{
		Metadata: &Metadata{
			ID:        uuid.New().String(),
			Name:      "merged-sbom",
			Version:   "1.0.0",
			Format:    FormatSPDXJSON,
			Timestamp: time.Now(),
			Generator: &GeneratorInfo{
				Name:    "shmocker-syft",
				Version: "1.0.0",
			},
			Subject: &Subject{
				Type: "merged",
				Name: "merged-components",
			},
		},
		Packages:      []*Package{},
		Files:         []*File{},
		Relationships: []*Relationship{},
	}

	// Merge packages, files, and relationships
	packageMap := make(map[string]*Package)
	for _, sbom := range sboms {
		for _, pkg := range sbom.Packages {
			if existing, exists := packageMap[pkg.ID]; !exists {
				packageMap[pkg.ID] = pkg
				merged.Packages = append(merged.Packages, pkg)
			} else {
				// Merge package information if needed
				if existing.Description == "" && pkg.Description != "" {
					existing.Description = pkg.Description
				}
				// Merge other fields as needed
			}
		}

		// Merge files
		merged.Files = append(merged.Files, sbom.Files...)

		// Merge relationships
		merged.Relationships = append(merged.Relationships, sbom.Relationships...)
	}

	return merged, nil
}

// convertScannerTypes converts our PackageType to Syft cataloger names.
func (g *SyftGenerator) convertScannerTypes(scannerTypes []PackageType) []string {
	catalogers := make([]string, 0, len(scannerTypes))
	for _, scannerType := range scannerTypes {
		switch scannerType {
		case PackageTypeApk:
			catalogers = append(catalogers, "apk")
		case PackageTypeDeb:
			catalogers = append(catalogers, "dpkg")
		case PackageTypeRpm:
			catalogers = append(catalogers, "rpm")
		case PackageTypeNPM:
			catalogers = append(catalogers, "npm")
		case PackageTypePyPI:
			catalogers = append(catalogers, "python")
		case PackageTypeGem:
			catalogers = append(catalogers, "gem")
		case PackageTypeGo:
			catalogers = append(catalogers, "go")
		case PackageTypeCargo:
			catalogers = append(catalogers, "rust")
		case PackageTypeMaven:
			catalogers = append(catalogers, "java")
		// Add more mappings as needed
		}
	}
	return catalogers
}

// convertSyftSBOM converts a Syft SBOM to our SBOM format.
func (g *SyftGenerator) convertSyftSBOM(syftSBOM *sbom.SBOM, req *GenerateRequest) (*SBOM, error) {
	if syftSBOM == nil {
		return nil, fmt.Errorf("syft SBOM cannot be nil")
	}

	// Generate SBOM ID
	sbomID := uuid.New().String()

	// Convert packages
	packages := make([]*Package, 0, len(syftSBOM.Artifacts.Packages.Sorted()))
	for _, syftPkg := range syftSBOM.Artifacts.Packages.Sorted() {
		pkg := g.convertSyftPackage(syftPkg)
		packages = append(packages, pkg)
	}

	// Convert files if requested
	var files []*File
	if req.Options != nil && req.Options.IncludeFiles {
		files = g.convertSyftFiles(syftSBOM.Artifacts.FileMetadata)
	}

	// Create relationships
	relationships := g.createRelationships(syftSBOM, sbomID)

	// Create subject information
	subject := &Subject{
		Type: "container-image",
		Name: req.ImageRef,
	}

	if req.ImageDigest != "" {
		subject.Digest = req.ImageDigest
	}

	// Create metadata
	metadata := &Metadata{
		ID:        sbomID,
		Name:      fmt.Sprintf("sbom-%s", strings.ReplaceAll(req.ImageRef, "/", "-")),
		Version:   "1.0.0",
		Format:    FormatSPDXJSON, // Default format
		Timestamp: time.Now(),
		Generator: &GeneratorInfo{
			Name:          "shmocker-syft",
			Version:       "1.0.0",
			Configuration: g.configuration,
		},
		Subject: subject,
	}

	// Override format if specified
	if req.Options != nil {
		metadata.Format = req.Options.Format
	}

	result := &SBOM{
		Metadata:      metadata,
		Packages:      packages,
		Files:         files,
		Relationships: relationships,
	}

	return result, nil
}

// convertSyftPackage converts a Syft package to our Package format.
func (g *SyftGenerator) convertSyftPackage(syftPkg artifact.Package) *Package {
	pkg := &Package{
		ID:          syftPkg.ID(),
		Name:        syftPkg.Name,
		Version:     syftPkg.Version,
		Type:        g.convertSyftPackageType(syftPkg.Type),
		Description: syftPkg.Description,
		PURL:        syftPkg.PURL,
		Metadata:    make(map[string]interface{}),
	}

	// Add licenses
	if len(syftPkg.Licenses.ToSlice()) > 0 {
		pkg.Licenses = make([]*License, 0, len(syftPkg.Licenses.ToSlice()))
		for _, license := range syftPkg.Licenses.ToSlice() {
			pkg.Licenses = append(pkg.Licenses, &License{
				ID:   license.Value,
				Name: license.Value,
			})
		}
	}

	// Add locations as metadata
	if len(syftPkg.Locations.ToSlice()) > 0 {
		locations := make([]string, 0, len(syftPkg.Locations.ToSlice()))
		for _, loc := range syftPkg.Locations.ToSlice() {
			locations = append(locations, loc.RealPath)
		}
		pkg.Metadata["locations"] = locations
	}

	return pkg
}

// convertSyftPackageType converts Syft package type to our PackageType.
func (g *SyftGenerator) convertSyftPackageType(syftType artifact.Type) PackageType {
	switch syftType {
	case artifact.ApkPkg:
		return PackageTypeApk
	case artifact.DebPkg:
		return PackageTypeDeb
	case artifact.RpmPkg:
		return PackageTypeRpm
	case artifact.NpmPkg:
		return PackageTypeNPM
	case artifact.PythonPkg:
		return PackageTypePyPI
	case artifact.GemPkg:
		return PackageTypeGem
	case artifact.GoModulePkg:
		return PackageTypeGo
	case artifact.RustPkg:
		return PackageTypeCargo
	case artifact.JavaPkg:
		return PackageTypeMaven
	default:
		return PackageTypeUnknown
	}
}

// convertSyftFiles converts Syft file metadata to our File format.
func (g *SyftGenerator) convertSyftFiles(fileMetadata map[source.Coordinates]source.FileMetadata) []*File {
	files := make([]*File, 0, len(fileMetadata))
			
	for coords, metadata := range fileMetadata {
		file := &File{
			ID:           fmt.Sprintf("file-%s", coords.RealPath),
			Path:         coords.RealPath,
			Size:         metadata.Size(),
			MimeType:     metadata.MIMEType,
			IsExecutable: metadata.IsExecutable(),
			Metadata:     make(map[string]interface{}),
		}

		// Add digests as checksums
		if len(metadata.Digests) > 0 {
			file.Checksums = make([]*Checksum, 0, len(metadata.Digests))
			for _, digest := range metadata.Digests {
				file.Checksums = append(file.Checksums, &Checksum{
					Algorithm: digest.Algorithm,
					Value:     digest.Value,
				})
			}
		}

		files = append(files, file)
	}

	return files
}

// createRelationships creates relationships between SBOM components.
func (g *SyftGenerator) createRelationships(syftSBOM *sbom.SBOM, sbomID string) []*Relationship {
	relationships := make([]*Relationship, 0)

	// Create describes relationships for each package
	for _, syftPkg := range syftSBOM.Artifacts.Packages.Sorted() {
		relationships = append(relationships, &Relationship{
			Subject: sbomID,
			Type:    RelationshipDescribes,
			Object:  syftPkg.ID(),
			Comment: "SBOM describes package",
		})
	}

	// Add package relationships from Syft
	for _, rel := range syftSBOM.Relationships {
		relType := g.convertSyftRelationshipType(rel.Type)
		if relType != "" {
			relationships = append(relationships, &Relationship{
				Subject: string(rel.From.ID()),
				Type:    RelationshipType(relType),
				Object:  string(rel.To.ID()),
			})
		}
	}

	return relationships
}

// convertSyftRelationshipType converts Syft relationship types to our format.
func (g *SyftGenerator) convertSyftRelationshipType(syftType artifact.RelationshipType) string {
	switch syftType {
	case artifact.ContainsRelationship:
		return string(RelationshipContains)
	case artifact.DependencyOfRelationship:
		return string(RelationshipDependsOn)
	default:
		return ""
	}
}

// SyftSerializer implements the Serializer interface for Syft formats.
type SyftSerializer struct{}

// NewSyftSerializer creates a new Syft-based serializer.
func NewSyftSerializer() *SyftSerializer {
	return &SyftSerializer{}
}

// Serialize converts an SBOM to the specified format.
func (s *SyftSerializer) Serialize(sbomData *SBOM, format Format) ([]byte, error) {
	if sbomData == nil {
		return nil, fmt.Errorf("SBOM cannot be nil")
	}

	// For now, implement basic JSON serialization
	// TODO: Implement proper format-specific serialization using Syft
	switch format {
	case FormatSPDXJSON, FormatCycloneDXJSON, FormatSYFTJSON:
		// Simple JSON serialization for now
		return json.MarshalIndent(sbomData, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// SerializeToWriter writes an SBOM to a writer in the specified format.
func (s *SyftSerializer) SerializeToWriter(sbomData *SBOM, format Format, writer io.Writer) error {
	data, err := s.Serialize(sbomData, format)
	if err != nil {
		return err
	}

	_, err = writer.Write(data)
	return err
}

// Deserialize converts serialized data back to an SBOM.
func (s *SyftSerializer) Deserialize(data []byte, format Format) (*SBOM, error) {
	// For now, implement basic JSON deserialization
	// TODO: Implement proper format-specific deserialization using Syft
	switch format {
	case FormatSPDXJSON, FormatCycloneDXJSON, FormatSYFTJSON:
		var sbomData SBOM
		if err := json.Unmarshal(data, &sbomData); err != nil {
			return nil, errors.Wrap(err, "failed to deserialize SBOM")
		}
		return &sbomData, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// GetSupportedFormats returns the formats this serializer supports.
func (s *SyftSerializer) GetSupportedFormats() []Format {
	return []Format{
		FormatSPDXJSON,
		FormatSPDXXML,
		FormatCycloneDXJSON,
		FormatCycloneDXXML,
		FormatSYFTJSON,
	}
}

// getSyftEncoder returns the appropriate Syft encoder for the format.
func (s *SyftSerializer) getSyftEncoder(format Format) (interface{}, error) {
	// For now, return a simplified implementation
	// TODO: Implement proper Syft format encoders when API is stable
	return nil, fmt.Errorf("syft encoder not implemented for format: %s", format)
}

// getSyftDecoder returns the appropriate Syft decoder for the format.
func (s *SyftSerializer) getSyftDecoder(format Format) (interface{}, error) {
	// For now, return a simplified implementation  
	// TODO: Implement proper Syft format decoders when API is stable
	return nil, fmt.Errorf("syft decoder not implemented for format: %s", format)
}

// convertToSyftSBOM converts our SBOM format to Syft's SBOM format.
func (s *SyftSerializer) convertToSyftSBOM(sbomData *SBOM) (*sbom.SBOM, error) {
	// This is a simplified conversion - in practice you'd need more comprehensive mapping
	syftSBOM := &sbom.SBOM{
		Artifacts: sbom.Artifacts{
			Packages: artifact.NewCollection(),
		},
		Relationships: []artifact.Relationship{},
		Source:        sbom.Source{},
		Descriptor: sbom.Descriptor{
			Name:    sbomData.Metadata.Generator.Name,
			Version: sbomData.Metadata.Generator.Version,
		},
	}

	// Convert packages back to Syft format
	for _, pkg := range sbomData.Packages {
		syftPkg := s.convertToSyftPackage(pkg)
		syftSBOM.Artifacts.Packages.Add(syftPkg)
	}

	return syftSBOM, nil
}

// convertToSyftPackage converts our Package to Syft's package format.
func (s *SyftSerializer) convertToSyftPackage(pkg *Package) artifact.Package {
	// This is a simplified conversion
	syftPkg := artifact.Package{
		Name:         pkg.Name,
		Version:      pkg.Version,
		Type:         s.convertToSyftPackageType(pkg.Type),
		PURL:         pkg.PURL,
		Description:  pkg.Description,
		Licenses:     artifact.NewLicenseSet(),
		Locations:    source.NewLocationSet(),
		CPEs:         artifact.NewCPESet(),
	}

	// Convert licenses
	for _, license := range pkg.Licenses {
		syftPkg.Licenses.Add(artifact.NewLicense(license.ID))
	}

	return syftPkg
}

// convertToSyftPackageType converts our PackageType to Syft's type.
func (s *SyftSerializer) convertToSyftPackageType(pkgType PackageType) artifact.Type {
	switch pkgType {
	case PackageTypeApk:
		return artifact.ApkPkg
	case PackageTypeDeb:
		return artifact.DebPkg
	case PackageTypeRpm:
		return artifact.RpmPkg
	case PackageTypeNPM:
		return artifact.NpmPkg
	case PackageTypePyPI:
		return artifact.PythonPkg
	case PackageTypeGem:
		return artifact.GemPkg
	case PackageTypeGo:
		return artifact.GoModulePkg
	case PackageTypeCargo:
		return artifact.RustPkg
	case PackageTypeMaven:
		return artifact.JavaPkg
	default:
		return artifact.UnknownPkg
	}
}
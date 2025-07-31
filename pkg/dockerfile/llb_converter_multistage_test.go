package dockerfile

import (
	"strings"
	"testing"
)

func TestLLBConverter_MultiStageBasic(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18 AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN make build

FROM alpine:3.18 AS runtime
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /src/dist/app /app/app
CMD ["/app/app"]
`

	// Parse Dockerfile
	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Validate AST
	if err := parser.Validate(ast); err != nil {
		t.Fatalf("Failed to validate AST: %v", err)
	}

	// Check stages
	if len(ast.Stages) != 2 {
		t.Fatalf("Expected 2 stages, got %d", len(ast.Stages))
	}

	// Check first stage
	builderStage := ast.Stages[0]
	if builderStage.Name != "builder" {
		t.Errorf("Expected stage name 'builder', got '%s'", builderStage.Name)
	}
	if builderStage.From.Image != "alpine" {
		t.Errorf("Expected base image 'alpine', got '%s'", builderStage.From.Image)
	}

	// Check second stage
	runtimeStage := ast.Stages[1]
	if runtimeStage.Name != "runtime" {
		t.Errorf("Expected stage name 'runtime', got '%s'", runtimeStage.Name)
	}

	// Find COPY --from instruction
	var copyFromFound bool
	for _, instr := range runtimeStage.Instructions {
		if copy, ok := instr.(*CopyInstruction); ok && copy.From == "builder" {
			copyFromFound = true
			if len(copy.Sources) != 1 || copy.Sources[0] != "/src/dist/app" {
				t.Errorf("Expected COPY source '/src/dist/app', got %v", copy.Sources)
			}
			if copy.Destination != "/app/app" {
				t.Errorf("Expected COPY destination '/app/app', got '%s'", copy.Destination)
			}
			break
		}
	}
	if !copyFromFound {
		t.Error("COPY --from=builder instruction not found")
	}

	// Convert to LLB
	converter := NewLLBConverter()
	opts := &ConvertOptions{
		BuildArgs: map[string]string{},
	}

	llbDef, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Failed to convert to LLB: %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}

	if len(llbDef.Definition) == 0 {
		t.Error("LLB definition is empty")
	}
}

func TestLLBConverter_MultiStageWithTarget(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18 AS base
RUN apk add --no-cache ca-certificates

FROM base AS builder
RUN apk add --no-cache git make
WORKDIR /src
COPY . .
RUN make build

FROM base AS runtime
WORKDIR /app
COPY --from=builder /src/dist/app /app/app
CMD ["/app/app"]

FROM runtime AS debug
RUN apk add --no-cache gdb strace
CMD ["/app/app", "--debug"]
`

	// Parse Dockerfile
	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Test targeting builder stage
	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Target: "builder",
	}

	llbDef, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Failed to convert to LLB with target 'builder': %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}

	// Test targeting runtime stage
	opts.Target = "runtime"
	llbDef, err = converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Failed to convert to LLB with target 'runtime': %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}

	// Test invalid target
	opts.Target = "nonexistent"
	_, err = converter.Convert(ast, opts)
	if err == nil {
		t.Error("Expected error for nonexistent target stage")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' in error message, got: %v", err)
	}
}

func TestLLBConverter_MultiStageStageReference(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18 AS base
RUN apk add --no-cache ca-certificates

FROM base AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN make build

FROM base AS runtime
WORKDIR /app
COPY --from=builder /src/dist/app /app/app
CMD ["/app/app"]
`

	// Parse Dockerfile
	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Validate that builder stage references base stage
	if len(ast.Stages) != 3 {
		t.Fatalf("Expected 3 stages, got %d", len(ast.Stages))
	}

	builderStage := ast.Stages[1]
	if builderStage.From.Stage != "base" {
		t.Errorf("Expected builder stage to reference 'base', got '%s'", builderStage.From.Stage)
	}

	runtimeStage := ast.Stages[2]
	if runtimeStage.From.Stage != "base" {
		t.Errorf("Expected runtime stage to reference 'base', got '%s'", runtimeStage.From.Stage)
	}

	// Convert to LLB
	converter := NewLLBConverter()
	opts := &ConvertOptions{}

	llbDef, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Failed to convert to LLB: %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}
}

func TestLLBConverter_MultiStageComplexDependencies(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18 AS deps
RUN apk add --no-cache ca-certificates curl

FROM golang:1.21-alpine AS builder
COPY --from=deps /etc/ssl/certs /etc/ssl/certs
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app

FROM alpine:3.18 AS tools
RUN apk add --no-cache git make

FROM deps AS runtime
COPY --from=builder /src/app /app/app
COPY --from=tools /usr/bin/git /usr/bin/git
CMD ["/app/app"]
`

	// Parse Dockerfile
	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Validate stage dependencies
	if len(ast.Stages) != 4 {
		t.Fatalf("Expected 4 stages, got %d", len(ast.Stages))
	}

	// Check that builder stage copies from deps
	builderStage := ast.Stages[1]
	foundDepsReference := false
	for _, instr := range builderStage.Instructions {
		if copy, ok := instr.(*CopyInstruction); ok && copy.From == "deps" {
			foundDepsReference = true
			break
		}
	}
	if !foundDepsReference {
		t.Error("Builder stage should copy from deps stage")
	}

	// Check that runtime stage references deps as base and copies from builder and tools
	runtimeStage := ast.Stages[3]
	if runtimeStage.From.Stage != "deps" {
		t.Errorf("Expected runtime stage to reference 'deps', got '%s'", runtimeStage.From.Stage)
	}

	builderCopyFound := false
	toolsCopyFound := false
	for _, instr := range runtimeStage.Instructions {
		if copy, ok := instr.(*CopyInstruction); ok {
			if copy.From == "builder" {
				builderCopyFound = true
			}
			if copy.From == "tools" {
				toolsCopyFound = true
			}
		}
	}
	if !builderCopyFound {
		t.Error("Runtime stage should copy from builder stage")
	}
	if !toolsCopyFound {
		t.Error("Runtime stage should copy from tools stage")
	}

	// Convert to LLB
	converter := NewLLBConverter()
	opts := &ConvertOptions{}

	llbDef, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Failed to convert to LLB: %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}
}

func TestLLBConverter_MultiStageInvalidDependencies(t *testing.T) {
	// Test forward reference (should fail validation)
	dockerfileContent := `
FROM alpine:3.18 AS base
COPY --from=future /app/binary /app/binary

FROM alpine:3.18 AS future
RUN echo "built binary" > /app/binary
`

	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// This should pass parsing but fail during validation in the builder
	// The converter itself doesn't enforce forward reference validation
	// as that's typically done at the builder level

	converter := NewLLBConverter()
	opts := &ConvertOptions{}

	// This should succeed in the converter, as it doesn't validate dependency order
	llbDef, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Converter should not fail on forward references: %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}
}

func TestLLBConverter_MultiStageWithPlatform(t *testing.T) {
	dockerfileContent := `
FROM --platform=linux/amd64 alpine:3.18 AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN make build

FROM --platform=linux/arm64 alpine:3.18 AS runtime
WORKDIR /app
COPY --from=builder /src/dist/app /app/app
CMD ["/app/app"]
`

	// Parse Dockerfile
	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	// Check platform specifications
	if ast.Stages[0].Platform != "linux/amd64" {
		t.Errorf("Expected builder platform 'linux/amd64', got '%s'", ast.Stages[0].Platform)
	}
	if ast.Stages[1].Platform != "linux/arm64" {
		t.Errorf("Expected runtime platform 'linux/arm64', got '%s'", ast.Stages[1].Platform)
	}

	// Convert to LLB with global platform override
	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Platform: "linux/386",
	}

	llbDef, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("Failed to convert to LLB: %v", err)
	}

	if llbDef == nil {
		t.Fatal("LLB definition is nil")
	}
}

func TestFindRequiredStages(t *testing.T) {
	dockerfileContent := `
FROM alpine:3.18 AS base
RUN apk add --no-cache ca-certificates

FROM base AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN make build

FROM base AS tools
RUN apk add --no-cache make

FROM base AS runtime
COPY --from=builder /src/app /app/app
COPY --from=tools /usr/bin/make /usr/bin/make
CMD ["/app/app"]

FROM runtime AS debug
RUN apk add --no-cache gdb
`

	parser := New()
	ast, err := parser.ParseBytes([]byte(dockerfileContent))
	if err != nil {
		t.Fatalf("Failed to parse Dockerfile: %v", err)
	}

	converter := NewLLBConverter().(*LLBConverterImpl)

	// Build stage names map
	stageNames := make(map[string]int)
	for i, stage := range ast.Stages {
		if stage.Name != "" {
			stageNames[stage.Name] = i
		}
	}

	// Test finding required stages for runtime (index 3)
	required := converter.findRequiredStages(ast, 3, stageNames)

	// Should include: base(0), builder(1), tools(2), runtime(3)
	expectedStages := []int{0, 1, 2, 3}
	if len(required) != len(expectedStages) {
		t.Fatalf("Expected %d required stages, got %d: %v", len(expectedStages), len(required), required)
	}

	for i, expected := range expectedStages {
		if required[i] != expected {
			t.Errorf("Expected stage %d at position %d, got %d", expected, i, required[i])
		}
	}

	// Test finding required stages for debug (index 4)
	required = converter.findRequiredStages(ast, 4, stageNames)

	// Should include: base(0), builder(1), tools(2), runtime(3), debug(4)
	expectedStages = []int{0, 1, 2, 3, 4}
	if len(required) != len(expectedStages) {
		t.Fatalf("Expected %d required stages, got %d: %v", len(expectedStages), len(required), required)
	}

	for i, expected := range expectedStages {
		if required[i] != expected {
			t.Errorf("Expected stage %d at position %d, got %d", expected, i, required[i])
		}
	}

	// Test finding required stages for builder only (index 1)
	required = converter.findRequiredStages(ast, 1, stageNames)

	// Should include: base(0), builder(1)
	expectedStages = []int{0, 1}
	if len(required) != len(expectedStages) {
		t.Fatalf("Expected %d required stages, got %d: %v", len(expectedStages), len(required), required)
	}

	for i, expected := range expectedStages {
		if required[i] != expected {
			t.Errorf("Expected stage %d at position %d, got %d", expected, i, required[i])
		}
	}
}
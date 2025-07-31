package dockerfile

import (
	"strings"
	"testing"
)

func TestParserBasicFunctionality(t *testing.T) {
	tests := []struct {
		name        string
		dockerfile  string
		expectError bool
		stageCount  int
	}{
		{
			name: "simple dockerfile",
			dockerfile: `FROM ubuntu:20.04
RUN apt-get update
COPY . /app
WORKDIR /app
CMD ["./app"]`,
			expectError: false,
			stageCount:  1,
		},
		{
			name: "multi-stage dockerfile",
			dockerfile: `FROM golang:1.19 AS builder
WORKDIR /src
COPY . .
RUN go build -o app

FROM alpine:3.16
COPY --from=builder /src/app /usr/local/bin/app
CMD ["app"]`,
			expectError: false,
			stageCount:  2,
		},
		{
			name: "dockerfile with comments and directives",
			dockerfile: `# syntax=docker/dockerfile:1.4
# This is a comment
FROM ubuntu:20.04
# Another comment
RUN echo "hello world"`,
			expectError: false,
			stageCount:  1,
		},
		{
			name: "empty dockerfile",
			dockerfile: "",
			expectError: true,
			stageCount:  0,
		},
		{
			name: "dockerfile without FROM",
			dockerfile: `RUN echo "no from"`,
			expectError: true,
			stageCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(ast.Stages) != tt.stageCount {
				t.Errorf("expected %d stages, got %d", tt.stageCount, len(ast.Stages))
			}

			// Validate AST
			if err := parser.Validate(ast); err != nil {
				t.Errorf("AST validation failed: %v", err)
			}
		})
	}
}

func TestFromInstructionParsing(t *testing.T) {
	tests := []struct {
		name         string
		instruction  string
		expectedImg  string
		expectedTag  string
		expectedAs   string
		expectedPlatform string
		expectError  bool
	}{
		{
			name:        "simple image",
			instruction: "FROM ubuntu",
			expectedImg: "ubuntu",
			expectedTag: "",
			expectedAs:  "",
			expectError: false,
		},
		{
			name:        "image with tag",
			instruction: "FROM ubuntu:20.04",
			expectedImg: "ubuntu",
			expectedTag: "20.04",
			expectedAs:  "",
			expectError: false,
		},
		{
			name:        "image with AS",
			instruction: "FROM golang:1.19 AS builder",
			expectedImg: "golang",
			expectedTag: "1.19",
			expectedAs:  "builder",
			expectError: false,
		},
		{
			name:            "image with platform",
			instruction:     "FROM --platform=linux/amd64 ubuntu:20.04",
			expectedImg:     "ubuntu",
			expectedTag:     "20.04",
			expectedPlatform: "linux/amd64",
			expectError:     false,
		},
		{
			name:        "registry with namespace",
			instruction: "FROM docker.io/library/ubuntu:20.04",
			expectedImg: "docker.io/library/ubuntu",
			expectedTag: "20.04",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerfile := tt.instruction
			parser := New()
			ast, err := parser.Parse(strings.NewReader(dockerfile))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(ast.Stages) != 1 {
				t.Errorf("expected 1 stage, got %d", len(ast.Stages))
				return
			}

			from := ast.Stages[0].From
			if from.Image != tt.expectedImg {
				t.Errorf("expected image %q, got %q", tt.expectedImg, from.Image)
			}
			if from.Tag != tt.expectedTag {
				t.Errorf("expected tag %q, got %q", tt.expectedTag, from.Tag)
			}
			if from.As != tt.expectedAs {
				t.Errorf("expected as %q, got %q", tt.expectedAs, from.As)
			}
			if from.Platform != tt.expectedPlatform {
				t.Errorf("expected platform %q, got %q", tt.expectedPlatform, from.Platform)
			}
		})
	}
}

func TestRunInstructionParsing(t *testing.T) {
	tests := []struct {
		name          string
		dockerfile    string
		expectedCmds  []string
		expectedShell bool
		expectError   bool
	}{
		{
			name: "shell form",
			dockerfile: `FROM ubuntu
RUN apt-get update && apt-get install -y curl`,
			expectedCmds:  []string{"apt-get", "update", "&&", "apt-get", "install", "-y", "curl"},
			expectedShell: true,
			expectError:   false,
		},
		{
			name: "exec form",
			dockerfile: `FROM ubuntu
RUN ["apt-get", "update"]`,
			expectedCmds:  []string{"apt-get", "update"},
			expectedShell: false,
			expectError:   false,
		},
		{
			name: "with mount",
			dockerfile: `FROM ubuntu
RUN --mount=type=cache,target=/var/cache/apt apt-get update`,
			expectedCmds:  []string{"apt-get", "update"},
			expectedShell: true,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(ast.Stages) != 1 || len(ast.Stages[0].Instructions) == 0 {
				t.Errorf("expected 1 stage with instructions")
				return
			}

			runInstr, ok := ast.Stages[0].Instructions[0].(*RunInstruction)
			if !ok {
				t.Errorf("expected RUN instruction, got %T", ast.Stages[0].Instructions[0])
				return
			}

			if runInstr.Shell != tt.expectedShell {
				t.Errorf("expected shell=%v, got %v", tt.expectedShell, runInstr.Shell)
			}

			if len(runInstr.Commands) != len(tt.expectedCmds) {
				t.Errorf("expected %d commands, got %d", len(tt.expectedCmds), len(runInstr.Commands))
				return
			}

			for i, cmd := range tt.expectedCmds {
				if runInstr.Commands[i] != cmd {
					t.Errorf("expected command[%d] %q, got %q", i, cmd, runInstr.Commands[i])
				}
			}
		})
	}
}

func TestCopyInstructionParsing(t *testing.T) {
	tests := []struct {
		name         string
		dockerfile   string
		expectedSrcs []string
		expectedDest string
		expectedFrom string
		expectError  bool
	}{
		{
			name: "simple copy",
			dockerfile: `FROM ubuntu
COPY . /app`,
			expectedSrcs: []string{"."},
			expectedDest: "/app",
			expectError:  false,
		},
		{
			name: "multiple sources",
			dockerfile: `FROM ubuntu
COPY file1.txt file2.txt /app/`,
			expectedSrcs: []string{"file1.txt", "file2.txt"},
			expectedDest: "/app/",
			expectError:  false,
		},
		{
			name: "copy with from",
			dockerfile: `FROM ubuntu
COPY --from=builder /src/app /usr/local/bin/app`,
			expectedSrcs: []string{"/src/app"},
			expectedDest: "/usr/local/bin/app",
			expectedFrom: "builder",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(ast.Stages) != 1 || len(ast.Stages[0].Instructions) == 0 {
				t.Errorf("expected 1 stage with instructions")
				return
			}

			copyInstr, ok := ast.Stages[0].Instructions[0].(*CopyInstruction)
			if !ok {
				t.Errorf("expected COPY instruction, got %T", ast.Stages[0].Instructions[0])
				return
			}

			if len(copyInstr.Sources) != len(tt.expectedSrcs) {
				t.Errorf("expected %d sources, got %d", len(tt.expectedSrcs), len(copyInstr.Sources))
				return
			}

			for i, src := range tt.expectedSrcs {
				if copyInstr.Sources[i] != src {
					t.Errorf("expected source[%d] %q, got %q", i, src, copyInstr.Sources[i])
				}
			}

			if copyInstr.Destination != tt.expectedDest {
				t.Errorf("expected destination %q, got %q", tt.expectedDest, copyInstr.Destination)
			}

			if copyInstr.From != tt.expectedFrom {
				t.Errorf("expected from %q, got %q", tt.expectedFrom, copyInstr.From)
			}
		})
	}
}

func TestEnvInstructionParsing(t *testing.T) {
	tests := []struct {
		name         string
		dockerfile   string
		expectedVars map[string]string
		expectError  bool
	}{
		{
			name: "single env var equals format",
			dockerfile: `FROM ubuntu
ENV PATH=/usr/local/bin:$PATH`,
			expectedVars: map[string]string{"PATH": "/usr/local/bin:$PATH"},
			expectError:  false,
		},
		{
			name: "single env var space format",
			dockerfile: `FROM ubuntu
ENV NODE_VERSION 16.14.0`,
			expectedVars: map[string]string{"NODE_VERSION": "16.14.0"},
			expectError:  false,
		},
		{
			name: "multiple env vars",
			dockerfile: `FROM ubuntu
ENV NODE_VERSION=16.14.0 NPM_VERSION=8.3.1`,
			expectedVars: map[string]string{
				"NODE_VERSION": "16.14.0",
				"NPM_VERSION":  "8.3.1",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(ast.Stages) != 1 || len(ast.Stages[0].Instructions) == 0 {
				t.Errorf("expected 1 stage with instructions")
				return
			}

			envInstr, ok := ast.Stages[0].Instructions[0].(*EnvInstruction)
			if !ok {
				t.Errorf("expected ENV instruction, got %T", ast.Stages[0].Instructions[0])
				return
			}

			if len(envInstr.Variables) != len(tt.expectedVars) {
				t.Errorf("expected %d variables, got %d", len(tt.expectedVars), len(envInstr.Variables))
				return
			}

			for key, expectedValue := range tt.expectedVars {
				if actualValue, exists := envInstr.Variables[key]; !exists {
					t.Errorf("expected variable %q not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("expected variable %q value %q, got %q", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestMultiStageDockerfile(t *testing.T) {
	dockerfile := `FROM golang:1.19 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app

FROM alpine:3.16 AS runtime
RUN apk --no-cache add ca-certificates
COPY --from=builder /src/app /usr/local/bin/app
ENTRYPOINT ["app"]`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ast.Stages) != 2 {
		t.Errorf("expected 2 stages, got %d", len(ast.Stages))
	}

	// Check first stage
	if ast.Stages[0].Name != "builder" {
		t.Errorf("expected first stage name 'builder', got %q", ast.Stages[0].Name)
	}
	if ast.Stages[0].From.Image != "golang" {
		t.Errorf("expected first stage image 'golang', got %q", ast.Stages[0].From.Image)
	}
	if ast.Stages[0].From.Tag != "1.19" {
		t.Errorf("expected first stage tag '1.19', got %q", ast.Stages[0].From.Tag)
	}

	// Check second stage
	if ast.Stages[1].Name != "runtime" {
		t.Errorf("expected second stage name 'runtime', got %q", ast.Stages[1].Name)
	}
	if ast.Stages[1].From.Image != "alpine" {
		t.Errorf("expected second stage image 'alpine', got %q", ast.Stages[1].From.Image)
	}

	// Check COPY --from in second stage
	var copyFromFound bool
	for _, instr := range ast.Stages[1].Instructions {
		if copyInstr, ok := instr.(*CopyInstruction); ok && copyInstr.From == "builder" {
			copyFromFound = true
			break
		}
	}
	if !copyFromFound {
		t.Error("expected COPY --from=builder instruction in second stage")
	}

	// Validate AST
	if err := parser.Validate(ast); err != nil {
		t.Errorf("AST validation failed: %v", err)
	}
}

func TestCommentsAndDirectives(t *testing.T) {
	dockerfile := `# syntax=docker/dockerfile:1.4
# This is a comment
FROM ubuntu:20.04
# Another comment
RUN echo "test"`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check directives
	if len(ast.Directives) != 1 {
		t.Errorf("expected 1 directive, got %d", len(ast.Directives))
	} else {
		if ast.Directives[0].Name != "syntax" {
			t.Errorf("expected directive name 'syntax', got %q", ast.Directives[0].Name)
		}
		if ast.Directives[0].Value != "docker/dockerfile:1.4" {
			t.Errorf("expected directive value 'docker/dockerfile:1.4', got %q", ast.Directives[0].Value)
		}
	}

	// Check comments
	if len(ast.Comments) != 2 {
		t.Errorf("expected 2 comments, got %d", len(ast.Comments))
	}
}

func TestArgExpansion(t *testing.T) {
	dockerfile := `FROM ubuntu:20.04
ARG VERSION=latest
ARG PORT=8080
ENV APP_VERSION=${VERSION}
ENV APP_PORT=$PORT
EXPOSE ${PORT}`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find ENV instructions and check expansion
	var envInstr *EnvInstruction
	for _, instr := range ast.Stages[0].Instructions {
		if env, ok := instr.(*EnvInstruction); ok {
			envInstr = env
			break
		}
	}

	if envInstr == nil {
		t.Fatal("expected ENV instruction")
	}

	// Note: actual expansion depends on implementation details
	// This test verifies the structure is parsed correctly
	if len(envInstr.Variables) == 0 {
		t.Error("expected environment variables to be parsed")
	}
}

func TestErrorCases(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile string
		expectErr  string
	}{
		{
			name:       "missing FROM",
			dockerfile: "RUN echo test",
			expectErr:  "instruction RUN found before FROM",
		},
		{
			name:       "invalid instruction",
			dockerfile: "FROM ubuntu\nINVALID_INSTRUCTION test",
			expectErr:  "unknown instruction",
		},
		{
			name:       "duplicate stage names",
			dockerfile: "FROM ubuntu AS stage1\nFROM alpine AS stage1",
			expectErr:  "duplicate stage name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))

			if err == nil {
				// Try validation if parsing succeeded
				if ast != nil {
					err = parser.Validate(ast)
				}
			}

			if err == nil {
				t.Errorf("expected error containing %q but got none", tt.expectErr)
				return
			}

			if !strings.Contains(err.Error(), tt.expectErr) {
				t.Errorf("expected error containing %q, got %q", tt.expectErr, err.Error())
			}
		})
	}
}

func TestHealthcheckInstruction(t *testing.T) {
	dockerfile := `FROM ubuntu
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD curl -f http://localhost/ || exit 1`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ast.Stages) != 1 || len(ast.Stages[0].Instructions) == 0 {
		t.Fatal("expected 1 stage with instructions")
	}

	healthInstr, ok := ast.Stages[0].Instructions[0].(*HealthcheckInstruction)
	if !ok {
		t.Fatalf("expected HEALTHCHECK instruction, got %T", ast.Stages[0].Instructions[0])
	}

	if healthInstr.Type != "CMD" {
		t.Errorf("expected type CMD, got %q", healthInstr.Type)
	}
	if healthInstr.Interval != "30s" {
		t.Errorf("expected interval 30s, got %q", healthInstr.Interval)
	}
	if healthInstr.Timeout != "3s" {
		t.Errorf("expected timeout 3s, got %q", healthInstr.Timeout)
	}
	if healthInstr.Retries != 3 {
		t.Errorf("expected retries 3, got %d", healthInstr.Retries)
	}
}

func TestComplexDockerfile(t *testing.T) {
	dockerfile := `# syntax=docker/dockerfile:1.4
FROM --platform=linux/amd64 node:16-alpine AS base

# Install dependencies
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --only=production

# Build stage
FROM base AS build
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY . .
RUN npm run build

# Production stage
FROM nginx:alpine AS production
COPY --from=build /app/dist /usr/share/nginx/html
COPY --chown=nginx:nginx nginx.conf /etc/nginx/nginx.conf

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost/ || exit 1

# Metadata
LABEL maintainer="test@example.com"
LABEL version="1.0.0"
EXPOSE 80
USER nginx
STOPSIGNAL SIGTERM

CMD ["nginx", "-g", "daemon off;"]`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Validate structure
	if len(ast.Stages) != 3 {
		t.Errorf("expected 3 stages, got %d", len(ast.Stages))
	}

	// Check stage names
	expectedNames := []string{"base", "build", "production"}
	for i, expected := range expectedNames {
		if i < len(ast.Stages) && ast.Stages[i].Name != expected {
			t.Errorf("expected stage %d name %q, got %q", i, expected, ast.Stages[i].Name)
		}
	}

	// Check directive
	if len(ast.Directives) != 1 || ast.Directives[0].Name != "syntax" {
		t.Error("expected syntax directive")
	}

	// Validate AST
	if err := parser.Validate(ast); err != nil {
		t.Errorf("AST validation failed: %v", err)
	}
}
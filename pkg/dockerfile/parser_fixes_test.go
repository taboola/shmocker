package dockerfile

import (
	"strings"
	"testing"
)

// TestLineContinuationAndFlagParsing tests the critical fixes for line continuation and flag parsing
func TestLineContinuationAndFlagParsing(t *testing.T) {
	tests := []struct {
		name           string
		dockerfile     string
		expectError    bool
		expectedCommands []string
	}{
		{
			name: "RUN with command flags and line continuation",
			dockerfile: `FROM ubuntu
RUN apk add --no-cache \
    curl \
    ca-certificates \
    && rm -rf /var/cache/apk/*`,
			expectError: false,
			expectedCommands: []string{"apk", "add", "--no-cache", "curl", "ca-certificates", "&&", "rm", "-rf", "/var/cache/apk/*"},
		},
		{
			name: "RUN with instruction flags vs command flags",
			dockerfile: `FROM ubuntu
RUN --mount=type=cache,target=/cache apt-get update --no-cache`,
			expectError: false,
			expectedCommands: []string{"apt-get", "update", "--no-cache"},
		},
		{
			name: "ENV with line continuation",
			dockerfile: `FROM ubuntu
ENV PATH=/usr/local/bin:$PATH \
    NODE_VERSION=16.14.0 \
    NPM_VERSION=8.3.1`,
			expectError: false,
		},
		{
			name: "HEALTHCHECK with line continuation",
			dockerfile: `FROM ubuntu
HEALTHCHECK --interval=30s --timeout=3s \
    CMD curl -f http://localhost/ || exit 1`,
			expectError: false,
		},
		{
			name: "Multiple line continuations in RUN",
			dockerfile: `FROM ubuntu
RUN apt-get update && \
    apt-get install -y \
        curl \
        wget \
        vim && \
    apt-get clean`,
			expectError: false,
			expectedCommands: []string{"apt-get", "update", "&&", "apt-get", "install", "-y", "curl", "wget", "vim", "&&", "apt-get", "clean"},
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

			if len(ast.Stages) != 1 {
				t.Errorf("expected 1 stage, got %d", len(ast.Stages))
				return
			}

			// Validate AST
			if err := parser.Validate(ast); err != nil {
				t.Errorf("AST validation failed: %v", err)
				return
			}

			// Check RUN instruction commands if expected
			if tt.expectedCommands != nil {
				if len(ast.Stages[0].Instructions) == 0 {
					t.Errorf("expected at least one instruction")
					return
				}

				runInstr, ok := ast.Stages[0].Instructions[0].(*RunInstruction)
				if !ok {
					t.Errorf("expected first instruction to be RUN, got %T", ast.Stages[0].Instructions[0])
					return
				}

				if len(runInstr.Commands) != len(tt.expectedCommands) {
					t.Errorf("expected %d commands, got %d", len(tt.expectedCommands), len(runInstr.Commands))
					t.Errorf("expected: %v", tt.expectedCommands)
					t.Errorf("got: %v", runInstr.Commands)
					return
				}

				for i, expected := range tt.expectedCommands {
					if runInstr.Commands[i] != expected {
						t.Errorf("command[%d]: expected %q, got %q", i, expected, runInstr.Commands[i])
					}
				}
			}
		})
	}
}

// TestInstructionFlagVsCommandFlag specifically tests the distinction between instruction flags and command flags
func TestInstructionFlagVsCommandFlag(t *testing.T) {
	dockerfile := `FROM ubuntu
RUN --mount=type=cache,target=/cache apt-get update --no-cache --quiet`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ast.Stages) != 1 || len(ast.Stages[0].Instructions) != 1 {
		t.Fatal("expected 1 stage with 1 instruction")
	}

	runInstr, ok := ast.Stages[0].Instructions[0].(*RunInstruction)
	if !ok {
		t.Fatalf("expected RUN instruction, got %T", ast.Stages[0].Instructions[0])
	}

	// Check that --mount was parsed as an instruction flag
	if len(runInstr.Mounts) != 1 {
		t.Errorf("expected 1 mount, got %d", len(runInstr.Mounts))
	}

	if len(runInstr.Mounts) > 0 {
		mount := runInstr.Mounts[0]
		if mount.Type != "cache" {
			t.Errorf("expected mount type 'cache', got %q", mount.Type)
		}
		if mount.Target != "/cache" {
			t.Errorf("expected mount target '/cache', got %q", mount.Target)
		}
	}

	// Check that --no-cache and --quiet were parsed as command arguments
	expectedCommands := []string{"apt-get", "update", "--no-cache", "--quiet"}
	if len(runInstr.Commands) != len(expectedCommands) {
		t.Errorf("expected %d commands, got %d", len(expectedCommands), len(runInstr.Commands))
		t.Errorf("expected: %v", expectedCommands)
		t.Errorf("got: %v", runInstr.Commands)
		return
	}

	for i, expected := range expectedCommands {
		if runInstr.Commands[i] != expected {
			t.Errorf("command[%d]: expected %q, got %q", i, expected, runInstr.Commands[i])
		}
	}
}

// TestRealWorldDockerfilePatterns tests common real-world patterns that were failing before
func TestRealWorldDockerfilePatterns(t *testing.T) {
	// This pattern is common in Alpine-based images
	alpinePattern := `FROM alpine:3.18
RUN apk add --no-cache --update \
    curl \
    ca-certificates \
    && rm -rf /var/cache/apk/*`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(alpinePattern))
	if err != nil {
		t.Fatalf("failed to parse Alpine pattern: %v", err)
	}

	if err := parser.Validate(ast); err != nil {
		t.Fatalf("validation failed for Alpine pattern: %v", err)
	}

	// This pattern is common in Node.js images
	nodePattern := `FROM node:16-alpine
COPY package*.json ./
RUN npm ci --only=production --silent \
    && npm cache clean --force`

	ast, err = parser.Parse(strings.NewReader(nodePattern))
	if err != nil {
		t.Fatalf("failed to parse Node pattern: %v", err)
	}

	if err := parser.Validate(ast); err != nil {
		t.Fatalf("validation failed for Node pattern: %v", err)
	}
}
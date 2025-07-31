package dockerfile

import (
	"strings"
	"testing"
)

func TestLLBConverterBasicConversion(t *testing.T) {
	dockerfile := `FROM ubuntu:20.04
RUN apt-get update
ENV NODE_VERSION=16.14.0
WORKDIR /app
COPY . .
EXPOSE 8080
CMD ["node", "index.js"]`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Platform: "linux/amd64",
	}

	definition, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if definition == nil {
		t.Fatal("expected LLB definition but got nil")
	}

	if len(definition.Definition) == 0 {
		t.Error("expected definition bytes but got empty")
	}

	if definition.Metadata == nil {
		t.Error("expected metadata but got nil")
	}
}

func TestLLBConverterMultiStage(t *testing.T) {
	dockerfile := `FROM golang:1.19 AS builder
WORKDIR /src
COPY . .
RUN go build -o app

FROM alpine:3.16
COPY --from=builder /src/app /usr/local/bin/app
CMD ["app"]`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Target: "builder", // Build only first stage
	}

	definition, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if definition == nil {
		t.Fatal("expected LLB definition but got nil")
	}

	// Test conversion of final stage (default)
	opts.Target = ""
	definition, err = converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("conversion error for final stage: %v", err)
	}

	if definition == nil {
		t.Fatal("expected LLB definition for final stage but got nil")
	}
}

func TestLLBConverterStageConversion(t *testing.T) {
	dockerfile := `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl
ENV PATH=/usr/local/bin:$PATH
WORKDIR /app
USER nginx
EXPOSE 80
VOLUME /data
LABEL version=1.0.0`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{}

	if len(ast.Stages) == 0 {
		t.Fatal("expected at least one stage")
	}

	state, err := converter.ConvertStage(ast.Stages[0], opts)
	if err != nil {
		t.Fatalf("stage conversion error: %v", err)
	}

	if state == nil {
		t.Fatal("expected LLB state but got nil")
	}

	if state.State == nil {
		t.Error("expected state but got nil")
	}

	if state.Metadata == nil {
		t.Error("expected metadata but got nil")
	}

	// Check that metadata contains expected values
	metadata := state.Metadata
	if env, ok := metadata["env"].(map[string]string); ok {
		if _, exists := env["PATH"]; !exists {
			t.Error("expected PATH environment variable in metadata")
		}
	}

	if workdir, ok := metadata["workdir"].(string); !ok || workdir != "/app" {
		t.Errorf("expected workdir '/app', got %v", workdir)
	}

	if user, ok := metadata["user"].(string); !ok || user != "nginx" {
		t.Errorf("expected user 'nginx', got %v", user)
	}

	if volumes, ok := metadata["volumes"].([]string); !ok || len(volumes) == 0 {
		t.Error("expected volumes in metadata")
	}

	if labels, ok := metadata["labels"].(map[string]string); !ok || labels["version"] != "1.0.0" {
		t.Error("expected labels in metadata")
	}

	if expose, ok := metadata["expose"].([]string); !ok || len(expose) == 0 {
		t.Error("expected exposed ports in metadata")
	}
}

func TestLLBConverterBaseImageResolution(t *testing.T) {
	tests := []struct {
		name        string
		from        *FromInstruction
		expectedRef string
		expectError bool
	}{
		{
			name: "simple image",
			from: &FromInstruction{
				Image: "ubuntu",
				Tag:   "20.04",
			},
			expectedRef: "ubuntu:20.04",
			expectError: false,
		},
		{
			name: "image with registry",
			from: &FromInstruction{
				Image: "docker.io/library/ubuntu",
				Tag:   "latest",
			},
			expectedRef: "docker.io/library/ubuntu:latest",
			expectError: false,
		},
		{
			name: "image with digest",
			from: &FromInstruction{
				Image:  "ubuntu",
				Digest: "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			},
			expectedRef: "ubuntu@sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expectError: false,
		},
		{
			name: "stage reference",
			from: &FromInstruction{
				Stage: "builder",
			},
			expectedRef: "builder",
			expectError: false,
		},
		{
			name: "empty FROM",
			from: &FromInstruction{},
			expectError: true,
		},
	}

	converter := NewLLBConverter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := converter.ResolveBaseImage(tt.from)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ref == nil {
				t.Fatal("expected image reference but got nil")
			}

			actualRef := ref.String()
			if actualRef != tt.expectedRef {
				t.Errorf("expected reference %q, got %q", tt.expectedRef, actualRef)
			}
		})
	}
}

func TestLLBConverterRunInstruction(t *testing.T) {
	run := &RunInstruction{
		Commands: []string{"apt-get", "update"},
		Shell:    true,
		Mounts: []*MountInstruction{
			{
				Type:   "cache",
				Target: "/var/cache/apt",
			},
		},
		Network:  "default",
		Security: "sandbox",
	}

	converter := NewLLBConverter()
	currentState := &LLBState{
		State:    map[string]interface{}{"type": "image"},
		Metadata: make(map[string]interface{}),
	}

	newState, err := converter.(*LLBConverterImpl).convertRunInstruction(run, currentState, nil)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if newState == nil {
		t.Fatal("expected new state but got nil")
	}

	state := newState.State.(map[string]interface{})
	if state["type"] != "exec" {
		t.Errorf("expected exec type, got %v", state["type"])
	}

	if meta, ok := state["meta"].(map[string]interface{}); ok {
		if args, ok := meta["args"].([]string); ok {
			// Shell form should wrap in shell
			if len(args) < 3 || args[0] != "/bin/sh" || args[1] != "-c" {
				t.Errorf("expected shell wrapper, got %v", args)
			}
		} else {
			t.Error("expected args in meta")
		}
	} else {
		t.Error("expected meta in state")
	}

	if mounts, ok := state["mounts"]; !ok {
		t.Error("expected mounts in state")
	} else if mountList, ok := mounts.([]map[string]interface{}); ok {
		if len(mountList) != 1 {
			t.Errorf("expected 1 mount, got %d", len(mountList))
		} else {
			mount := mountList[0]
			if mount["type"] != "cache" {
				t.Errorf("expected cache mount, got %v", mount["type"])
			}
			if mount["target"] != "/var/cache/apt" {
				t.Errorf("expected target /var/cache/apt, got %v", mount["target"])
			}
		}
	}

	if state["network"] != "default" {
		t.Errorf("expected network default, got %v", state["network"])
	}

	if state["security"] != "sandbox" {
		t.Errorf("expected security sandbox, got %v", state["security"])
	}
}

func TestLLBConverterCopyInstruction(t *testing.T) {
	copy := &CopyInstruction{
		Sources:     []string{"src/", "config/"},
		Destination: "/app/",
		From:        "builder",
		Chown:       "nginx:nginx",
		Chmod:       "755",
	}

	converter := NewLLBConverter()
	currentState := &LLBState{
		State:    map[string]interface{}{"type": "image"},
		Metadata: make(map[string]interface{}),
	}

	newState, err := converter.(*LLBConverterImpl).convertCopyInstruction(copy, currentState, nil, nil)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if newState == nil {
		t.Fatal("expected new state but got nil")
	}

	state := newState.State.(map[string]interface{})
	if state["type"] != "file" {
		t.Errorf("expected file type, got %v", state["type"])
	}

	if state["from"] != "builder" {
		t.Errorf("expected from builder, got %v", state["from"])
	}

	if actions, ok := state["actions"].([]map[string]interface{}); ok {
		if len(actions) != 1 {
			t.Errorf("expected 1 action, got %d", len(actions))
		} else {
			action := actions[0]
			if action["action"] != "copy" {
				t.Errorf("expected copy action, got %v", action["action"])
			}
			if action["src"] != "src/ config/" {
				t.Errorf("expected 'src/ config/', got %v", action["src"])
			}
			if action["dest"] != "/app/" {
				t.Errorf("expected '/app/', got %v", action["dest"])
			}
			if action["chown"] != "nginx:nginx" {
				t.Errorf("expected chown nginx:nginx, got %v", action["chown"])
			}
			if action["chmod"] != "755" {
				t.Errorf("expected chmod 755, got %v", action["chmod"])
			}
		}
	} else {
		t.Error("expected actions in state")
	}
}

func TestLLBConverterBuildArgs(t *testing.T) {
	dockerfile := `FROM ubuntu:20.04
ARG VERSION=1.0.0
ARG PORT=8080
ENV APP_VERSION=${VERSION}
EXPOSE ${PORT}`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		BuildArgs: map[string]string{
			"VERSION": "2.0.0", // Override default
			"PORT":    "9090",  // Override default
		},
	}

	definition, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if definition == nil {
		t.Fatal("expected LLB definition but got nil")
	}

	// Check that build args metadata is included
	if buildArgsData, ok := definition.Metadata["build_args"]; ok {
		buildArgsStr := string(buildArgsData)
		if !strings.Contains(buildArgsStr, "VERSION") {
			t.Error("expected VERSION in build args metadata")
		}
		if !strings.Contains(buildArgsStr, "2.0.0") {
			t.Error("expected overridden VERSION value in metadata")
		}
	} else {
		t.Error("expected build_args in metadata")
	}
}

func TestLLBConverterHealthcheck(t *testing.T) {
	health := &HealthcheckInstruction{
		Type:        "CMD",
		Commands:    []string{"curl", "-f", "http://localhost/"},
		Interval:    "30s",
		Timeout:     "3s",
		StartPeriod: "5s",
		Retries:     3,
	}

	converter := NewLLBConverter()
	currentState := &LLBState{
		State:    map[string]interface{}{"type": "image"},
		Metadata: make(map[string]interface{}),
	}

	newState, err := converter.(*LLBConverterImpl).convertHealthcheckInstruction(health, currentState, nil)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if newState == nil {
		t.Fatal("expected new state but got nil")
	}

	healthSpec, ok := newState.Metadata["healthcheck"].(map[string]interface{})
	if !ok {
		t.Fatal("expected healthcheck in metadata")
	}

	if healthSpec["type"] != "CMD" {
		t.Errorf("expected type CMD, got %v", healthSpec["type"])
	}

	if test, ok := healthSpec["test"].([]string); !ok || len(test) != 3 {
		t.Errorf("expected test command, got %v", healthSpec["test"])
	}

	if healthSpec["interval"] != "30s" {
		t.Errorf("expected interval 30s, got %v", healthSpec["interval"])
	}

	if healthSpec["timeout"] != "3s" {
		t.Errorf("expected timeout 3s, got %v", healthSpec["timeout"])
	}

	if healthSpec["start_period"] != "5s" {
		t.Errorf("expected start_period 5s, got %v", healthSpec["start_period"])
	}

	if healthSpec["retries"] != 3 {
		t.Errorf("expected retries 3, got %v", healthSpec["retries"])
	}
}

func TestLLBConverterErrors(t *testing.T) {
	converter := NewLLBConverter()

	// Test nil AST
	_, err := converter.Convert(nil, nil)
	if err == nil {
		t.Error("expected error for nil AST")
	}

	// Test empty AST
	emptyAST := &AST{Stages: []*Stage{}}
	_, err = converter.Convert(emptyAST, nil)
	if err == nil {
		t.Error("expected error for empty AST")
	}

	// Test invalid target stage
	dockerfile := `FROM ubuntu`
	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	opts := &ConvertOptions{Target: "nonexistent"}
	_, err = converter.Convert(ast, opts)
	if err == nil {
		t.Error("expected error for nonexistent target stage")
	}
}

func TestLLBConverterComplexDockerfile(t *testing.T) {
	dockerfile := `FROM --platform=linux/amd64 node:16-alpine AS base
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --only=production

FROM base AS build
RUN --mount=type=cache,target=/root/.npm npm ci
COPY . .
RUN npm run build

FROM nginx:alpine AS production
COPY --from=build /app/dist /usr/share/nginx/html
COPY --chown=nginx:nginx nginx.conf /etc/nginx/nginx.conf
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD curl -f http://localhost/
LABEL maintainer="test@example.com"
EXPOSE 80 443/tcp
USER nginx
CMD ["nginx", "-g", "daemon off;"]`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Platform: "linux/amd64",
		BuildArgs: map[string]string{
			"NODE_ENV": "production",
		},
	}

	// Convert all stages (default behavior)
	definition, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	if definition == nil {
		t.Fatal("expected LLB definition but got nil")
	}

	// Convert specific stage
	opts.Target = "build"
	buildDefinition, err := converter.Convert(ast, opts)
	if err != nil {
		t.Fatalf("build stage conversion error: %v", err)
	}

	if buildDefinition == nil {
		t.Fatal("expected build stage LLB definition but got nil")
	}

	// Verify metadata is included
	if definition.Metadata == nil {
		t.Error("expected metadata but got nil")
	}

	if len(definition.Definition) == 0 {
		t.Error("expected definition bytes but got empty")
	}
}
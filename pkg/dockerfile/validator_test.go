package dockerfile

import (
	"strings"
	"testing"
)

func TestValidatorBasicValidation(t *testing.T) {
	tests := []struct {
		name        string
		dockerfile  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid simple dockerfile",
			dockerfile: `FROM ubuntu:20.04
RUN apt-get update
COPY . /app
WORKDIR /app
CMD ["./app"]`,
			expectError: false,
		},
		{
			name: "valid multi-stage dockerfile",
			dockerfile: `FROM golang:1.19 AS builder
WORKDIR /src
COPY . .
RUN go build -o app

FROM alpine:3.16
COPY --from=builder /src/app /usr/local/bin/app
CMD ["app"]`,
			expectError: false,
		},
		{
			name:        "dockerfile without FROM",
			dockerfile:  "RUN echo test",
			expectError: true,
			errorMsg:    "instruction RUN found before FROM",
		},
		{
			name: "duplicate stage names",
			dockerfile: `FROM ubuntu AS stage1
FROM alpine AS stage1`,
			expectError: true,
			errorMsg:    "duplicate stage name",
		},
		{
			name: "invalid stage reference",
			dockerfile: `FROM ubuntu
COPY --from=nonexistent /app /app`,
			expectError: true,
			errorMsg:    "unknown stage reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))
			
			if err != nil && !tt.expectError {
				t.Fatalf("unexpected parse error: %v", err)
			}
			
			if err == nil && ast != nil {
				validator := NewValidator()
				err = validator.ValidateAST(ast)
			}

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidatorImageReference(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		image       string
		tag         string
		digest      string
		expectError bool
	}{
		{
			name:        "valid image name",
			image:       "ubuntu",
			expectError: false,
		},
		{
			name:        "valid image with namespace",
			image:       "library/ubuntu",
			expectError: false,
		},
		{
			name:        "valid registry/namespace/image",
			image:       "docker.io/library/ubuntu",
			expectError: false,
		},
		{
			name:        "valid tag",
			image:       "ubuntu",
			tag:         "20.04",
			expectError: false,
		},
		{
			name:        "valid digest",
			image:       "ubuntu",
			digest:      "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			expectError: false,
		},
		{
			name:        "empty image",
			image:       "",
			expectError: true,
		},
		{
			name:        "invalid tag",
			image:       "ubuntu",
			tag:         "invalid@tag",
			expectError: true,
		},
		{
			name:        "invalid digest",
			image:       "ubuntu",
			digest:      "invalid-digest",
			expectError: true,
		},
		{
			name:        "tag too long",
			image:       "ubuntu",
			tag:         strings.Repeat("a", 129),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateImageReference(tt.image, tt.tag, tt.digest)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorPlatform(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		platform    string
		expectError bool
	}{
		{
			name:        "valid os only",
			platform:    "linux",
			expectError: false,
		},
		{
			name:        "valid os/arch",
			platform:    "linux/amd64",
			expectError: false,
		},
		{
			name:        "valid os/arch/variant",
			platform:    "linux/arm/v7",
			expectError: false,
		},
		{
			name:        "invalid os",
			platform:    "invalid/amd64",
			expectError: true,
		},
		{
			name:        "invalid arch",
			platform:    "linux/invalid",
			expectError: true,
		},
		{
			name:        "too many parts",
			platform:    "linux/amd64/v7/extra",
			expectError: true,
		},
		{
			name:        "empty platform",
			platform:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validatePlatform(tt.platform)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorPortFormat(t *testing.T) {
	tests := []struct {
		name        string
		port        string
		expectError bool
	}{
		{
			name:        "valid port number",
			port:        "8080",
			expectError: false,
		},
		{
			name:        "valid port with protocol",
			port:        "8080/tcp",
			expectError: false,
		},
		{
			name:        "valid udp port",
			port:        "53/udp",
			expectError: false,
		},
		{
			name:        "port out of range",
			port:        "70000",
			expectError: true,
		},
		{
			name:        "invalid port number",
			port:        "abc",
			expectError: true,
		},
		{
			name:        "invalid protocol",
			port:        "8080/http",
			expectError: true,
		},
		{
			name:        "port zero",
			port:        "0",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorChownFormat(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		chown       string
		expectError bool
	}{
		{
			name:        "user only",
			chown:       "nginx",
			expectError: false,
		},
		{
			name:        "user and group",
			chown:       "nginx:nginx",
			expectError: false,
		},
		{
			name:        "numeric uid",
			chown:       "1000",
			expectError: false,
		},
		{
			name:        "numeric uid:gid",
			chown:       "1000:1000",
			expectError: false,
		},
		{
			name:        "empty user",
			chown:       "",
			expectError: true,
		},
		{
			name:        "invalid format",
			chown:       "user:group:extra",
			expectError: true,
		},
		{
			name:        "invalid user",
			chown:       "invalid-user-@",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateChownFormat(tt.chown)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorChmodFormat(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		chmod       string
		expectError bool
	}{
		{
			name:        "valid octal 3 digits",
			chmod:       "755",
			expectError: false,
		},
		{
			name:        "valid octal 4 digits",
			chmod:       "0644",
			expectError: false,
		},
		{
			name:        "valid octal leading zero",
			chmod:       "0755",
			expectError: false,
		},
		{
			name:        "invalid decimal",
			chmod:       "999",
			expectError: true,
		},
		{
			name:        "invalid format",
			chmod:       "rwxr-xr-x",
			expectError: true,
		},
		{
			name:        "too short",
			chmod:       "75",
			expectError: true,
		},
		{
			name:        "too long",
			chmod:       "07555",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateChmodFormat(tt.chmod)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorEnvironmentVariableName(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		envVar      string
		expectError bool
	}{
		{
			name:        "valid simple name",
			envVar:      "PATH",
			expectError: false,
		},
		{
			name:        "valid with underscore",
			envVar:      "MY_VAR",
			expectError: false,
		},
		{
			name:        "valid with numbers",
			envVar:      "VAR123",
			expectError: false,
		},
		{
			name:        "valid starting with underscore",
			envVar:      "_PRIVATE",
			expectError: false,
		},
		{
			name:        "invalid starting with number",
			envVar:      "123VAR",
			expectError: true,
		},
		{
			name:        "invalid with hyphen",
			envVar:      "MY-VAR",
			expectError: true,
		},
		{
			name:        "invalid with special chars",
			envVar:      "MY@VAR",
			expectError: true,
		},
		{
			name:        "empty name",
			envVar:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateEnvironmentVariableName(tt.envVar)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorLabelKey(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		labelKey    string
		expectError bool
	}{
		{
			name:        "simple key",
			labelKey:    "version",
			expectError: false,
		},
		{
			name:        "reverse dns key",
			labelKey:    "com.example.version",
			expectError: false,
		},
		{
			name:        "key with hyphen",
			labelKey:    "app-version",
			expectError: false,
		},
		{
			name:        "key with underscore",
			labelKey:    "app_version",
			expectError: false,
		},
		{
			name:        "empty key",
			labelKey:    "",
			expectError: true,
		},
		{
			name:        "key starting with number",
			labelKey:    "1version",
			expectError: true,
		},
		{
			name:        "invalid reverse dns",
			labelKey:    "com..example.version",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateLabelKey(tt.labelKey)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorDuration(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		duration    string
		expectError bool
	}{
		{
			name:        "valid seconds",
			duration:    "30s",
			expectError: false,
		},
		{
			name:        "valid minutes",
			duration:    "5m",
			expectError: false,
		},
		{
			name:        "valid hours",
			duration:    "1h",
			expectError: false,
		},
		{
			name:        "invalid unit",
			duration:    "30d",
			expectError: true,
		},
		{
			name:        "no unit",
			duration:    "30",
			expectError: true,
		},
		{
			name:        "invalid number",
			duration:    "abc",
			expectError: true,
		},
		{
			name:        "empty duration",
			duration:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateDuration(tt.duration)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorSignalFormat(t *testing.T) {
	validator := NewValidator()
	
	tests := []struct {
		name        string
		signal      string
		expectError bool
	}{
		{
			name:        "valid signal name",
			signal:      "SIGTERM",
			expectError: false,
		},
		{
			name:        "valid signal number",
			signal:      "15",
			expectError: false,
		},
		{
			name:        "another valid signal",
			signal:      "SIGKILL",
			expectError: false,
		},
		{
			name:        "invalid signal name",
			signal:      "INVALID",
			expectError: true,
		},
		{
			name:        "invalid format",
			signal:      "sig15",
			expectError: true,
		},
		{
			name:        "empty signal",
			signal:      "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateSignalFormat(tt.signal)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatorInstructionOccurrences(t *testing.T) {
	tests := []struct {
		name        string
		dockerfile  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "single CMD",
			dockerfile: `FROM ubuntu
CMD ["echo", "test"]`,
			expectError: false,
		},
		{
			name: "multiple CMD instructions",
			dockerfile: `FROM ubuntu
CMD ["echo", "test1"]
CMD ["echo", "test2"]`,
			expectError: true,
			errorMsg:    "CMD can only appear once",
		},
		{
			name: "multiple ENTRYPOINT instructions",
			dockerfile: `FROM ubuntu
ENTRYPOINT ["echo"]
ENTRYPOINT ["cat"]`,
			expectError: true,
			errorMsg:    "ENTRYPOINT can only appear once",
		},
		{
			name: "multiple HEALTHCHECK instructions",
			dockerfile: `FROM ubuntu
HEALTHCHECK CMD curl -f http://localhost/
HEALTHCHECK CMD curl -f http://localhost/health`,
			expectError: true,
			errorMsg:    "HEALTHCHECK can only appear once",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			validator := NewValidator()
			err = validator.ValidateAST(ast)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidatorOnbuildRestrictions(t *testing.T) {
	tests := []struct {
		name        string
		dockerfile  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid ONBUILD RUN",
			dockerfile: `FROM ubuntu
ONBUILD RUN apt-get update`,
			expectError: false,
		},
		{
			name: "valid ONBUILD COPY",
			dockerfile: `FROM ubuntu
ONBUILD COPY . /app`,
			expectError: false,
		},
		{
			name: "invalid ONBUILD FROM",
			dockerfile: `FROM ubuntu
ONBUILD FROM alpine`,
			expectError: true,
			errorMsg:    "ONBUILD instruction cannot contain FROM",
		},
		{
			name: "invalid ONBUILD ONBUILD",
			dockerfile: `FROM ubuntu
ONBUILD ONBUILD RUN echo test`,
			expectError: true,
			errorMsg:    "ONBUILD instruction cannot contain ONBUILD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := New()
			ast, err := parser.Parse(strings.NewReader(tt.dockerfile))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}

			validator := NewValidator()
			err = validator.ValidateAST(ast)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestValidatorComplexDockerfile(t *testing.T) {
	dockerfile := `FROM --platform=linux/amd64 node:16-alpine AS base
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production

FROM base AS build
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine AS production
COPY --from=build /app/dist /usr/share/nginx/html
COPY --chown=nginx:nginx nginx.conf /etc/nginx/nginx.conf
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost/ || exit 1
LABEL maintainer="test@example.com"
LABEL version="1.0.0"
EXPOSE 80 443/tcp
USER nginx
STOPSIGNAL SIGTERM
CMD ["nginx", "-g", "daemon off;"]`

	parser := New()
	ast, err := parser.Parse(strings.NewReader(dockerfile))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	validator := NewValidator()
	err = validator.ValidateAST(ast)
	if err != nil {
		t.Errorf("validation error: %v", err)
	}

	// Additional checks
	if len(ast.Stages) != 3 {
		t.Errorf("expected 3 stages, got %d", len(ast.Stages))
	}

	stageNames := []string{"base", "build", "production"}
	for i, expectedName := range stageNames {
		if i < len(ast.Stages) && ast.Stages[i].Name != expectedName {
			t.Errorf("expected stage %d name %q, got %q", i, expectedName, ast.Stages[i].Name)
		}
	}
}
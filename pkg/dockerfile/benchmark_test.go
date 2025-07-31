package dockerfile

import (
	"strings"
	"testing"
)

const simpleDockerfile = `FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl
COPY . /app
WORKDIR /app
EXPOSE 8080
CMD ["./app"]`

const complexDockerfile = `# syntax=docker/dockerfile:1.4
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
LABEL maintainer="test@example.com" \
      version="1.0.0" \
      description="Multi-stage Node.js application"

EXPOSE 80 443/tcp
USER nginx
STOPSIGNAL SIGTERM

CMD ["nginx", "-g", "daemon off;"]`

const realWorldDockerfile = `# syntax=docker/dockerfile:1.4

# Build dependencies
FROM ubuntu:20.04 AS build-deps
RUN apt-get update && apt-get install -y \
    build-essential \
    curl \
    git \
    python3 \
    python3-pip \
    nodejs \
    npm

# Python dependencies
FROM build-deps AS python-deps
WORKDIR /python-deps
COPY requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip \
    pip3 install --user -r requirements.txt

# Node.js dependencies
FROM build-deps AS node-deps
WORKDIR /node-deps
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --only=production

# Build application
FROM build-deps AS builder
WORKDIR /src

# Copy Python dependencies
COPY --from=python-deps /root/.local /root/.local
ENV PATH=/root/.local/bin:$PATH

# Copy Node.js dependencies
COPY --from=node-deps /node-deps/node_modules ./node_modules

# Copy source code
COPY . .

# Build application
RUN make build
RUN make test

# Runtime image
FROM ubuntu:20.04 AS runtime
RUN apt-get update && apt-get install -y \
    python3 \
    python3-distutils \
    nodejs \
    nginx \
    supervisor \
    && rm -rf /var/lib/apt/lists/*

# Create app user
RUN useradd -m -u 1000 appuser

# Copy application
COPY --from=builder --chown=appuser:appuser /src/dist /app
COPY --from=python-deps --chown=appuser:appuser /root/.local /home/appuser/.local

# Copy configuration
COPY --chown=appuser:appuser config/nginx.conf /etc/nginx/nginx.conf
COPY --chown=appuser:appuser config/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# Set environment
ENV PATH=/home/appuser/.local/bin:$PATH
ENV NODE_ENV=production
ENV PYTHONPATH=/app

WORKDIR /app
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Expose ports
EXPOSE 8080 8443

# Metadata
LABEL maintainer="devops@example.com" \
      version="2.1.0" \
      description="Multi-language web application" \
      org.opencontainers.image.title="MyApp" \
      org.opencontainers.image.description="Production web application" \
      org.opencontainers.image.vendor="Example Corp" \
      org.opencontainers.image.licenses="MIT"

# Start application
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/conf.d/supervisord.conf"]`

func BenchmarkLexerSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		lexer, err := NewLexer(strings.NewReader(simpleDockerfile))
		if err != nil {
			b.Fatal(err)
		}
		_, err = lexer.TokenizeAll()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLexerComplex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		lexer, err := NewLexer(strings.NewReader(complexDockerfile))
		if err != nil {
			b.Fatal(err)
		}
		_, err = lexer.TokenizeAll()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLexerRealWorld(b *testing.B) {
	for i := 0; i < b.N; i++ {
		lexer, err := NewLexer(strings.NewReader(realWorldDockerfile))
		if err != nil {
			b.Fatal(err)
		}
		_, err = lexer.TokenizeAll()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(simpleDockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserComplex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(complexDockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserRealWorld(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(realWorldDockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidatorSimple(b *testing.B) {
	parser := New()
	ast, err := parser.Parse(strings.NewReader(simpleDockerfile))
	if err != nil {
		b.Fatal(err)
	}

	validator := NewValidator()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateAST(ast)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidatorComplex(b *testing.B) {
	parser := New()
	ast, err := parser.Parse(strings.NewReader(complexDockerfile))
	if err != nil {
		b.Fatal(err)
	}

	validator := NewValidator()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateAST(ast)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidatorRealWorld(b *testing.B) {
	parser := New()
	ast, err := parser.Parse(strings.NewReader(realWorldDockerfile))
	if err != nil {
		b.Fatal(err)
	}

	validator := NewValidator()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateAST(ast)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLLBConverterSimple(b *testing.B) {
	parser := New()
	ast, err := parser.Parse(strings.NewReader(simpleDockerfile))
	if err != nil {
		b.Fatal(err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Platform: "linux/amd64",
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := converter.Convert(ast, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLLBConverterComplex(b *testing.B) {
	parser := New()
	ast, err := parser.Parse(strings.NewReader(complexDockerfile))
	if err != nil {
		b.Fatal(err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Platform: "linux/amd64",
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := converter.Convert(ast, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLLBConverterRealWorld(b *testing.B) {
	parser := New()
	ast, err := parser.Parse(strings.NewReader(realWorldDockerfile))
	if err != nil {
		b.Fatal(err)
	}

	converter := NewLLBConverter()
	opts := &ConvertOptions{
		Platform: "linux/amd64",
		BuildArgs: map[string]string{
			"NODE_ENV": "production",
			"VERSION":  "2.1.0",
		},
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := converter.Convert(ast, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFullPipelineSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Parse
		parser := New()
		ast, err := parser.Parse(strings.NewReader(simpleDockerfile))
		if err != nil {
			b.Fatal(err)
		}

		// Validate
		validator := NewValidator()
		err = validator.ValidateAST(ast)
		if err != nil {
			b.Fatal(err)
		}

		// Convert to LLB
		converter := NewLLBConverter()
		opts := &ConvertOptions{
			Platform: "linux/amd64",
		}
		_, err = converter.Convert(ast, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFullPipelineComplex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Parse
		parser := New()
		ast, err := parser.Parse(strings.NewReader(complexDockerfile))
		if err != nil {
			b.Fatal(err)
		}

		// Validate
		validator := NewValidator()
		err = validator.ValidateAST(ast)
		if err != nil {
			b.Fatal(err)
		}

		// Convert to LLB
		converter := NewLLBConverter()
		opts := &ConvertOptions{
			Platform: "linux/amd64",
		}
		_, err = converter.Convert(ast, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFullPipelineRealWorld(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Parse
		parser := New()
		ast, err := parser.Parse(strings.NewReader(realWorldDockerfile))
		if err != nil {
			b.Fatal(err)
		}

		// Validate
		validator := NewValidator()
		err = validator.ValidateAST(ast)
		if err != nil {
			b.Fatal(err)
		}

		// Convert to LLB
		converter := NewLLBConverter()
		opts := &ConvertOptions{
			Platform: "linux/amd64",
			BuildArgs: map[string]string{
				"NODE_ENV": "production",
				"VERSION":  "2.1.0",
			},
		}
		_, err = converter.Convert(ast, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Memory allocation benchmarks
func BenchmarkParserMemorySimple(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(simpleDockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserMemoryComplex(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(complexDockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserMemoryRealWorld(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(realWorldDockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Scalability tests with varying sizes
func BenchmarkParserScaling10Lines(b *testing.B) {
	dockerfile := generateDockerfile(10)
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(dockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserScaling100Lines(b *testing.B) {
	dockerfile := generateDockerfile(100)
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(dockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParserScaling1000Lines(b *testing.B) {
	dockerfile := generateDockerfile(1000)
	for i := 0; i < b.N; i++ {
		parser := New()
		_, err := parser.Parse(strings.NewReader(dockerfile))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to generate Dockerfiles of varying sizes
func generateDockerfile(lines int) string {
	var builder strings.Builder
	builder.WriteString("FROM ubuntu:20.04\n")
	
	for i := 1; i < lines; i++ {
		switch i % 6 {
		case 0:
			builder.WriteString("RUN apt-get update\n")
		case 1:
			builder.WriteString("ENV VAR" + string(rune('0'+i%10)) + "=value" + string(rune('0'+i%10)) + "\n")
		case 2:
			builder.WriteString("COPY file" + string(rune('0'+i%10)) + ".txt /app/\n")
		case 3:
			builder.WriteString("WORKDIR /app/dir" + string(rune('0'+i%10)) + "\n")
		case 4:
			builder.WriteString("EXPOSE " + string(rune('0'+8000+i%1000)) + "\n")
		case 5:
			builder.WriteString("LABEL key" + string(rune('0'+i%10)) + "=value" + string(rune('0'+i%10)) + "\n")
		}
	}
	
	builder.WriteString("CMD [\"./app\"]\n")
	return builder.String()
}

// Concurrent parsing benchmark
func BenchmarkParserConcurrent(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			parser := New()
			_, err := parser.Parse(strings.NewReader(complexDockerfile))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
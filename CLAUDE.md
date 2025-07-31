# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the **shmocker** project - a rootless Docker-image builder designed as a secure, license-free, drop-in replacement for `docker build`. The project aims to build OCI images from Dockerfiles without requiring Docker, root privileges, or adding operational complexity.

## Development Workflow

### Agent Orchestration

You are the orchestrator and should coordinate between specialized expert agents:

1. **Architect Agent** - Controls overall flow, interfaces, and model definitions
2. **Layer-Specific Agents** - One for each application layer requiring specific expertise
3. **QA Agent** - Breaks down requirements into testable elements, finds faults, controls test scenarios
4. **DevOps Agent** - Sets up project files, build pipelines, test harnesses
5. **Strategic Adviser** - Helps decide between alternative paths
6. **Security & Compliance Auditor** - Performs code reviews, ensures security and regulatory compliance

### Version Control

Create a commit immediately after completing each task to maintain a clear development journal. Use descriptive commit messages that explain what was accomplished.

## Technical Requirements

### Core Goals
- **G-1**: Build any existing Dockerfile unmodified
- **G-2**: Match or beat Docker Buildx performance (≤110% cold cache, ≤105% warm cache)
- **G-3**: Distribute as single static Linux binary (amd64 & arm64)
- **G-4**: Run rootless (no CAP_SYS_ADMIN or privileged pod required)
- **G-5**: Provide SBOM + Cosign signature out-of-the-box

### Key Technical Decisions
- **Language**: Go (for BuildKit ecosystem compatibility and static binaries)
- **Build Engine**: Embed BuildKit as library (in-process)
- **Execution**: Rootless OCI worker
- **Distribution**: Static musl binary + GitHub release
- **Output**: OCI Image v1.1 format
- **Supply Chain**: Syft SBOM + Sigstore Cosign signature

### Architecture Components
```
shmocker (single binary)
 ├─ Dockerfile frontend → AST → LLB
 ├─ BuildKit controller (in-process)
 │    └─ rootless OCI worker (overlayfs snapshotter, runc executor)
 ├─ Content store (local or cache dir)
 ├─ Image assembler → OCI manifest
 ├─ SBOM generator (Syft lib call)
 ├─ Cosign signer
 └─ Registry client (OCI dist v1)
```

## Build Commands

Once the project is set up:
```bash
# Build the binary
go build -o shmocker -ldflags="-s -w" -tags netgo,osusergo ./cmd/shmocker

# Run tests
go test ./...

# Run specific test
go test -run TestName ./pkg/...

# Lint (once configured)
golangci-lint run

# Security checks
gosec ./...
govulncheck ./...
```

## Testing Strategy

1. **Static Analysis**: `go vet`, `staticcheck`, `gosec`, `govulncheck`
2. **Unit Tests**: Table-driven parser & flag tests
3. **Golden Tests**: LLB JSON diff for known Dockerfiles
4. **Fuzz Testing**: Random Dockerfile tokens to parser
5. **Integration Tests**: Build docker-samples repo matrix
6. **Performance Tests**: Benchmark suite with 10% regression threshold
7. **Chaos Tests**: SIGTERM, registry 503, disk-full scenarios
8. **Security Tests**: OPA/Gatekeeper policy compliance

## Development Milestones

- **M-0**: Repository bootstrap, CI skeleton, static build POC
- **M-1**: Dockerfile parser + single-stage build, local tar output
- **M-2**: Registry push, cache import/export
- **M-3**: Multi-stage, multi-arch, rootless implementation
- **M-4**: SBOM + Cosign integration, security audit
- **M-5**: Beta rollout to pilot teams
- **GA**: Company-wide adoption

## Important Constraints

- Must parse full Dockerfile grammar up to Docker 24 syntax
- Support for `--cache-from`, `--cache-to`, `--build-arg`, `--target`, `--platform`
- JSON progress stream output for CI dashboards
- Exit with non-zero on errors with detailed propagation
- Structured logging in logfmt format
- Trace export for debugging
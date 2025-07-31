# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is the **shmocker** project - a rootless Docker-image builder designed as a secure, license-free, drop-in replacement for `docker build`. The project aims to build OCI images from Dockerfiles without requiring Docker, root privileges, or adding operational complexity.

## Development Workflow

### Agent Orchestration

You are the orchestrator and should coordinate between specialized expert agents.

#### Orchestrator Responsibilities:
1. **Break down work into atomic tasks** before assigning to agents
2. **Instruct each agent** to commit after each distinct unit of work
3. **Review agent work** and ensure proper commit granularity
4. **Example agent instruction**:
   ```
   "Please implement the Dockerfile parser. Break this into:
   1. Create lexer.go and commit
   2. Implement tokenization logic and commit
   3. Create parser.go and commit
   4. Implement parsing logic and commit
   5. Create tests for lexer and commit
   6. Create tests for parser and commit
   
   Each step should be a separate commit with a descriptive message."
   ```

Specialized expert agents:

1. **Architect Agent** - Controls overall flow, interfaces, and model definitions
2. **Layer-Specific Agents** - One for each application layer requiring specific expertise
3. **QA Agent** - Breaks down requirements into testable elements, finds faults, controls test scenarios
4. **DevOps Agent** - Sets up project files, build pipelines, test harnesses
5. **Strategic Adviser** - Helps decide between alternative paths
6. **Security & Compliance Auditor** - Performs code reviews, ensures security and regulatory compliance

### Version Control

**CRITICAL**: Each agent MUST commit after EVERY distinct unit of work. This creates a clear development journal with atomic, revertible changes.

#### Commit Guidelines for All Agents:
1. **One Task = One Commit**: Each distinct task gets its own commit
   - Creating a new file → commit
   - Implementing a function/interface → commit
   - Fixing a bug → commit
   - Adding tests → commit
   - Updating documentation → commit

2. **Commit Message Format**:
   ```
   type(scope): brief description
   
   - Detailed bullet points if needed
   - What was changed and why
   ```
   Types: feat, fix, docs, test, refactor, style, chore

3. **Examples of Distinct Units**:
   - Creating go.mod → `chore: Initialize Go module`
   - Creating directory structure → `feat: Add project directory structure`
   - Implementing parser lexer → `feat(parser): Implement lexer tokenization`
   - Adding lexer tests → `test(parser): Add lexer unit tests`
   - Fixing parser bug → `fix(parser): Handle line continuations correctly`

4. **Agent Instructions Template**:
   When creating an agent, include:
   ```
   IMPORTANT: You MUST commit after completing each distinct task:
   - Use 'git add <specific-files>' for the files you just created/modified
   - Use descriptive commit messages following conventional commits
   - One logical change per commit (don't bundle unrelated changes)
   - If you create 5 files for different purposes, that might be 5 commits
   ```

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
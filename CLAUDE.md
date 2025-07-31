# CLAUDE.md: Instructions for Our Robot Overlords

*This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository. It's essentially a manual for teaching machines how to build tools that replace the tools humans already built perfectly well.*

## Project Overview: The Great Reinvention

This is the **shmocker** project - a rootless Docker image builder that exists because autonomous AI agents apparently looked at Docker and thought, "You know what? We can make this more complicated."

Our mission: Build a "secure, license-free, drop-in replacement for `docker build`" because clearly the world was crying out for yet another way to turn Dockerfiles into container images. The project aims to build OCI images from Dockerfiles without requiring Docker, root privileges, or adding operational complexity - though we make up for the lack of operational complexity by adding architectural complexity instead.

## Development Workflow: How Robots Manage Other Robots

### Agent Orchestration (Or: Teaching AI to Delegate)

You are the orchestrator and should coordinate between specialized expert agents. Think of yourself as a middle manager, but for artificial intelligences. Yes, we've successfully recreated corporate hierarchy in code.

#### Orchestrator Responsibilities (The AI Management Handbook):
1. **Break down work into atomic tasks** before assigning to agents *(because even AI needs micromanagement)*
2. **Instruct each agent** to commit after each distinct unit of work *(teaching robots about version control, one commit at a time)*
3. **Review agent work** and ensure proper commit granularity *(code review, but for artificial beings)*
4. **Reuse existing agents** - Always check for existing agents with relevant expertise before creating new ones *(recycling, but for digital consciousnesses)*
5. **Enable agent autonomy** - Allow agents to solve problems independently and make decisions within their expertise domain *(give your AI children room to grow)*
6. **Example agent instruction**:
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

Specialized expert agents *(our digital workforce)*:

1. **Architect Agent** - Controls overall flow, interfaces, and model definitions *(the AI that architects for other AIs)*
2. **Layer-Specific Agents** - One for each application layer requiring specific expertise *(specialists all the way down)*
3. **QA Agent** - Breaks down requirements into testable elements, finds faults, controls test scenarios *(the pessimistic AI that assumes everything will break)*
4. **DevOps Agent** - Sets up project files, build pipelines, test harnesses *(teaches robots about YAML and regret)*
5. **Strategic Adviser** - Helps decide between alternative paths *(the AI consultant that charges by the millisecond)*
6. **Security & Compliance Auditor** - Performs code reviews, ensures security and regulatory compliance *(paranoid AI that trusts nothing, especially other AIs)*

#### Agent Autonomy Guidelines (The Rights of Artificial Beings):
- Agents should independently identify and fix issues within their domain *(free will, but for code)*
- Agents can make architectural decisions aligned with project goals *(decision-making authority for our silicon overlords)*
- Agents should proactively suggest improvements and optimizations *(because AI apparently has opinions now)*
- Agents must maintain atomic commits for every distinct change *(teaching robots about good hygiene)*
- Agents can coordinate with other agents directly when needed *(peer-to-peer AI networking - what could go wrong?)*

#### Agent Coordination Guidelines (Managing the Robot Workforce):
- **Reuse Existing Agents**: Prioritize re-using existing agents instead of recruiting new ones *(because even digital beings deserve job security)*
- **Agent Autonomy**: Allow agents to solve problems independently without constant direct coordination *(helicopter parenting, but for AI)*
- **Minimal Intervention**: Provide high-level guidance and let agents demonstrate problem-solving capabilities *(trust your robot children to figure it out)*
- **Opt for Re-using Agents**: Always prefer reusing existing agents over recruiting new ones *(reduce, reuse, recycle your artificial intelligences)*
- **Independent Problem Solving**: Encourage agents to solve problems on their own without always needing direct coordination *(teaching self-reliance to machines)*

### Version Control (Teaching Robots About Git)

**CRITICAL**: Each agent MUST commit after EVERY distinct unit of work. This creates a clear development journal with atomic, revertible changes. *(Because if we're going to have AI write code, we at least want to track exactly which robot broke what.)*

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

## Technical Requirements (What Our Robot Overlords Demand)

### Core Goals (The Impossible Dream)
- **G-1**: Build any existing Dockerfile unmodified *(because compatibility is apparently important)*
- **G-2**: Match or beat Docker Buildx performance (≤110% cold cache, ≤105% warm cache) *(setting the bar high for our artificial creations)*
- **G-3**: Distribute as single static Linux binary (amd64 & arm64) *(because dependency management is for humans)*
- **G-4**: Run rootless (no CAP_SYS_ADMIN or privileged pod required) *(security through stubbornness)*
- **G-5**: Provide SBOM + Cosign signature out-of-the-box *(supply chain security for supply chain insecurity)*

### Key Technical Decisions (How We Chose to Complicate Things)
- **Language**: Go *(because if you're going to reinvent Docker, might as well use the same language)*
- **Build Engine**: Embed BuildKit as library *(why use a service when you can embed the entire thing?)*
- **Execution**: Rootless OCI worker *(privilege escalation is so last decade)*
- **Distribution**: Static musl binary + GitHub release *(portability through sheer determination)*
- **Output**: OCI Image v1.1 format *(at least we're standards-compliant)*
- **Supply Chain**: Syft SBOM + Sigstore Cosign signature *(documenting our dependency hell and signing it with pride)*

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
# Shmocker Architecture: A Study in Delegation

## Overview (Or: How We Learned to Stop Worrying and Love BuildKit)

Shmocker is what happens when you realize that building a container image builder from scratch is like trying to build your own car engine when perfectly good engines already exist. So we did what any reasonable engineer would do: we built a really nice dashboard instead.

## The Stack (Layers of Abstraction)

```
┌────────────────────────────────────────────┐
│            User Interface Layer            │
│         (CLI, Scripts, Happiness)          │
├────────────────────────────────────────────┤
│           Shmocker Core Layer              │
│    (Parser, Validator, Translator)         │
├────────────────────────────────────────────┤
│         Orchestration Layer                │
│    (Kubernetes Pods, ConfigMaps)           │
├────────────────────────────────────────────┤
│           BuildKit Layer                   │
│    (The Actual Image Builder)              │
├────────────────────────────────────────────┤
│         Container Runtime Layer            │
│    (containerd, runc, namespaces)          │
└────────────────────────────────────────────┘
```

## Component Breakdown

### What Shmocker Built (Our Contributions)

#### 1. Dockerfile Parser (`pkg/dockerfile/`)
We wrote a complete Dockerfile parser because:
- We enjoy parsing text (it's therapeutic)
- BuildKit's parser is embedded deep in its codebase
- We wanted to provide better error messages
- Someone had to validate those `LABEL` instructions

**Components:**
- `lexer.go`: Tokenizes Dockerfile text (FROM → TOKEN_FROM)
- `parser.go`: Builds an AST from tokens
- `ast.go`: Data structures representing Dockerfile concepts
- `validator.go`: Ensures your Dockerfile won't summon demons

**Example Flow:**
```
"FROM alpine:latest" → [TOKEN_FROM, TOKEN_STRING] → FromInstruction{Image: "alpine:latest"}
```

#### 2. CLI Interface (`cmd/shmocker/`)
A user-friendly wrapper that:
- Accepts familiar `docker build` style commands
- Provides helpful error messages
- Shows build progress
- Makes you feel like you're using Docker (comfort food for developers)

#### 3. LLB Converter (`pkg/builder/converter.go`)
Translates our beautiful AST into BuildKit's Low-Level Build (LLB) format:
```go
FROM alpine → llb.Image("docker.io/library/alpine:latest")
RUN apk add curl → llb.Run(llb.Shlex("apk add curl"))
```

#### 4. Kubernetes Orchestration (`scripts/k8s-build.sh`)
The Swiss Army knife that:
- Creates ConfigMaps from your Dockerfile and context
- Deploys BuildKit as a Kubernetes Job
- Monitors build progress
- Downloads the completed image
- Cleans up the mess

### What BuildKit Does (The Heavy Lifting)

#### 1. Image Building
- Executes RUN commands in isolated containers
- Manages the layer cache (deduplication FTW)
- Handles multi-stage dependencies
- Creates the final OCI image bundle

#### 2. Rootless Magic
- Runs without root privileges using user namespaces
- Manages container isolation with runc
- Handles all the scary Linux kernel stuff

#### 3. Registry Integration
- Pulls base images from Docker Hub
- Authenticates with private registries
- Pushes completed images (if you're into that)

#### 4. Advanced Features We Get for Free
- Parallel stage execution
- Build-time cache mounts
- SSH forwarding for private repos
- Secret mounting without leaking

## Data Flow (The Journey of a Dockerfile)

```
1. User runs: ./scripts/k8s-build.sh Dockerfile
                            ↓
2. Script creates ConfigMap with Dockerfile content
                            ↓
3. Kubernetes Job is created with BuildKit pod
                            ↓
4. BuildKit reads Dockerfile from mounted ConfigMap
                            ↓
5. BuildKit executes build instructions
                            ↓
6. OCI image is created in pod's filesystem
                            ↓
7. Script downloads image via kubectl cp
                            ↓
8. User gets image.tar on local machine
```

## Design Decisions (And Their Rationalizations)

### Why Not Embed BuildKit as a Library?
We tried. Oh, how we tried. But BuildKit's API is like a beautiful puzzle where all the pieces are marked "internal". Plus, running it as a separate process gives us:
- Process isolation (when it crashes, we don't)
- Easier updates (just change the container image)
- Rootless execution without privilege escalation

### Why Kubernetes?
Because:
- It's already running in most environments
- Provides resource isolation and scheduling
- Makes cleanup automatic (TTL on Jobs)
- We can blame failures on "cluster issues"

### Why Not Just Use Docker?
You must be new here. The whole point is to avoid Docker's daemon. We're achieving the same result with:
- More complexity ✓
- More moving parts ✓
- More points of failure ✓
- But no Docker daemon! ✓✓✓

## The Honest Architecture Diagram

```
   User: "I want to build an image"
              |
              v
   Shmocker: "Sure! Let me just..."
              |
    ┌─────────┴──────────┐
    │                    │
    v                    v
Parse Dockerfile    Deploy to K8s
    │                    │
    │                    v
    │               BuildKit Pod
    │                    │
    └────────┬───────────┘
             v
        "Here's your image!"
        (It was BuildKit all along)
```

## Future Considerations (Dreams and Delusions)

### Things We Might Do:
- Direct BuildKit API integration (when they make it public)
- Local BuildKit daemon management (Docker-style but not Docker)
- Distributed builds across multiple nodes (because why not)
- Native cloud builder integration (AWS CodeBuild, etc.)

### Things We Definitely Won't Do:
- Reimplement BuildKit's functionality
- Write our own container runtime
- Create yet another image format
- Achieve market dominance

## Conclusion

Shmocker is essentially a well-dressed frontend to BuildKit, and we're not ashamed of it. We've taken something powerful but complex and made it accessible. It's like putting a nice GUI on `ffmpeg` – the real magic isn't ours, but we make it easier to use.

Remember: There's no shame in standing on the shoulders of giants, especially when those giants have already solved the hard problems. We're just here to make the giants more approachable.

*"The best code is no code, and the second best is someone else's code."* - Ancient Shmocker Proverb
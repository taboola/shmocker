# Shmocker: Because reinventing Docker is easier than reading the docs

*A rootless Docker image builder crafted by autonomous AI agents in an experiment to test how far they could go. Spoiler: Pretty far.*

## The Magnificent Problem I Was Solving (Or: How This Experiment Began)

This project started as two experiments that collided spectacularly:

**Experiment 1: How Far Can Autonomous AI Agents Go?**  
I wanted to test the limits of AI agent autonomy. Could they design, architect, and implement a complex system with minimal human intervention? (Spoiler: They can, and they will, even when it's a terrible idea.)

**Experiment 2: Building a Docker Alternative**  
I heard Docker Desktop was now paid and thought "Docker isn't free anymore!" My righteous indignation was matched only by my complete misunderstanding that:
- Docker CLI is still open source (always was, always will be)
- Docker Desktop ≠ Docker  
- Building images was never the paid part

By the time I realized my mistake, the agents had already:
- Created a complete architecture
- Implemented a Dockerfile parser
- Set up Kubernetes integrations
- Written more documentation than most real projects

I could have stopped them. I should have stopped them. But watching autonomous agents build a Docker replacement out of spite (misguided spite!) was too fascinating to halt. It's like watching a Rube Goldberg machine—you know it's overcomplicated, but you can't look away.

And so, Shmocker lives—a testament to what happens when human misunderstanding meets AI determination.

## What Shmocker Actually Is (An Architectural Confession)

Let's be honest: Shmocker isn't trying to replace the wheel. My agents and I are more like someone who looked at a perfectly good wheel and said, "What if we gave it a better user interface?"

### The Real Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│     You     │────▶│   Shmocker   │────▶│  BuildKit   │
│ (Dockerfile)│     │  (Translator) │     │  (Builder)  │
└─────────────┘     └──────────────┘     └─────────────┘
                            │                     │
                            ▼                     ▼
                     ┌──────────────┐      ┌───────────┐
                     │  Kubernetes  │      │ OCI Image │
                     │(Orchestrator)│      │ (Output)  │
                     └──────────────┘      └───────────┘
```

### Division of Labor (Or: What We Actually Built vs. What We Borrowed)

**What Shmocker Does (Our Actual Code):**
- 📖 **Dockerfile Parser**: A lovingly hand-crafted lexer and parser that understands all 47 flavors of Dockerfile syntax (Docker doesn't help with this part)
- 🎯 **CLI Interface**: Because `shmocker build` sounds cooler than `buildctl build --frontend dockerfile.v0 --local context=.` (Docker is notably absent from this command)
- 🔄 **Translation Layer**: Converts our parsed AST into BuildKit's LLB format (Docker watches from the sidelines, confused)
- 🎼 **Kubernetes Orchestration**: Manages the intricate dance of ConfigMaps, Pods, and hope (Docker Desktop would charge for this, probably)

**What BuildKit Does (The Actual Magic):**
- 🏗️ **Image Building**: The real hero that executes RUN commands, manages layers, and makes containers happen (ironically, from the Docker project)
- 🔒 **Rootless Execution**: All the namespace and cgroup wizardry that makes security folks happy (Docker daemon weeps in privileged mode)
- 📦 **Registry Operations**: Pulls base images, pushes results, handles auth (Docker Hub still involved, awkwardly)
- ⚡ **Intelligent Caching**: Makes rebuilds fast because waiting is for chumps (Docker's like "hey, I invented that!")

**What Docker Does in Our Project:**
- 🦗 **Absolutely Nothing**: Docker sits in the corner, wondering why we're so mad at it
- 😢 **Provides BuildKit**: Oh wait, that's actually pretty important
- 🤷 **Exists**: Continues being open source while we spite it for no reason

### In Restaurant Terms

Think of it this way:
- **You**: The customer with a recipe (Dockerfile)
- **Shmocker**: The waiter who insists they don't work for "that restaurant" while wearing their uniform
- **BuildKit**: The master chef who actually cooks your meal (trained at Docker Culinary Institute)
- **Kubernetes**: The restaurant building (which we don't own but act like we do)
- **Docker**: The restaurant owner we're boycotting while using their kitchen, recipes, and ingredients

I'm essentially running a food truck in Docker's parking lot, using their suppliers, following their recipes, but putting my own logo on the napkins. When customers ask "Isn't this just Docker?" I reply "No! This is a *rootless* dining experience! Totally different!"

I'm not claiming to be Gordon Ramsay here. I'm more like that friend who "invented" a new recipe by adding salt to someone else's dish.

## Features (Or: Things Docker Already Does, But Now With More Steps)

- **Rootless container image building** - Because sudo is apparently too mainstream
- **OCI-compliant image output** - We promise our containers work just like real ones
- **SBOM generation** - Now you can know exactly which vulnerabilities you're shipping
- **Image signing with Cosign** - Cryptographically prove this madness came from us
- **Multi-stage build support** - All the complexity of Docker, none of the ecosystem
- **Build caching** - We cache things! Just like that other tool you already use

## Project Structure (A Monument to Over-Engineering)

```
shmocker/
├── cmd/shmocker/           # The main event (all 3 users will love it)
├── pkg/                    # Public packages (as if anyone will import these)
│   ├── builder/           # Core build logic (reinventing the wheel)
│   ├── dockerfile/        # Dockerfile parser (because Docker's wasn't good enough)
│   ├── registry/          # OCI registry client (Docker Hub compatibility sold separately)
│   ├── sbom/             # SBOM generation (for when you need to document your mistakes)
│   └── signing/          # Cosign integration (trust, but verify our poor life choices)
├── internal/              # Private packages (where the real magic happens)
│   ├── config/           # Configuration management (YAML files, the root of all evil)
│   └── cache/            # Build cache (faster failures!)
├── .github/workflows/     # CI/CD (robots building tools for robots)
├── Makefile              # Build automation (make all your problems)
├── Dockerfile            # Container image definition (the irony is not lost on us)
└── go.mod                # Go module definition (dependency hell, here we come!)
```

## Building (Or: How to Compile Your Own Disappointment)

### Prerequisites

- Go 1.21 or later (because staying current is for try-hards)
- Make (the tool that makes other tools)
- An inexplicable desire to avoid using Docker to build Docker images
- **macOS Users**: Lima VM (don't worry, we automated this because reading Lima docs is harder than reinventing Docker)

### Build Commands

```bash
# Build for local development (embrace the chaos)
make build-local

# Build static binary for Linux (because portability is overrated)
make build

# Build release binaries for all platforms (reach tens of users worldwide)
make release

# Run tests (watch our beautiful failures)
make test

# Run linters (because even AI code needs judgment)
make lint

# Clean build artifacts (start fresh, fail again)
make clean
```

### Docker Build (The Ultimate Irony)

```bash
docker build -t shmocker .
```

*Yes, we use Docker to build a Docker replacement. The autonomous agents found this hilarious.*

## Usage (Or: How to Replace One Command With Many)

### The Kubernetes Way (What We Actually Built)

```bash
# Build an image using BuildKit on Kubernetes
./scripts/k8s-build.sh Dockerfile . myimage.tar

# What actually happens behind the scenes:
# 1. Uploads your Dockerfile to Kubernetes (as a ConfigMap)
# 2. Spins up a rootless BuildKit pod 
# 3. BuildKit does the actual building (we just watch)
# 4. Downloads the OCI image to your machine
# 5. Cleans up and pretends it was seamless

# Load it into your runtime of choice
docker load < myimage.tar              # Yes, the irony burns
podman load --input myimage.tar        # For the Docker-averse
```

### The Local Way (For macOS Masochists)

```bash
# One-time setup (downloads Ubuntu, because of course it does)
./scripts/setup-macos-colima.sh

# Start your personal Linux (when you need to build)
colima start

# Build through a VM, SSH tunnel, and prayer
shmocker build -t my-image:latest .

# Stop the VM (save those precious macOS resources)
colima stop
```

### What About the Go Binary?

Oh, that beautiful `shmocker` binary we built? It's more of an aspirational piece. It can:
- Parse your Dockerfile (we're really good at reading!)
- Validate the syntax (we'll tell you what's wrong!)
- Generate an AST (Abstract Syntax Trees are cool!)
- ...and then politely inform you that actual building requires BuildKit

Think of it as a very sophisticated Dockerfile linter that dreams of one day becoming a real build tool.

## macOS Support (Or: VMs All The Way Down)

Since BuildKit refuses to run on macOS (something about "kernel features"), we've wrapped a VM in a wrapper in a CLI tool. It's like Docker Desktop, but with more steps and less licensing fees:

```bash
# One-time setup (downloads Ubuntu, because of course it does)
./scripts/setup-macos.sh

# Start your personal Linux (when you need to build)
./scripts/lima-vm.sh start

# Build "natively" (through a VM, ssh, and TCP forwarding)
shmocker build -t my-containerized-disappointment:latest .

# Stop the VM (save those precious macOS resources)
./scripts/lima-vm.sh stop
```

See [macOS Setup Guide](docs/MACOS_SETUP.md) if you enjoy reading about networking layers and socket forwarding.

## Configuration (Because Simple Things Must Be Complex)

Shmocker can be configured using a YAML configuration file at `$HOME/.shmocker.yaml`, because JSON was apparently too readable:

```yaml
default_platform: "linux/amd64"  # We're very platform-agnostic (for exactly one platform)
cache_dir: "~/.shmocker/cache"   # Where dreams go to be cached
signing_enabled: false           # Trust is overrated anyway
sbom_enabled: false             # Ignorance is bliss

registries:
  docker.io:                    # Yes, we still need Docker Hub
    url: "https://registry-1.docker.io"
    username: "myuser"          # Please don't use this in production
    password: "mypass"          # Seriously, don't
```

Environment variables also work, because consistency is for the weak:

```bash
export SHMOCKER_SIGNING_ENABLED=true   # Trust our digital signatures
export SHMOCKER_SBOM_ENABLED=true     # Embrace the paper trail
```

## Development (A Journey Into Madness)

Our autonomous agents have architected a beautifully over-engineered system:

- **`pkg/builder`**: Core image building functionality (NIH syndrome in action)
- **`pkg/dockerfile`**: Dockerfile parsing and validation (because regex wasn't painful enough)
- **`pkg/registry`**: OCI registry interaction (Docker Hub, but complicated)
- **`pkg/sbom`**: Software Bill of Materials generation (itemizing our dependencies' dependencies)
- **`pkg/signing`**: Image signing with Cosign (blockchain for containers, essentially)
- **`internal/config`**: Configuration management (YAML parsing as a service)
- **`internal/cache`**: Build artifact caching (premature optimization, perfectly executed)

## CI/CD (Robots All the Way Down)

Our GitHub Actions workflow is a masterpiece of automation:

- Runs tests and linters (quality control for chaos)
- Performs security scanning (finding vulnerabilities in our vulnerability-finding tool)
- Builds static binaries (portability through stubbornness)
- Creates releases (so the three users can stay updated)
- Builds and publishes Docker images (see previous irony note)

## Status (Or: The Current State of Our Hubris)

🚀 **This project actually works!** (I'm as surprised as you are)

Here's what my autonomous agents accomplished while I wasn't paying attention:

- ✅ **Dockerfile parsing** - Complete lexer and parser supporting Docker 24.x syntax
- ✅ **Image building** - Via BuildKit on Kubernetes (we delegate like pros)
- ✅ **Kubernetes integration** - Automated deployment script that actually works
- ✅ **Rootless execution** - No root required, security team approved
- ✅ **Multi-stage builds** - Because one stage is never enough
- ✅ **Build caching** - BuildKit handles it, we take the credit
- ✅ **OCI image output** - Standards-compliant images that work everywhere
- 🚧 **Registry operations** - Can pull images, pushing is a TODO
- 🚧 **SBOM generation** - The code exists but refuses to compile
- 🚧 **Image signing** - Theoretically possible, practically untested

### What Actually Works Today

```bash
# This will actually build your image
./scripts/k8s-build.sh Dockerfile . myimage.tar

# This will actually run
podman load --input myimage.tar
podman run myimage:latest
```

It's not pretty, it's not fast, but it builds images without Docker. Mission accomplished? 🎉

## FAQ (Frequently Avoided Questions)

**Q: Why does this exist?**  
A: Two reasons:
1. I wanted to test how far autonomous AI agents could go (turns out: very far)
2. I heard "Docker Desktop now costs money" and my agents immediately started building a Docker replacement, blissfully unaware that Docker CLI is still free

It's like building your own car because you heard BMW charges for heated seats, except the car is being built by robots you can't stop.

**Q: Is this better than Docker?**  
A: It's not trying to be better than Docker. It's trying to be Docker-without-Docker. Think of it as Docker's rootless cousin who went to art school.

**Q: What does Shmocker actually do?**  
A: We parse Dockerfiles, validate them, then politely ask BuildKit to do the actual building. We're like a very sophisticated middleman with good intentions.

**Q: Should I use this in production?**  
A: If you're already running Kubernetes and need rootless builds, why not? The core building is done by BuildKit, which is production-ready. We just add a layer of convenience and humor.

**Q: Will this replace Docker?**  
A: No. We literally use BuildKit, which is from the Docker/Moby project. We're more like Docker's helpful friend who knows how to work with Kubernetes.

**Q: Why not just use BuildKit directly?**  
A: You could! But then you'd have to:
- Write your own Kubernetes manifests
- Handle ConfigMap creation
- Manage pod lifecycle
- Download images manually
- Miss out on our delightful error messages

**Q: Is this a real project or an elaborate joke?**  
A: Yes.

**Q: Wait, so this entire project is based on a misunderstanding?**  
A: Essentially, yes. We thought we were sticking it to Big Docker™, but it turns out we were just confused about pricing models. Now we're too deep to stop. The commits have been made. The architecture diagrams have been drawn. We're pot-committed to our misguided spite.

## License

This project is licensed under the Apache License, Version 2.0 - see the [LICENSE](LICENSE) file for details.

### Why Apache 2.0?

We chose Apache 2.0 because:
- **Dependency Alignment**: Our core dependencies (BuildKit, Containerd, Syft) are Apache 2.0 licensed
- **Patent Protection**: Provides explicit patent grant protection for users
- **Industry Standard**: Widely adopted in the container/cloud native ecosystem
- **Commercial Friendly**: Allows commercial use while maintaining open source principles

### Third-Party Licenses

Shmocker incorporates several open source projects:
- **BuildKit** (Apache 2.0) - The actual container building engine
- **Containerd** (Apache 2.0) - Container runtime components  
- **Anchore Syft** (Apache 2.0) - SBOM generation capabilities
- **Go-containerregistry** (Apache 2.0) - Container registry interactions
- Various other dependencies under MIT and BSD licenses

All dependency licenses are compatible with Apache 2.0 and maintain their respective license requirements.
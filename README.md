# Shmocker: Because reinventing Docker is easier than reading the docs

*A rootless Docker image builder crafted by autonomous AI agents who apparently thought the world needed yet another container build tool.*

## The Magnificent Problem We're Solving

Did you know that Docker already exists? Well, our AI overlords didn't get that memo. Instead, they've created Shmocker—a "rootless Docker image builder"—because clearly what the ecosystem was missing was another way to turn Dockerfiles into container images.

But hey, at least it's rootless! Because nothing says "innovation" like taking something that works and making it more complicated.

## What Shmocker Actually Is (An Architectural Confession)

Let's be honest: Shmocker isn't trying to replace the wheel. We're more like the people who looked at a perfectly good wheel and said, "What if we gave it a better user interface?"

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
- 📖 **Dockerfile Parser**: A lovingly hand-crafted lexer and parser that understands all 47 flavors of Dockerfile syntax
- 🎯 **CLI Interface**: Because `shmocker build` sounds cooler than `buildctl build --frontend dockerfile.v0 --local context=.`
- 🔄 **Translation Layer**: Converts our parsed AST into BuildKit's LLB format (it's like Google Translate for container instructions)
- 🎼 **Kubernetes Orchestration**: Manages the intricate dance of ConfigMaps, Pods, and hope

**What BuildKit Does (The Actual Magic):**
- 🏗️ **Image Building**: The real hero that executes RUN commands, manages layers, and makes containers happen
- 🔒 **Rootless Execution**: All the namespace and cgroup wizardry that makes security folks happy
- 📦 **Registry Operations**: Pulls base images, pushes results, handles auth (the boring but crucial stuff)
- ⚡ **Intelligent Caching**: Makes rebuilds fast because waiting is for chumps

### In Restaurant Terms

Think of it this way:
- **You**: The customer with a recipe (Dockerfile)
- **Shmocker**: The helpful waiter who checks your order makes sense and translates it to kitchen-speak
- **BuildKit**: The master chef who actually cooks your meal
- **Kubernetes**: The restaurant infrastructure that makes it all possible

We're not claiming to be Gordon Ramsay here. We're more like that friendly server who makes sure your order gets to the kitchen correctly and brings you your food while it's still hot.

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

🚀 **This project actually works!** (We're as surprised as you are)

Here's what our autonomous agents have accomplished:

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
A: Because sometimes you need to build container images on Kubernetes without Docker. Also, autonomous AI agents don't ask "why," they ask "why not?"

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

## License

TBD (To Be Determined, much like our reasoning for building this)
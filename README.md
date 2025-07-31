# Shmocker: Because reinventing Docker is easier than reading the docs

*A rootless Docker image builder crafted by autonomous AI agents who apparently thought the world needed yet another container build tool.*

## The Magnificent Problem We're Solving

Did you know that Docker already exists? Well, our AI overlords didn't get that memo. Instead, they've created Shmocker—a "rootless Docker image builder"—because clearly what the ecosystem was missing was another way to turn Dockerfiles into container images.

But hey, at least it's rootless! Because nothing says "innovation" like taking something that works and making it more complicated.

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

## Usage (Or: How to Replace One Command With Another)

```bash
# Build an image (just like docker build, but different)
shmocker build /path/to/build/context

# Build with custom tag (because naming things is hard)
shmocker build -t myimage:latest /path/to/build/context

# Build with SBOM generation (document your supply chain sins)
shmocker build --sbom -t myimage:latest /path/to/build/context

# Build and sign the image (cryptographically guarantee it's our fault)
shmocker build --sign -t myimage:latest /path/to/build/context
```

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

🚧 **This project is currently in the "what have we done" phase.**

Our autonomous agents have established a magnificent foundation, but they're still working on making it actually *do* anything:

- [ ] Dockerfile parsing (turning text into more text)
- [ ] Image building logic (the hard part we conveniently ignored)
- [ ] Registry operations (talking to Docker Hub, but different)
- [ ] SBOM generation (cataloging our technical debt)
- [ ] Image signing (digitally signing our mistakes)
- [ ] Build caching (caching broken builds for efficiency)

## FAQ (Frequently Avoided Questions)

**Q: Why does this exist?**  
A: Autonomous AI agents don't ask "why," they ask "why not?"

**Q: Is this better than Docker?**  
A: Define "better." It's certainly more complicated.

**Q: Should I use this in production?**  
A: Only if you enjoy explaining to your team why your build system was written by robots.

**Q: Will this replace Docker?**  
A: About as much as cryptocurrency replaced traditional currency.

## License

TBD (To Be Determined, much like our reasoning for building this)
# Shmocker

A rootless Docker image builder that provides a secure and efficient way to build container images without requiring root privileges.

## Features

- Rootless container image building
- OCI-compliant image output
- SBOM generation
- Image signing with Cosign
- Multi-stage build support
- Build caching

## Project Structure

```
shmocker/
├── cmd/shmocker/           # Main CLI entry point
├── pkg/                    # Public packages
│   ├── builder/           # Core build logic
│   ├── dockerfile/        # Dockerfile parser
│   ├── registry/          # OCI registry client
│   ├── sbom/             # SBOM generation
│   └── signing/          # Cosign integration
├── internal/              # Private packages
│   ├── config/           # Configuration management
│   └── cache/            # Build cache
├── .github/workflows/     # GitHub Actions CI/CD
├── Makefile              # Build automation
├── Dockerfile            # Container image definition
└── go.mod                # Go module definition
```

## Building

### Prerequisites

- Go 1.21 or later
- Make

### Build Commands

```bash
# Build for local development
make build-local

# Build static binary for Linux
make build

# Build release binaries for all platforms
make release

# Run tests
make test

# Run linters
make lint

# Clean build artifacts
make clean
```

### Docker Build

```bash
docker build -t shmocker .
```

## Usage

```bash
# Build an image
shmocker build /path/to/build/context

# Build with custom tag
shmocker build -t myimage:latest /path/to/build/context

# Build with SBOM generation
shmocker build --sbom -t myimage:latest /path/to/build/context

# Build and sign the image
shmocker build --sign -t myimage:latest /path/to/build/context
```

## Configuration

Shmocker can be configured using a YAML configuration file located at `$HOME/.shmocker.yaml`:

```yaml
default_platform: "linux/amd64"
cache_dir: "~/.shmocker/cache"
signing_enabled: false
sbom_enabled: false

registries:
  docker.io:
    url: "https://registry-1.docker.io"
    username: "myuser"
    password: "mypass"
```

Environment variables can also be used with the `SHMOCKER_` prefix:

```bash
export SHMOCKER_SIGNING_ENABLED=true
export SHMOCKER_SBOM_ENABLED=true
```

## Development

The project uses a modular architecture with the following key packages:

- **`pkg/builder`**: Core image building functionality
- **`pkg/dockerfile`**: Dockerfile parsing and validation
- **`pkg/registry`**: OCI registry interaction
- **`pkg/sbom`**: Software Bill of Materials generation
- **`pkg/signing`**: Image signing with Cosign
- **`internal/config`**: Configuration management
- **`internal/cache`**: Build artifact caching

## CI/CD

The project includes a comprehensive GitHub Actions workflow that:

- Runs tests and linters
- Performs security scanning
- Builds static binaries
- Creates releases with multi-platform binaries
- Builds and publishes Docker images

## Status

🚧 **This project is currently in initial development phase.**

The basic project structure and CLI framework have been established, but core functionality is not yet implemented. This includes:

- [ ] Dockerfile parsing
- [ ] Image building logic
- [ ] Registry operations
- [ ] SBOM generation
- [ ] Image signing
- [ ] Build caching

## License

TBD
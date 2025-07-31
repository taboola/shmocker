# Shmocker Examples: Demonstrations of Unnecessary Complexity

*This directory contains example Dockerfiles and build scenarios for testing shmocker functionality. Think of it as a showcase for how to do exactly what Docker already does, but with more steps.*

## Basic Example (Reimagining the Wheel)

The main `Dockerfile` demonstrates our ability to handle all the standard container features:
- Single-stage build *(because we had to start somewhere)*
- Build arguments with defaults *(parameterization for the masses)*
- Metadata labels *(documenting our hubris)*
- Package installation *(dependency management, containerized)*
- User creation and switching *(security through ceremony)*
- File copying with ownership *(preserving the illusion of control)*
- Environment variables *(configuration as code, naturally)*
- Health checks *(monitoring our own questionable decisions)*
- Port exposure *(networking for the brave)*

## Testing the Build (Putting Our Creation to Work)

### Basic build (The "Hello World" of container overengineering):
```bash
shmocker build -t myapp:latest .
# Just like docker build, but with more existential dread
```

### Build with custom arguments (Parameterization Gone Wild):
```bash
shmocker build \
  --build-arg VERSION=2.0.0 \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --build-arg VCS_REF=$(git rev-parse HEAD) \
  -t myapp:v2.0.0 \
  .
# Because hardcoding values is for amateurs
```

### Build for multiple platforms (Architecture Diversity in Action):
```bash
shmocker build \
  --platform linux/amd64,linux/arm64 \
  -t myapp:multiarch \
  .
# Supporting both Intel and ARM, because choice is good
```

### Build with custom Dockerfile:
```bash
shmocker build \
  -f examples/Dockerfile \
  -t myapp:custom \
  .
```

### Build with labels:
```bash
shmocker build \
  --label "project=shmocker-example" \
  --label "team=platform" \
  -t myapp:labeled \
  .
```

### Build with cache:
```bash
shmocker build \
  --cache-from type=registry,ref=myregistry/myapp:cache \
  --cache-to type=registry,ref=myregistry/myapp:cache \
  -t myapp:cached \
  .
```

### Build with output to tar:
```bash
shmocker build \
  --output type=tar,dest=./myapp.tar \
  -t myapp:export \
  .
```

### Build with SBOM and signing (The Full Security Theatre Experience):
```bash
shmocker build \
  --sbom \
  --sign \
  -t myapp:secure \
  .
# Document every dependency and cryptographically prove it's our fault
```

## Expected Behavior (What Should Happen If Everything Goes Right)

When the build completes successfully, you should see *(assuming our autonomous agents got it right)*:
1. Progress output showing each build step *(because transparency builds trust)*
2. Final image ID and digest *(the unique fingerprint of our creation)*
3. Build time and cache statistics *(performance metrics for the competitive)*
4. SBOM generation (if enabled) *(a detailed inventory of our technical dependencies)*
5. Image signing (if enabled) *(cryptographic proof of origin)*

The resulting image will *(if all goes according to plan)*:
- Run as non-root user (appuser) *(security through privilege restriction)*
- Listen on port 8080 *(because 80 is so mainstream)*
- Include health check endpoint *(self-monitoring for the paranoid)*
- Display version and build information when run *(transparency in action)*
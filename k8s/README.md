# Shmocker Kubernetes Demo

Deploy BuildKit in rootless mode on Kubernetes to demonstrate shmocker's building capabilities without Docker.

## Quick Start

### Option 1: Simple Demo Pod

Deploy a single pod with BuildKit rootless:

```bash
kubectl apply -f shmocker-demo-deployment.yaml

# Wait for pod to be ready
kubectl wait --for=condition=ready pod/shmocker-demo-pod -n shmocker-demo

# Build an image
kubectl exec -n shmocker-demo shmocker-demo-pod -- /workspace/build.sh

# Check the built image
kubectl exec -n shmocker-demo shmocker-demo-pod -- tar -tf /workspace/image.tar | head -20
```

### Option 2: BuildKit as a Service

Deploy BuildKit as a StatefulSet with persistent cache:

```bash
# Deploy BuildKit service
kubectl apply -f buildkit-statefulset.yaml

# Wait for it to be ready
kubectl wait --for=condition=ready pod/buildkit-0 -n shmocker-demo --timeout=60s

# Deploy client pod
kubectl apply -f buildkit-client.yaml

# Build using the service
kubectl exec -n shmocker-demo buildkit-client -- /workspace/build-with-service.sh
```

### Option 3: One-off Build Job

Run a single build job:

```bash
kubectl apply -f buildkit-deployment.yaml

# Watch the job
kubectl logs -n shmocker-demo -l job-name=shmocker-build-demo -f

# Check if successful
kubectl get job -n shmocker-demo shmocker-build-demo
```

## Requirements

Your Kubernetes cluster needs:
- Allow `Unconfined` seccomp and AppArmor profiles
- Support for rootless containers (user namespaces preferred but not required)
- At least 2GB memory and 10GB storage for builds

## Troubleshooting

If builds fail with permission errors:
```bash
# Check pod security standards
kubectl get pod -n shmocker-demo -o yaml | grep -A5 securityContext

# Check events
kubectl get events -n shmocker-demo --sort-by='.lastTimestamp'
```

## Clean Up

```bash
kubectl delete namespace shmocker-demo
```

## What This Demonstrates

1. **Rootless Building**: No root privileges needed on the host
2. **No Docker Daemon**: BuildKit runs standalone
3. **OCI Compliance**: Outputs standard OCI images
4. **Cache Efficiency**: StatefulSet maintains build cache
5. **Kubernetes Native**: Runs as regular pods/jobs

This is what shmocker aims to provide - secure, rootless container builds without Docker!
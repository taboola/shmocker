# Product Requirements Document (PRD)

## Rootless Docker‑Image Builder (working name: kimg)

---

### 1  Background & Context

Modern CI/CD pipelines—and Kubernetes clusters—no longer need a full Docker Engine, only the **ability to turn a **``** into an OCI image**. Commercial Docker licences, daemon privileges and the Docker socket all introduce cost or security friction.

Through several technical explorations (recorded in our chat sessions), we converged on the following priorities:

1. **Rootless by default** – must run unprivileged in CI pods and developer laptops.
2. **Single static binary** – easy distribution and deterministic builds.
3. **Full Dockerfile grammar support** – zero change‑management for existing repos.
4. **Production‑grade performance** – parity with Docker Buildx (parallel layers, cache import/export).
5. **Minimal new surface area** – reuse battle‑tested open‑source internals rather than a green‑field re‑impl.

---

### 2  Problem Statement

> *“We need a secure, licence‑free, drop‑in replacement for **`docker build`** that can be invoked in any pipeline or workstation without installing Docker, without requiring root, and without adding operational complexity.”*

---

### 3  Goal(s)

| #   | Goal                                                       | Success indicator                                                                   |
| --- | ---------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| G‑1 | Build any existing project’s `Dockerfile` unmodified.      | 100 % of projects in **core‑services** GitHub org build successfully on first pass. |
| G‑2 | Match or beat Docker Buildx wall‑clock time.               | ≤ 110 % of Buildx time on cold cache; ≤ 105 % on warm cache.                        |
| G‑3 | Distribute as **one** Linux‑static binary (amd64 & arm64). | `curl ‑L …/kimg && chmod +x` works on GitHub Actions, CircleCI, GitLab runners.     |
| G‑4 | Run rootless.                                              | No `CAP_SYS_ADMIN` or privileged pod required.                                      |
| G‑5 | Provide SBOM + Cosign signature out‑of‑the‑box.            | Image pushed contains SPDX SBOM artifact and valid signature against team KMS key.  |

---

### 4  Non‑Goals

- Running or orchestrating containers (Kubernetes or containerd will handle runtime).
- Replacing registries or distribution protocols.
- Supporting bespoke non‑Dockerfile build definitions (e.g., Bazel rules, Nix derivations) at launch.

---

### 5  Stakeholders

- **Engineering Productivity** – owns CI templates and developer tooling.
- **Platform Security** – reviews rootless mode, SBOM and signature chain.
- **Application Teams** – consumers; success measured by migration ease.
- **Release Management** – monitors build throughput and failure rates.

---

### 6  Guiding Principles

1. **Leverage existing OSS where proven** – minimise maintenance burden.
2. **Secure by design** – rootless, content‑addressable, signed outputs.
3. **Performance is a feature** – DO NOT regress build times vs Docker.
4. **Everything as code** – config, SBOM, license scan, pipeline definitions versioned.

---

### 7  Key Decisions & Rationale

| Cross‑point             | Decision                                               | Rationale (focus on what matters to us)                                                                         |
| ----------------------- | ------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------- |
| Implementation language | **Go**                                                 | Native ecosystem for BuildKit, containerd, runc; static binaries build in seconds; abundant in‑house expertise. |
| Build engine            | **Embed BuildKit as a library (in‑process)**           | Retains BuildKit’s DAG solver & cache; eliminates daemon and IPC overhead while staying a single executable.    |
| Execution mode          | **Rootless OCI worker**                                | Avoids root privileges; aligns with constrained environments (K8s pod security standards).                      |
| Distribution format     | **Static musl binary + GitHub release**                | Zero external deps; reproducible hashes for supply‑chain attestation.                                           |
| Image output format     | **OCI Image v1.1**                                     | Universally consumable by registries/runtimes; future‑proof (SBOM & signature attachments).                     |
| Supply‑chain metadata   | **Syft SBOM + Sigstore Cosign signature**              | Meets internal compliance and public‑artifact policies.                                                         |
| Test philosophy         | **Layered: unit → golden → fuzz → integration → perf** | Ensures correctness, backwards compatibility and performance regressions caught early.                          |

---

### 8  User Stories (Happy‑Path only)

1. **CI engineer** adds `kimg build -f Dockerfile -t ghcr.io/acme/app:$SHA .` to workflow and build passes first time.
2. **Developer on M1 laptop** downloads `kimg`, runs build without sudo, sees familiar Buildx‑style progress UI.
3. **Security reviewer** queries registry and finds SBOM + valid Cosign signature attached to every pushed tag.

---

### 9  Functional Requirements

| ID  | Requirement                                                                                               |
| --- | --------------------------------------------------------------------------------------------------------- |
| F‑1 | Parse full Dockerfile grammar up to Docker 24 syntax extensions (`RUN --mount=type=cache`, secrets, SSH). |
| F‑2 | Accept remote build context via `--context` (git URL, tar.gz, stdin).                                     |
| F‑3 | Support `--cache-from`, `--cache-to`, `--build-arg`, `--target`, `--platform`.                            |
| F‑4 | Produce multi‑arch manifests (`linux/amd64`, `linux/arm64`) when QEMU available.                          |
| F‑5 | Push directly to OCI registry or emit OCI‑layout tar file.                                                |
| F‑6 | Emit SPDX‑2.3 SBOM and attach via OCI artifact association.                                               |
| F‑7 | Sign final image with Cosign key pair supplied via env or KMS provider.                                   |
| F‑8 | Provide JSON event stream (`--progress=json`) for CI dashboards.                                          |
| F‑9 | Exit with non‑zero code on unexpected network/disc errors, propagate error detail.                        |

---

### 10  Non‑Functional Requirements

| Category            | Requirement                                                                                      |
| ------------------- | ------------------------------------------------------------------------------------------------ |
| **Performance**     | Cold‑cache build ≤ 110 % of Docker Buildx baseline on reference project set; warm cache ≤ 105 %. |
| **Security**        | Pass `gosec`, `govulncheck`; no root required; supply chain signed.                              |
| **Portability**     | Runs on Linux kernel ≥ 5.4 with overlayfs; no daemon, no systemd deps.                           |
| **Observability**   | Structured logs (`logfmt`), trace export (`buildkit --trace`) retained in CI artifacts.          |
| **Release cadence** | Patch releases on demand; minor release monthly; security fixes < 24 h.                          |

---

### 11  High‑Level Architecture

```
kimg (single binary)
 ├─ Dockerfile frontend → AST → LLB
 ├─ BuildKit controller (in‑process)
 │    └─ rootless OCI worker (overlayfs snapshotter, runc executor)
 ├─ Content store (local or cache dir)
 ├─ Image assembler → OCI manifest
 ├─ SBOM generator (Syft lib call)
 ├─ Cosign signer
 └─ Registry client (OCI dist v1)
```

---

### 12  Test & QA Strategy

1. **Static analysis** – `go vet`, `staticcheck`, `gosec`, `govulncheck`.
2. **Unit tests** – table‑driven parser & flag tests.
3. **Golden tests** – LLB JSON diff for known Dockerfiles.
4. **Go fuzzing** – random Dockerfile tokens to parser (nightly).
5. **Integration** – spin up in‑process builder, build official docker‑samples repo matrix.
6. **Performance regression** – benchmark suite; fail CI if > 10 % slower.
7. **Chaos** – inject SIGTERM, registry 503, disk‑full scenarios.
8. **Security** – OPA/Gatekeeper policy test harness ensures images meet org rules.

---

## 13  Milestones & Timeline *(rough)*

| Milestone | Scope                                                          |
| --------- | -------------------------------------------------------------- |
| M‑0       | Repo bootstrap, CI skeleton, static build POC                  |
| M‑1       | Dockerfile parse + single‑stage build, local tar output        |
| M‑2       | Registry push, cache import/export                             |
| M‑3       | Multi‑stage, multi‑arch, rootless polished                     |
| M‑4       | SBOM + Cosign, security audit pass                             |
| M‑5       | Beta rollout to 3 pilot teams, feedback loop                   |
| GA        | Company‑wide adoption, deprecate Docker Buildx in CI templates |

---

## 14  Success Metrics

- **Adoption** – ≥ 80 % of weekly CI builds use `kimg` within 60 days of GA.
- **Build time delta** – Median build time ≤ 105 % of previous baseline across fleet.
- **Security incidents** – 0 critical CVEs arising from builder in first 12 months.
- **License savings** – 100 % Docker Desktop licence renewals eliminated for dev laptops.

---

## 15  Risks & Mitigations

| Risk                                                         | Impact                           | Mitigation                                                                     |
| ------------------------------------------------------------ | -------------------------------- | ------------------------------------------------------------------------------ |
| Upstream BuildKit API churn                                  | Build breakage                   | Vendor tagged releases; integration tests pin digest.                          |
| Kernel feature mismatch (overlayfs, userns) on older runners | Build fails                      | Detect at startup; graceful fallback to `fuse-overlayfs` or informative error. |
| Performance regression vs Buildx                             | Longer CI, developer frustration | Perf suite, fail PRs over threshold.                                           |
| Supply‑chain tool CVEs (Syft, Cosign)                        | Security block                   | Version pin + Dependabot alerts + 24 h patch SLA.                              |

---

##

---

##


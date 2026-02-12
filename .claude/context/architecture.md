# Architecture

## CLI Execution Flow

1. `main.go` initializes root Cobra command with global flags
2. Commands registered via `root.AddCommand()` (build, deploy, up, down, context, namespace, etc.)
3. Each command defines `RunE` function for execution logic
4. Commands use injected dependencies (ioController, k8sLogger, analytics, insights)
5. Commands respect current Okteto context and Kubernetes namespace via `pkg/okteto` context store

## Directory Structure

### `cmd/`

- Top-level command files: `version.go`, `status.go`, `doctor.go`, etc.
- Subdirectories for commands with subcommands: `context/`, `namespace/`, `deploy/`, `destroy/`, `up/`, etc.
- Each command imports packages from `pkg/` for business logic

### `pkg/`

Core business logic organized by domain:

**Kubernetes operations:**

- `k8s/` - Kubernetes resource operations (CRUD for configmaps, secrets, deployments, etc.)
- `syncthing/` - File synchronization between local and remote
- `ssh/` - SSH server management for dev containers

**Data & Configuration:**

- `model/` - Core data structures (Manifest, Dev, Stack, etc.)
- `types/` - Type definitions for manifests and build options
- `config/` - Configuration management (home directory, binary name, version)

**Okteto Platform:**

- `okteto/` - Okteto API client (contexts, namespaces, pipelines, previews, secrets)
- `analytics/` - Analytics tracking
- `insights/` - Insights publishing to cluster

**Build & Deploy:**

- `build/` - Build orchestration and execution
- `deployable/` - Deployable entity detection and management
- `registry/` - Container registry operations

**Utilities:**

- `log/` - Logging with multiple output formats (TTY, plain, JSON)
- `errors/` - Custom error types (UserError with hints)
- `filesystem/` - File system operations
- `validator/` - Manifest validation
- `env/` - Environment variable handling

### `tools/`

Internal utilities that run inside development containers:

- `tools/remote/` - SSH server for remote access to dev containers
- `tools/supervisor/` - Process supervisor for managing application processes
- `tools/clean/` - Cleanup utility for container environments
- Unified Go module (`tools/go.mod`) with shared dependency management
- Built from source during Docker image creation (see `tools-builder` stage in Dockerfile)
- Not part of the CLI binary - bundled separately in `/usr/bin-image/bin/` in the Docker image

**Build:**

- Local: `cd tools && make build` (outputs to `bin/`)
- Docker: Automatically built via `tools-builder` stage in main Dockerfile

> **See also**: [build-and-release.md](build-and-release.md) for complete build system details, Docker image build process, and CVE remediation workflow.

### `internal/`

- `internal/test/` - Test utilities (fake K8s clients, fake Okteto clients, context helpers)
- Not importable by external packages (Go visibility rules)

### `integration/`

- E2e tests requiring live Kubernetes cluster
- Organized by command: `integration/build/`, `integration/deploy/`, `integration/up/`, etc.
- Require build tag `//go:build integration`
- Use environment variables for cluster access

## Key Package Interactions

**Command → Business Logic:**

- `cmd/up/` → `pkg/model` (manifest parsing) → `pkg/k8s` (deployment) → `pkg/syncthing` (sync)
- `cmd/deploy/` → `pkg/deployable` (detect stack/compose) → `pkg/build` → `pkg/okteto` (API)
- `cmd/build/` → `pkg/build` → `pkg/registry` → `pkg/okteto` (API)

**Cross-Cutting:**

- All commands use `pkg/log` for output
- All commands use `pkg/okteto` for context management
- All commands use `pkg/analytics` for tracking

**Context Flow:**

- `pkg/okteto.GetContextStore()` provides current context
- Context includes: cluster URL, namespace, user, K8s config
- Commands call `contextCMD.LoadManifestWithContext()` to respect active context

## Manifest-Driven Design

- All deployment commands operate on `okteto.yml` manifests
- `pkg/model.Manifest` is central to command execution
- Manifest defines: dev containers, build images, deploy commands, dependencies
- Commands discover manifest via `pkg/discovery` if not explicitly provided

# Okteto CLI

A Go CLI tool that accelerates Kubernetes application development. Developers write code locally and Okteto instantly syncs changes to remote Kubernetes environments, enabling seamless cloud-native development without local infrastructure.

## Repository Structure

```
okteto/
├── cmd/              # CLI commands (Cobra-based): up, down, build, deploy, destroy, etc.
├── pkg/              # Shared packages: k8s, model, analytics, okteto client, types
├── tools/            # Internal tools (remote, supervisor, clean) - see Tools section below
├── internal/         # Internal code (test utilities)
├── integration/      # Integration tests requiring Kubernetes cluster
├── samples/          # Example projects for different languages
├── main.go           # Entry point - wires up all commands
├── Dockerfile        # Container image definition (includes tools build)
└── Makefile          # Build, test, lint automation
```

**Command structure**: Each command in `cmd/` may have subcommands in subdirectories (e.g., `cmd/namespace/create.go`, `cmd/context/use.go`).

**Package organization**: `pkg/` contains domain packages. Core packages include `k8s` (Kubernetes operations), `model` (data structures), `okteto` (API client), `types` (manifest types).

**Tools directory**: `tools/` contains internal utilities (remote, supervisor, clean) that are compiled and bundled into the Okteto CLI Docker image. These tools run inside development containers.

> **See also**: [context/architecture.md](.claude/context/architecture.md) for detailed package interactions, CLI execution flow, and manifest-driven design patterns.

## Go Conventions

- **Go version**: check go.mod (do not assume)
- **Style**: Follow Effective Go and Uber Go Style Guide
- **CLI framework**: spf13/cobra for all commands
- **Error handling**: Return errors, don't panic
- **Logging**: Use `pkg/log` for consistent logging across commands
- **Testing**: stretchr/testify with `require` package (not `assert`)
- **Imports**: Standard library → external packages → internal packages
- **Naming**: Exported identifiers in UpperCamelCase, unexported in lowerCamelCase

## CLI Behavior Expectations

- **Never invent commands, flags, or subcommands** - only implement what exists or is explicitly requested
- **Consistency**: Match patterns from existing commands (flag naming, help text style, error messages)
- **User feedback**: Commands should provide clear progress indicators and error messages
- **Context awareness**: All commands respect current Okteto context and Kubernetes namespace
- **Manifest-driven**: All commands (up, deploy, destroy) operate on `okteto.yml` manifests

> **See also**: [context/commands.md](.claude/context/commands.md) for detailed command implementation patterns, Cobra usage, and adding new commands.

## Testing Rules

- **Test files**: Co-locate with source as `*_test.go`
- **Framework**: Use Go's `testing` package + `github.com/stretchr/testify/require`
- **Run tests**: `make test` (runs all unit tests with race detection and coverage)
- **Integration tests**: In `integration/` directory, run with `make integration` (requires cluster and environment variables)
- **Coverage**: Aim for meaningful test coverage, especially for core business logic
- **Table-driven tests**: Prefer table-driven tests for multiple scenarios
- **Mocking**: Use interfaces for dependencies to enable testing

> **See also**: [context/testing.md](.claude/context/testing.md) for test structure patterns, mocking strategies, integration test setup, and the no-branching rule.

## Strict Rules

### NEVER:

- Invent new CLI commands, flags, or arguments without explicit requirements
- Modify code without reading it first - always Read before Edit
- Skip copyright headers (all `.go` files require Apache 2.0 header with year 2023-2025)
- Use `panic()` in production code - return errors instead
- Break existing CLI interfaces or change flag behavior without discussion

### ALWAYS:

- Read existing code before making changes to understand patterns
- Run `make lint` before considering changes complete
- Write tests for new functionality or bug fixes
- Sign commits with `git commit -s` (DCO requirement)
- Use existing patterns from similar commands as templates
- Keep test bodies linear: no branching (`if/switch`) in test functions; split into multiple tests instead.

## Priority Rules (Order of Operations)

1. **Read first, write second**: Use Read/Grep/Glob to understand existing code before proposing changes
2. **Ask before assuming**: If requirements are unclear, use AskUserQuestion before implementing
3. **Small, consistent changes**: Prefer focused changes that match existing code style
4. **Test new functionality**: Add tests for new features, validate with `make test`
5. **Respect the architecture**: Follow established patterns (Cobra commands, package boundaries, error handling)
6. **Copyright compliance**: Every new `.go` file needs the copyright header from `.copyright-header.tmpl`

## Key Commands

- `make build` - Build `okteto` binary to `bin/okteto`
- `make test` - Run unit tests with race detection and coverage
- `make lint` - Run golangci-lint and pre-commit checks
- `make integration` - Run all integration tests (requires Kubernetes cluster)
- `make dep` - Tidy Go modules

### Integration Test Commands

Integration tests are organized by package and can be run individually:

- `make integration-actions` - Run actions integration tests
- `make integration-build` - Run build integration tests
- `make integration-deploy` - Run deploy integration tests
- `make integration-okteto` - Run okteto integration tests
- `make integration-up` - Run up integration tests
- `make integration-okteto-test` - Run okteto test integration tests
- `make integration-gateway` - Run gateway/ingress integration tests (see below)

#### Gateway Integration Tests

Tests for compose endpoints with Gateway API (HTTPRoute) and Ingress:

- `make integration-gateway` - Run all gateway/ingress tests
- `make integration-gateway ARGS="gateway"` - Run only Gateway (HTTPRoute) tests
- `make integration-gateway ARGS="ingress"` - Run only Ingress tests

These tests verify:

- `OKTETO_COMPOSE_ENDPOINTS_TYPE=gateway` creates HTTPRoute resources (not Ingress)
- `OKTETO_COMPOSE_ENDPOINTS_TYPE=ingress` creates Ingress resources (not HTTPRoute)
- Full lifecycle: namespace → deploy → up → file sync → pod recovery → down → cleanup

### Tools Commands

Tools are internal utilities compiled into the CLI Docker image:

- `cd tools && make build` - Build all tools (remote, supervisor, clean) to `bin/`
- `cd tools && make test` - Run tools unit tests
- `cd tools && make lint` - Lint tools code
- `cd tools && make remote` - Build only remote binary
- `cd tools && make supervisor` - Build only supervisor binary
- `cd tools && make clean-tool` - Build only clean binary

> **See also**: [context/build-and-release.md](.claude/context/build-and-release.md) for detailed build system, cross-platform compilation, Docker image build process, linting configuration, and release workflow.

## Tools (Internal Utilities)

The `tools/` directory contains utilities that run inside development containers:

- **remote** (`tools/remote/`) - SSH server for remote access to dev containers
- **supervisor** (`tools/supervisor/`) - Process supervisor for managing application processes
- **clean** (`tools/clean/`) - Cleanup utility for container environments

These tools are:

- Built from source during Docker image creation (see `tools-builder` stage in Dockerfile)
- Installed in the CLI image at `/usr/bin-image/bin/{okteto-remote,okteto-supervisor,clean}`
- Managed as a single Go module (`tools/go.mod`) with unified dependencies
- Not intended for direct user interaction (internal to Okteto's development workflow)

**Security**: To fix CVEs in the Docker image (including tools and bundled binaries), use the `/fix-cves` skill which automates vulnerability scanning with Trivy and systematic remediation. See [skills/fix-cves/SKILL.md](.claude/skills/fix-cves/SKILL.md) for the complete workflow.

## Priority Rules

If there is a conflict:

1. **Follow this file**
2. **Follow existing code**
3. **Ask before inventing new patterns**

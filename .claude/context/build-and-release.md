# Build and Release

## Build System

### Makefile Targets

**Development:**

- `make build` - Build `bin/okteto` binary for local platform
- `make dep` - Tidy Go modules (`go mod tidy`)

**Testing:**

- `make test` - Run unit tests with race detection and coverage
- `make integration` - Run all integration tests (requires cluster)
- `make integration-build` / `make integration-deploy` / etc. - Specific integration suites

**Quality:**

- `make lint` - Run pre-commit hooks + golangci-lint
- `make lint-fix` - Auto-fix linting issues where possible
- `make codecov` - Generate HTML coverage report

**Release:**

- `make build-all` - Cross-compile for all platforms (linux, darwin, windows; amd64, arm64)
- `make latest` - Write version to `bin/latest`

### Build Command

Binary built with:

```bash
go build -trimpath \
  -ldflags "-s -w -X github.com/okteto/okteto/pkg/config.VersionString=${VERSION_STRING}" \
  -o bin/okteto
```

**Flags:**

- `-trimpath` - Remove file system paths from binary
- `-s -w` - Strip debug info and symbol table (smaller binary)
- `-X ...VersionString=...` - Inject version at compile time
- `-tags` - Build tags for platform-specific code (netgo, osusergo, static_build)

**Version String:**

- Defaults to short git commit hash (`git rev-parse --short HEAD`)
- Override with `VERSION_STRING` environment variable

### Cross-Platform Builds

Platforms supported:

- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

Each platform build produces:

- Binary: `bin/okteto-Platform-Arch` (or `okteto.exe` on Windows)
- Checksum: `bin/okteto-Platform-Arch.sha256`

## Linting

### Pre-commit Hooks

Configured in `.pre-commit-config.yaml`:

- **prettier** - Format JSON, Markdown, YAML files
- **gitleaks** - Detect hardcoded secrets
- **pre-commit-hooks** - Check file size, AST, case conflicts, etc.

Run manually: `pre-commit run --all-files`

### golangci-lint

Configured in `.golangci.yml`:

- **unparam** - Detect unused parameters
- **errcheck** - Check error handling
- **goheader** - Enforce copyright headers
- **mnd** - Magic number detection

Run: `golangci-lint run -v --timeout 5m`

### Copyright Headers

**Required in all `.go` files:**

```go
// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// ...
```

Template: `.copyright-header.tmpl`
Enforced by: `golangci-lint` goheader linter
Valid years: 2023, 2024, 2025 (regex in `.golangci.yml`)

## CI Expectations

### Automated Checks

Code expects CI to enforce:

- All tests pass (`make test`)
- Linting passes (`make lint`)
- Integration tests pass on real cluster
- Copyright headers present

### Git Requirements

- Signed commits (`git commit -s`) for DCO compliance
- No force pushes to main/master
- Hooks must not be skipped (no `--no-verify`)

## Release Process

1. Version is set via `VERSION_STRING` environment variable
2. `make build-all` compiles for all platforms
3. Checksums generated for each binary
4. Binaries and checksums published (process not in this repo)

## Development Workflow

**Before committing:**

1. `make test` - Ensure unit tests pass
2. `make lint` - Fix any linting issues
3. Ensure copyright headers on new files

**Before submitting PR:**

1. All pre-commit hooks must pass
2. Sign commits with `-s` flag

## Version Information

- Version injected at build time into `pkg/config.VersionString`
- Displayed via `okteto version` command
- Used for analytics and update checks
- Defaults to git commit hash in development builds

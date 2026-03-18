# Go Conventions & CLI Behavior

> **See also**: [commands.md](commands.md) for command implementation patterns and [architecture.md](architecture.md) for package structure.

## Go Conventions

- **Go version**: check `go.mod` — do not assume
- **Style**: Follow Effective Go and Uber Go Style Guide
- **CLI framework**: `spf13/cobra` for all commands
- **Error handling**: Return errors, don't panic
- **Logging**: Use `pkg/log` for consistent logging across commands
- **Testing**: `stretchr/testify` with `require` package (NOT `assert`)
- **Imports**: Standard library → external packages → internal packages
- **Naming**: Exported identifiers in `UpperCamelCase`, unexported in `lowerCamelCase`

## CLI Behavior Expectations

- **Never invent commands, flags, or subcommands** — only implement what exists or is explicitly requested
- **Consistency**: Match patterns from existing commands (flag naming, help text style, error messages)
- **User feedback**: Commands should provide clear progress indicators and error messages
- **Context awareness**: All commands respect current Okteto context and Kubernetes namespace
- **Manifest-driven**: All commands (`up`, `deploy`, `destroy`) operate on `okteto.yml` manifests

## Copyright Headers

All `.go` files require this header (template: `.copyright-header.tmpl`):

```go
// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// ...
```

Valid years: 2023 onwards. Enforced by `golangci-lint` goheader linter.

# Commands

## Cobra Command Organization

### Structure in `cmd/`

- **Top-level commands**: Single file at `cmd/` root (e.g., `cmd/version.go`, `cmd/status.go`)
- **Commands with subcommands**: Directory at `cmd/` with files for each subcommand
  - Example: `cmd/context/` contains `create.go`, `use.go`, `list.go`, `show.go`, etc.
  - Example: `cmd/namespace/` contains `create.go`, `delete.go`, `use.go`, etc.
- **Main parent command**: Often a file named after the directory (e.g., `cmd/context/context.go`)

### Registration in `main.go`

Commands registered in order:

```go
root.AddCommand(cmd.Version())
root.AddCommand(contextCMD.Context())
root.AddCommand(build.Build(ctx, ...))
root.AddCommand(deploy.Deploy(ctx, ...))
root.AddCommand(up.Up(...))
// etc.
```

## Command Implementation Pattern

### 1. Options Struct

Defines command-specific flags and runtime state:

```go
type Options struct {
    ManifestPathFlag string  // User-provided -f flag
    ManifestPath     string  // Resolved path during execution
    Namespace        string
    Variables        []string
    // ... other flags
}
```

### 2. Command Constructor Function

Returns `*cobra.Command` with dependencies injected:

```go
func Deploy(ctx context.Context, at analytics, insights, ioCtrl, k8sLogger) *cobra.Command {
    opts := &Options{}
    cmd := &cobra.Command{
        Use:   "deploy [service]",
        Short: "Deploy your Development Environment",
        RunE: func(cmd *cobra.Command, args []string) error {
            // execution logic
        },
    }
    // Add flags
    cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "...")
    return cmd
}
```

### 3. Flag Definitions

- Use `cmd.Flags().StringVarP()` for flags with shorthand
- Use `cmd.Flags().BoolVar()` for boolean flags
- Mark hidden flags: `cmd.Flags().MarkHidden("flag-name")`
- Set default values in function signature or via flag definition

### 4. RunE Function

Main execution logic:

- Parse args
- Connect to okteto context
- Load manifest via `discovery` or direct path
- Validate options
- Execute business logic (build, deploy, sync, etc.)
- Track analytics on success/failure
- Return error (sets exit code and displays to user)

### 5. Error Handling

- Return errors from `RunE` (command framework handles display)
- Use `oktetoErrors.UserError{E: err, Hint: "..."}` for actionable hints
- Set `cmd.SilenceUsage = true` in `RunE` to avoid usage on errors

## Common Patterns

### Manifest Loading

```go
manifest, err := contextCMD.LoadManifestWithContext(ctx, opts.ManifestPath, opts.Namespace)
```

### Context & Namespace Setup

```go
ctxStore := okteto.GetContextStore()
if !okteto.IsOkteto() {
    return errOnlyOktetoContexts
}
```

### Analytics Tracking

```go
defer func() {
    analytics.TrackDeploy(err == nil)
}()
```

### Signal Handling

Long-running commands (up, deploy) use context cancellation:

```go
ctx, cancel := context.WithCancel(ctx)
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
go func() {
    <-sigChan
    cancel()
}()
```

## Shared Command Utilities

### `cmd/utils/`

- `MaximumNArgsAccepted()` - Arg count validation
- Flag parsing helpers
- Common validation logic

### `cmd/namespace/`, `cmd/context/`

Reusable functions for:

- Namespace selection/creation
- Context switching
- Kubeconfig manipulation

## Adding New Commands

1. Create directory under `cmd/` (if subcommands needed)
2. Implement constructor returning `*cobra.Command`
3. Define Options struct for flags
4. Implement RunE with business logic
5. Register in `main.go` via `root.AddCommand()`
6. Follow existing command patterns for consistency
7. Add tests in `*_test.go` alongside command file

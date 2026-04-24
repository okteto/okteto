---
title: "feat: okteto sandbox info command"
date: 2026-04-24
status: active
origin: docs/brainstorms/2026-04-24-sandbox-info-requirements.md
---

# Plan: `okteto sandbox info`

**Created:** 2026-04-24
**Origin doc:** `docs/brainstorms/2026-04-24-sandbox-info-requirements.md`

---

## Problem Frame

Once `okteto connect <deployment>` is running there is no way to inspect its status from a separate terminal. Users cannot tell whether the session is actively syncing, still starting up, or has failed without tailing the original process.

## Goal

Add `okteto sandbox info <deployment>` — a read-only command that reads the state file written by `okteto connect` and prints a human-readable status with a scripting-friendly exit code.

Also: `okteto connect` currently never writes the `ready` state. It must be added so the `ready` row in the state table is reachable.

---

## Scope Boundaries

- **In scope:** read state, display message, exit with appropriate code; write `ready` state in `okteto connect`
- **Out of scope:** `--watch` flag, showing logs, restarting the session, any additional sandbox subcommands (`list`, `stop`)

---

## Key Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Command group name | `sandbox` | Deliberate product noun for manifest-free sessions (see origin doc) |
| Exit code for absent/failed | 1 (via `oktetoErrors.UserError`) | Scripting / health-check friendliness; `UserError` propagates through `root.Execute()` → `main()` → `os.Exit(1)` |
| Exit code for unknown state | 0 (return nil) | Unknown ≠ broken; safe to continue |
| Distinguish absent vs unreadable | `os.Stat` before `config.GetState` | `GetState` returns `Failed` for any read error, including missing file (see origin doc) |
| Context init | `contextCMD.NewContextCommand().Run` | Canonical pattern across all commands (see `cmd/connect/connect.go:75`) |
| Namespace source | `okteto.GetContext().Namespace` after context init | Same as `okteto connect` |
| Where `ready` state is written | After `c.success = true` in `activate()`, before `waitUntilExitOrInterrupt` | Matches the progression described in the origin doc |
| Command group structure | `cmd/sandbox/sandbox.go` + `cmd/sandbox/info.go` | Standard Okteto multi-subcommand pattern (see `cmd/namespace/`, `cmd/pipeline/`) |

---

## State Mapping

(see origin doc for full table)

| `config.UpState` value | Message printed | Exit code |
| --- | --- | --- |
| file absent | `Sandbox "X" is not running` | 1 |
| `activating` | `Sandbox "X" is starting (activating dev container…)` | 0 |
| `starting` | `Sandbox "X" is starting (scheduling pod…)` | 0 |
| `attaching` | `Sandbox "X" is starting (attaching persistent volume…)` | 0 |
| `pulling` | `Sandbox "X" is starting (pulling image…)` | 0 |
| `startingSync` | `Sandbox "X" is starting (initialising file sync…)` | 0 |
| `synchronizing` | `Sandbox "X" is running and syncing files` | 0 |
| `ready` | `Sandbox "X" is running` | 0 |
| `failed` | `Sandbox "X" has failed` | 1 |
| unknown value | `Sandbox "X" is in an unknown state (<value>)` | 0 |

---

## Implementation Units

### Unit 1 — Write `ready` state in `okteto connect`

**Goal:** Make the `ready` state reachable by having `okteto connect` emit it once the SSH tunnel is up and the initial sync has completed.

**Files:**

- Modify: `cmd/connect/activate.go`

**Approach:**
After the line `c.success = true` (the exact line where connect marks itself successful), call `config.UpdateStateFile(c.Dev.Name, c.Namespace, config.Ready)`. Log the error with `oktetoLog.Infof` if the write fails — do not abort the session, the state file is informational only.

**Patterns to follow:** `config.UpdateStateFile` calls already present earlier in `activate.go` (e.g. `config.Activating`, `config.Synchronizing`).

**Test scenarios:**

- The existing activation flow compiles without errors (CI)
- `config.Ready` constant exists in `pkg/config/config.go` — verify with a grep

**Verification:** `grep -n "config.Ready" cmd/connect/activate.go` shows the new call at the correct insertion point.

---

### Unit 2 — `cmd/sandbox` package

**Goal:** Create the `sandbox` command group with the `info` subcommand.

**Files:**

- Create: `cmd/sandbox/sandbox.go`
- Create: `cmd/sandbox/info.go`
- Create: `cmd/sandbox/info_test.go`

**Approach — `sandbox.go`:**

Expose a single `Sandbox(ctx context.Context) *cobra.Command` factory. The parent command accepts no args itself (`Args: utils.NoArgsAccepted(...)`). Register the `Info(ctx)` subcommand. No `Command` struct needed — `info` is stateless at the group level.

**Approach — `info.go`:**

```
func Info(ctx context.Context) *cobra.Command
```

Options struct with `Namespace` and `K8sContext` string fields (for `--namespace` / `--context` flags, matching the `okteto connect` flag names).

`RunE` flow:

1. `cmd.SilenceUsage = true`
2. `contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Show: true, Namespace: opts.Namespace, Context: opts.K8sContext})`
3. `ns := okteto.GetContext().Namespace`
4. `statePath := filepath.Join(config.GetAppHome(ns, name), "okteto.state")`
5. `os.Stat(statePath)` — if `os.IsNotExist`, print `Sandbox "<name>" is not running` and return `oktetoErrors.UserError{E: fmt.Errorf("sandbox %q is not running", name)}`
6. `state, _ := config.GetState(name, ns)` — error ignored (Stat already confirmed the file exists)
7. Switch on `state` → print message; return `oktetoErrors.UserError` for `config.Failed`; return nil for all running/unknown states
8. For unknown state, use a default case: `oktetoLog.Printf("Sandbox %q is in an unknown state (%s)\n", name, state)`

Use `oktetoLog.Printf` (not `fmt.Printf`) for output to respect quiet/json modes.

**Patterns to follow:**

- `cmd/connect/connect.go` — context init, Options struct, flag binding
- `cmd/namespace/use.go` — `RunE` structure with `SilenceUsage = true`
- `pkg/config/config.go` — `GetState`, `GetAppHome`, state constants

**Test scenarios for `info_test.go`:**

- File absent → returns a non-nil error (UserError), correct message substring
- State `activating` → nil error, message contains "starting (activating"
- State `starting` → nil error, message contains "starting (scheduling"
- State `attaching` → nil error, message contains "starting (attaching"
- State `pulling` → nil error, message contains "starting (pulling"
- State `startingSync` → nil error, message contains "starting (initialising"
- State `synchronizing` → nil error, message contains "running and syncing"
- State `ready` → nil error, message contains "is running" (no extra qualifier)
- State `failed` → non-nil error (UserError), message contains "has failed"
- Unknown state value → nil error, message contains "unknown state" and the raw value

Tests must use a temporary directory (`t.TempDir()`) for the state file; inject the path via the afero FS or by pointing `OKTETO_HOME` to the temp dir. Do **not** mock `config.GetState` — write the real state file so the full read path is exercised.

**Verification:** `go test ./cmd/sandbox/...` passes; `make lint` passes.

---

### Unit 3 — Register `sandbox` in `main.go`

**Goal:** Wire the new `sandbox` command into the CLI's root command so it appears in `--help` and is executable.

**Files:**

- Modify: `main.go`

**Approach:**
Import `sandboxCMD "github.com/okteto/okteto/cmd/sandbox"` and add one line in the command registration block:

```go
root.AddCommand(sandboxCMD.Sandbox(ctx))
```

Position it alphabetically near `connect` in the registration block. The `Sandbox` factory takes only `ctx context.Context` — no `ioController` or `k8sLogger` because the info subcommand is read-only and uses the legacy `oktetoLog` logger (consistent with how `cmd/connect` is registered).

**Patterns to follow:** Existing `root.AddCommand(...)` calls in `main.go` (lines 153–183).

**Test scenarios:**

- `go build ./...` succeeds (compilation check)
- `okteto sandbox --help` lists `info` as a subcommand (manual smoke test)

**Verification:** `go build ./...` produces no errors; `okteto sandbox info --help` shows correct usage.

---

## Dependencies and Sequencing

Unit 1 and Unit 2 are independent — they touch different packages. Both must land before or alongside Unit 3 (which merely imports and registers).

Recommended order: 1 → 2 → 3, each as a separate commit.

---

## Test File Paths

| Implementation file | Test file |
| --- | --- |
| `cmd/sandbox/info.go` | `cmd/sandbox/info_test.go` |
| `cmd/connect/activate.go` | `cmd/connect/activate_test.go` (existing) |

---

## References

- Origin doc: `docs/brainstorms/2026-04-24-sandbox-info-requirements.md`
- State constants + helpers: `pkg/config/config.go`
- Context init pattern: `cmd/connect/connect.go` (lines 73–86)
- Command group template: `cmd/namespace/namespace.go`, `cmd/namespace/use.go`
- Root registration: `main.go` (lines 153–183)
- Where `ready` state is inserted: `cmd/connect/activate.go` (after `c.success = true`)

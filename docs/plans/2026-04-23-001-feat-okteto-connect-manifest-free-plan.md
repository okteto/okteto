---
title: "feat: Add manifest-free okteto connect command"
type: feat
status: active
date: 2026-04-23
origin: docs/brainstorms/okteto-connect-requirements.md
---

# feat: Add manifest-free `okteto connect` command

## Overview

Add a new `okteto connect <name>` command that activates a dev container against an existing Deployment or StatefulSet — without requiring an `okteto.yaml` manifest. It syncs the local CWD to the container's workdir via syncthing, always injects an init container that installs Claude Code, and leaves the dev container running when the user exits (reconnectable via a second `okteto connect <name>`).

## Problem Frame

Both `okteto up` and the (non-existent-yet) `okteto connect` require a manifest to drive dev-mode activation. This creates friction for two workflows:

1. A developer wants to quickly iterate against an existing deployment without authoring YAML.
2. A Claude Code–driven workflow where the goal is to sync code into a pod and launch an AI coding session from inside the container — the manifest is overhead, not value.

(See origin: `docs/brainstorms/okteto-connect-requirements.md`)

## Requirements Trace

- R1. `okteto connect <name>` works with no manifest present.
- R2. Syncs CWD ↔ container workdir via syncthing.
- R3. Always injects a Claude Code installer init container.
- R4. On Ctrl+C, local sync stops but the dev container stays running.
- R5. Re-running `okteto connect <name>` reconnects to an already-active dev container.
- R6. `--image` flag overrides the container image; otherwise uses the deployment's existing image.
- R7. Standard `--namespace` / `--context` / `--env` flags are supported.

## Scope Boundaries

- No port-forwarding support in v1.
- No manifest generation or interactive wizard.
- No entrypoint/command replacement (unlike `okteto up`).
- No persistent volume creation (`PersistentVolume: false`).
- No hybrid mode support.

### Deferred to Separate Tasks

- Port-forward flag (`--forward`): separate PR once core is stable.
- `--remote-path` flag to override inferred workdir: separate PR.

---

## Context & Research

### Relevant Code and Patterns

- `cmd/up/up.go` — entry point pattern to mirror: dependency injection in constructor, manifest loading in `RunE`, synthetic `upContext` struct, `start()` → `activateLoop()` → `activate()`.
- `cmd/up/activate.go` — activation loop; calls `apps.GetTranslations` → `apps.TranslateDevMode` → `secrets.Create` → `deployMainDev` → `apps.GetRunningPodInLoop`.
- `cmd/up/types.go:upContext` — context struct holding all state for a session.
- `cmd/up/up.go:loadManifestOverrides` — env var override pattern to follow.
- `cmd/up/up.go:addStignoreSecrets`, `addSyncFieldHash` — syncthing helpers; `cmd/connect` will need equivalent logic adapted locally.
- `pkg/model/dev.go:NewDev()` — safe constructor; always call before populating fields.
- `pkg/model/dev.go:SetDefaults()` — sets SSH port, command, image pull policy, timeouts; must be called before activation.
- `pkg/model/translation.go:TranslationRule` — controls what gets translated; does NOT need modification for Claude injection.
- `pkg/k8s/apps/translate.go:TranslateOktetoInitBinContainer` — pattern to follow for the Claude init container function.
- `pkg/k8s/apps/translate.go:tr.translate()` — called by `TranslateDevMode`; appends init containers when `rule.IsMainDevContainer()` is true.
- `pkg/k8s/apps/crud.go:IsDevModeOn` — checks `dev.okteto.com` label; used to detect reconnect and skip re-mutation.
- `main.go:164` — pattern: `root.AddCommand(up.Up(at, insights, ioController, k8sLogger, fs))`.

### Institutional Learnings

- `model.NewDev()` then `SetDefaults()` is the required construction sequence; skipping either causes runtime panics.
- `secrets.Create` must run after syncthing is initialized and before `TranslateDevMode`/`deployMainDev` — the secret must exist when the pod is scheduled.
- `apps.IsDevModeOn(app)` gates re-mutation: if dev mode is already on during a retry, the translation step is skipped, which also naturally prevents duplicate init container injection.
- Copyright header (Apache 2.0, 2023–2025) is required on every new `.go` file; `make lint` enforces it.

### External References

- None — all patterns are established locally in the codebase.

---

## Key Technical Decisions

- **Synthetic `model.Dev` without manifest**: Build via `model.NewDev()`, populate `Name`, `Image`, `Workdir`, `Sync.Folders`, `Environment`, then call `dev.SetDefaults()`. Wrap in a minimal `*model.Manifest{Dev: map[string]*model.Dev{name: dev}}`. This lets the existing `apps.GetTranslations` / `apps.TranslateDevMode` pipeline run unchanged. *(see origin: known implementation constraints #2)*

- **Claude init container via new exported function**: Add `TranslateClaudeCodeInitContainer(spec *apiv1.PodSpec)` to `pkg/k8s/apps/translate.go`. It appends a raw `apiv1.Container` (image: `curlimages/curl`, command downloads Claude CLI to `/okteto/claude-bin`, volume mount shares `okteto-claude-bin` emptyDir with the dev container). Call it in `connectContext.devMode()` after `apps.TranslateDevMode(trMap)` but before `deployMainDev`. No changes to `model.InitContainer`, `model.TranslationRule`, or `model.Dev`. *(see origin: known implementation constraints #1)*

- **No `model.Dev` / `model.TranslationRule` changes for Claude**: Claude-specific behaviour lives entirely in `cmd/connect`; the shared model and translate layers stay clean.

- **Init container deduplication on reconnect**: Handled naturally by `apps.IsDevModeOn(app)`. When reconnecting, `activate()` detects dev mode is already on (same as `cmd/up`) and skips `GetTranslations`+`TranslateDevMode`+`deployMainDev` — so no re-injection occurs. *(see origin: known implementation constraints #3)*

- **Workdir inference with error on empty**: Read `spec.containers[0].workingDir` from the K8s object after `utils.GetApp`. If empty, return a clear user error: "Could not infer working directory from deployment spec. Set `workingDir` in your container spec or add `--remote-path` in a future version." **Deliberate deviation from the requirements doc**, which specified a `/app` fallback — that fallback is rejected here because syncing to a non-existent `/app` produces a silent syncthing failure that is hard to debug. *(see origin: known implementation constraints #4)*

- **`PersistentVolumeInfo.Enabled = false`**: No PVC creation for a connect-only flow. Files live in an emptyDir; they are re-synced on reconnect.

- **Session lifecycle (no auto-teardown)**: `connectContext.start()` does not call `downOp.Down` on Ctrl+C. The dev container stays running. This mirrors `cmd/up`'s exit model.

- **`cmd/connect` is self-contained**: Does not import from `cmd/up`. Shares only exported packages (`pkg/k8s/apps`, `pkg/model`, `pkg/syncthing`, etc.).

---

## Open Questions

### Resolved During Planning

- **Can Claude init container be injected without changing `model.TranslationRule`?** Yes — call a new exported translate function after `TranslateDevMode` in the connect-specific activation code.
- **How is reconnect handled?** `apps.IsDevModeOn(app)` returns true on retry; the activation branch skips re-mutation. Syncthing reconnects via the same `sync()` path with `isRetry=true`.
- **Can `model.Dev` be built without a manifest?** Yes — `NewDev()` + field population + `SetDefaults()` is sufficient; wrap in a `*model.Manifest` for pipeline compatibility.

### Deferred to Implementation

- Exact syncthing rescan-interval and compression defaults for the connect flow (verify against `dev.SetDefaults()`).
- Whether `addSyncFieldHash` logic needs to be moved to a shared pkg or copied into `cmd/connect`.
- Runtime behaviour when the Claude download fails inside the init container (init container failure → pod pending; user sees K8s events).

---

## High-Level Technical Design

> *This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.*

```
okteto connect api
       │
       ▼
RunE (cmd/connect/connect.go)
  1. contextCMD.Run()          // resolve k8s context
  2. inferDevFromDeployment()  // fetch Deployment/SS → model.Dev
  3. syncthing upgrade check
  4. connectContext.start()
       │
       ▼
activateLoop() (cmd/connect/activate.go)
  ├─ utils.GetApp()            // find Deployment or StatefulSet
  ├─ IsDevModeOn?
  │   ├─ YES (reconnect): skip mutation, jump to sync
  │   └─ NO: GetTranslations → TranslateDevMode
  │                           → TranslateClaudeCodeInitContainer  ← NEW
  │                           → secrets.Create
  │                           → deployMainDev
  ├─ GetRunningPodInLoop()
  ├─ forwards (SSH only)
  └─ sync()                   // bidirectional file sync

Ctrl+C → shutdown() only (no downOp.Down)
```

---

## Output Structure

```
cmd/connect/
  connect.go       # Cobra command definition, Options struct, RunE
  context.go       # connectContext struct, start(), shutdown()
  activate.go      # activateLoop(), activate(), devMode(), inferDevFromDeployment()
  syncthing.go     # initializeSyncthing(), startSyncthing(), sync() — adapted from cmd/up
  connect_test.go  # unit tests
```

---

## Implementation Units

- [ ] **Unit 1: Claude Code init container translator**

**Goal:** Add an exported, idempotent function that appends the Claude Code installer init container to a pod spec.

**Requirements:** R3

**Dependencies:** None — pure function on `*apiv1.PodSpec`.

**Files:**
- Modify: `pkg/k8s/apps/translate.go`
- Test: `pkg/k8s/apps/translate_test.go`

**Approach:**
- Add `TranslateClaudeCodeInitContainer(spec *apiv1.PodSpec)` alongside `TranslateOktetoInitBinContainer`.
- Init container spec: name `okteto-claude-installer`, image `curlimages/curl:latest` (chosen over `alpine/curl` from the requirements doc — `curlimages/curl` is the maintained official image), command downloads the Claude CLI binary (via direct binary URL) into an emptyDir volume mounted at `/okteto/claude-bin`.
- Volume `okteto-claude-bin` (name chosen to avoid collision with the existing `okteto-bin` volume used by `TranslateOktetoInitBinContainer`) is an emptyDir added to `spec.Volumes`.
- The function must be idempotent: check whether a container named `okteto-claude-installer` already exists in `spec.InitContainers` before appending.
- The dev container's PATH mutation (prepending `/okteto/claude-bin`) is handled in Unit 2 when building the environment variables for the `connectContext`.

**Patterns to follow:**
- `pkg/k8s/apps/translate.go:TranslateOktetoInitBinContainer` — exact structural pattern for the new function.

**Test scenarios:**
- Happy path: calling on an empty `PodSpec` adds exactly one init container named `okteto-claude-installer` and one volume named `okteto-claude-bin`.
- Idempotency: calling twice on the same spec does not add a duplicate init container or volume.
- Existing containers: calling on a spec that already has other init containers appends without disturbing them.

**Verification:**
- `go test ./pkg/k8s/apps/...` passes.
- `make lint` passes.

---

- [ ] **Unit 2: `cmd/connect` package — command, context, activation**

**Goal:** Implement the full `okteto connect` command: Cobra wiring, dev inference from deployment, activation loop, session lifecycle.

**Requirements:** R1, R2, R4, R5, R6, R7

**Dependencies:** Unit 1 (`TranslateClaudeCodeInitContainer` must exist).

**Files:**
- Create: `cmd/connect/connect.go`
- Create: `cmd/connect/context.go`
- Create: `cmd/connect/activate.go`
- Create: `cmd/connect/syncthing.go`
- Create: `cmd/connect/connect_test.go`

**Approach:**

*`connect.go`* — Cobra command constructor following the same pattern as `cmd/up/up.go`:
- Function signature: `Connect(at analyticsTrackerInterface, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, fs afero.Fs) *cobra.Command`
- `Use: "connect [name]"`, `Args: cobra.ExactArgs(1)`
- Flags: `--image` / `-i`, `--namespace` / `-n`, `--context` / `-c`, `--env` / `-e`
- `RunE`: context resolution → `inferDevFromDeployment()` → syncthing upgrade check → `connectContext.start()`

*`inferDevFromDeployment(ctx, name, namespace, opts, k8sClient)`* — builds the synthetic `*model.Dev`:
1. `utils.GetApp(ctx, &model.Dev{Name: name}, namespace, k8sClient, false)` — fetches Deployment or StatefulSet.
2. Read `spec.containers[0].workingDir`; error if empty.
3. Call `model.NewDev()`, set `Name`, `Image` (from opts or deployment spec), `Workdir`, `Sync.Folders = [{LocalPath: os.Getwd(), RemotePath: workdir}]`, `PersistentVolumeInfo.Enabled = false`.
4. Apply env overrides from `--env` flag.
5. Call `dev.SetDefaults()`.
6. Return `&model.Manifest{Dev: map[string]*model.Dev{name: dev}}`.

*`context.go`* — `connectContext` struct (subset of `upContext`): `Namespace`, `Manifest`, `Dev`, `Exit chan error`, `Sy *syncthing.Syncthing`, `K8sClientProvider`, `isRetry`, `Cancel`, `Options`. `start()`: signal handler that calls `shutdown()` on Ctrl+C but does **not** call `downOp.Down`.

*`activate.go`* — `activateLoop()` / `activate()` following `cmd/up/activate.go` but simplified:
- No deploy step, no hybrid mode, no port forwards, no `pidController`, no `autoDown`.
- After `apps.TranslateDevMode(trMap)`, call `apps.TranslateClaudeCodeInitContainer` on the main dev app's pod spec.
- Prepend `/okteto/claude-bin` to `PATH` in dev container environment.
- Then: `secrets.Create` → `deployMainDev` → `GetRunningPodInLoop` → `startSyncthing` → `forwards` (SSH only) → `sync`.

*`syncthing.go`* — adapted from `cmd/up/syncthing.go`; only the `initializeSyncthing`, `startSyncthing`, and `sync` functions are needed.

**Execution note:** Implement `inferDevFromDeployment` and its unit tests first (it is the core new logic); then wire the activation loop.

**Patterns to follow:**
- `cmd/up/up.go` — command constructor, `RunE` structure, syncthing upgrade check.
- `cmd/up/activate.go` — `activate()` call sequence.
- `cmd/up/syncthing.go` — syncthing helpers.

**Test scenarios:**
- Happy path — `inferDevFromDeployment` with a deployment that has `workingDir=/app`: returns a `*model.Manifest` with `Dev["api"].Workdir == "/app"` and `Sync.Folders[0].RemotePath == "/app"`.
- Image override — `--image myimage:latest` sets `dev.Image = "myimage:latest"` regardless of deployment spec.
- Workdir empty — `inferDevFromDeployment` with a deployment whose `workingDir` is empty returns a `UserError` with a message directing the user to set `workingDir` in their container spec.
- Reconnect skip — when `apps.IsDevModeOn(app)` returns true, `activate()` skips translation and jumps directly to syncthing reconnect.
- Env override — `--env FOO=bar` sets `dev.Environment` with `FOO=bar`.
- Error on unknown deployment — `utils.GetApp` returns `not found`; `RunE` propagates a clear error.

**Verification:**
- `go test ./cmd/connect/...` passes.
- `okteto connect nonexistent` exits with a clear "not found" error.
- `make lint` passes.

---

- [ ] **Unit 3: Wire `cmd/connect` into `main.go`**

**Goal:** Register the new command in the root Cobra command so it appears in `okteto --help` and is callable as `okteto connect`.

**Requirements:** R1 (entry point)

**Dependencies:** Unit 2 (`cmd/connect` package must compile).

**Files:**
- Modify: `main.go`

**Approach:**
- Add import for `connectCMD "github.com/okteto/okteto/cmd/connect"`.
- Add `root.AddCommand(connectCMD.Connect(at, ioController, k8sLogger, fs))` after the `up.Up(...)` line (line 164).
- The constructor signature follows the same pattern as `up.Up` (no `insights` arg needed for v1).

**Test expectation:** none — pure wiring; integration is covered by Unit 2 tests and manual verification.

**Verification:**
- `okteto --help` shows `connect` in the command list.
- `go build ./...` succeeds.
- `make lint` passes.

---

## System-Wide Impact

- **Interaction graph:** No existing commands are modified. `TranslateClaudeCodeInitContainer` is a new exported function; it is not called by any existing code path.
- **Error propagation:** `inferDevFromDeployment` returns `oktetoErrors.UserError` for missing workdir (renders cleanly without stack trace). K8s API errors bubble up as-is.
- **State lifecycle risks:** `PersistentVolume: false` means synced files live in an emptyDir and are lost on pod restart. This is acceptable for v1; the user re-syncs via `okteto connect`.
- **API surface parity:** `okteto down <name>` already works against a dev-mode deployment; no changes needed to tear down.
- **Unchanged invariants:** `cmd/up`, `cmd/down`, and all other existing commands are unmodified. The `model.Dev`, `model.TranslationRule`, and `model.InitContainer` structs are unmodified.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Claude CLI download URL changes or is unavailable at init-container time | Pin to a specific release URL; document in code. Pod will stay pending with a clear init-container failure message visible via `kubectl describe pod`. |
| `syncthing.go` helpers in `cmd/up` are hard to adapt without duplicating | Copy the three needed functions (`initializeSyncthing`, `startSyncthing`, `sync`) into `cmd/connect/syncthing.go`; they are small and stable. |
| `addSyncFieldHash` is unexported in `cmd/up` | Re-implement equivalent logic in `cmd/connect/activate.go`; it is a simple hash annotation. |
| `SetDefaults()` has side effects not safe to call without a manifest path | Verify by reading `pkg/model/dev.go:SetDefaults` during implementation; the `devPath` passed to `PreparePathsAndExpandEnvFiles` can be `os.Getwd()` for the manifest-free case. |

## Sources & References

- **Origin document:** [docs/brainstorms/okteto-connect-requirements.md](docs/brainstorms/okteto-connect-requirements.md)
- Related code: `cmd/up/up.go`, `cmd/up/activate.go`, `cmd/up/types.go`, `pkg/k8s/apps/translate.go`, `pkg/model/dev.go`, `pkg/model/translation.go`
- External docs: none

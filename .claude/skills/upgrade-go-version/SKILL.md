---
name: upgrade-go-version
description: Upgrade the Go toolchain version of the Okteto CLI across both Go modules (root + tools), the Dockerfile base image (pinned by sha256), and the golang-ci CI image to a target version, validating with okteto build / okteto test, then opening a PR.
---

# Upgrade Go Version

Upgrade the Go toolchain used by the Okteto CLI to a target version, review the release
notes for breaking changes, make sure both Go modules still build and test green, then open
a PR.

## INPUT

- The target Go version comes from the prompt (e.g. `/upgrade-go-version 1.26.2`).
- If no version is provided, ask the user for the exact target version (`major.minor.patch`)
  **before** changing anything. The repo pins the patch level, so a bare `major.minor` is not
  enough.

## CONTEXT — where a Go version lives in this repo

This repo has **two** Go modules plus several pinned images. Only these hold the build
toolchain version:

- **`go.mod`** (module `github.com/okteto/okteto`) — the `go` directive.
- **`tools/go.mod`** (module `github.com/okteto/tools`) — the `go` directive.
- **`Dockerfile`** — `ARG GOLANG_VERSION` + `ARG GOLANG_SHA`, used by both the `tools-builder` and
  `golang-builder` stages (`golang:${GOLANG_VERSION}-bookworm@${GOLANG_SHA}`).
- **`.circleci/config.yml`** + **`okteto.yml`** — the `okteto/golang-ci:<ver>@sha256:...` image,
  **only** if the user provides a new golang-ci image (Step 8).

**Do NOT touch** these — they are not the build toolchain:

- `okteto/golang:1` floating tags (`okteto.yml` `dev`, `samples/`, `pkg/linguist/dev.go`)
- `okteto/golang:1.22` / `golang:1.20` strings inside `pkg/schema` tests (`schema_test.go`,
  `test_test.go`) and `integration/validate/manifests/` (e.g. `valid-movies.yml`) — they assert
  manifest parsing
- `samples/golang/go.mod`

## WORKFLOW

### 1. Determine current and target versions

- Read the current versions:

  ```bash
  grep -E '^go ' go.mod tools/go.mod
  grep GOLANG_VERSION Dockerfile
  ```

- Confirm both modules match. Classify the jump:
  - **Patch** (`1.25.11` → `1.25.12`): security / bug fixes only; breaking changes very unlikely.
  - **Minor** (`1.25.x` → `1.26.x`): new language, tooling, and stdlib behavior; breaking changes possible.
  - **Multiple minors** (`1.24.x` → `1.26.x`): review _every_ intervening minor.

### 2. Review release notes for breaking changes

Use `WebFetch` / `WebSearch` to read the notes for **each minor** between current and target:

- Per-minor release notes: `https://go.dev/doc/go1.26` (substitute the version)
- Full release history + per-patch security fixes: `https://go.dev/doc/devel/release`

Focus on: language changes, `go` command / tooling changes, removed or behavior-changed
standard-library APIs, deprecations, and minimum OS/arch requirements. Summarize the
risky items that could matter to a Go codebase.

### 3. Evaluate whether our code is affected

- Delegate a focused scan to the `explore` subagent (keeps the main context clean — see the
  subagent strategy in `.claude/CLAUDE.md`). For each risky item from Step 2, search **both**
  modules (root packages **and** `tools/`) for affected stdlib calls, language constructs,
  build tags, or `//go:` directives.
- Report anything affected or potentially affected, with `file:line` references. Decide what
  (if anything) needs changing — but make code edits only after the version bump, when the
  compiler can confirm (Step 7).

### 4. Bump the `go` directive in both modules

Set both modules to the exact target version, preserving the `major.minor.patch` precision the
repo already uses (run from the repo root — `go mod edit` takes the target file as its last arg):

```bash
go mod edit -go=<target> go.mod
go mod edit -go=<target> tools/go.mod
```

Editing the `go` line by hand is equivalent. **Do not** add a `toolchain` directive if none
exists today.

### 5. Update the Dockerfile base image and pin by sha256

- Set `ARG GOLANG_VERSION=<target>` (must match the `go` directive patch version).
- Resolve the **multi-arch manifest-list (index) digest** for `golang:<target>-bookworm` —
  the CLI is built for multiple arches, so a single-platform digest is wrong:

  ```bash
  docker buildx imagetools inspect golang:<target>-bookworm --format '{{.Manifest.Digest}}'
  # alternative: crane digest golang:<target>-bookworm
  ```

- Set `ARG GOLANG_SHA=sha256:<digest>` to the value returned above.

### 6. Tidy both modules

The version bump (and any module-graph changes it pulls in) can leave `go.mod` / `go.sum` out of
sync. Run `go mod tidy` in **both** modules so each is consistent before building:

```bash
go mod tidy                    # root module
cd tools && go mod tidy && cd ..
```

Review the resulting diff — bumping only the `go` directive should produce minimal churn, so
investigate anything unexpected before moving on.

### 7. Build to surface compilation errors, then fix

- **Preferred:** `okteto build` (builds the `cli` image from the Dockerfile remotely; any
  compilation error shows in the build output). Build a single service if needed: `okteto build cli`.
- **Local fallback** (no okteto context): `docker build -t okteto-cli:test .`.
- If the build fails, fix the code for the new Go version (apply the Step 3 findings). Read code
  before editing, keep changes minimal, preserve the existing Apache 2.0 license headers, and follow
  existing patterns. Re-build until green.

### 8. Ask about the golang-ci image (CI dependency)

Do this **before** running the tests: `okteto test unit` and the CI lint / unit-test jobs all run
**inside** the `okteto/golang-ci` image (pinned in the `.circleci/config.yml` executor and in
`okteto.yml` under `test.unit`). If that image's bundled Go toolchain is older than the new `go`
directive, CI will fail and the local run won't truly exercise the new toolchain — so settle the
image first.

Use `AskUserQuestion`: _"Is there an `okteto/golang-ci` image that already supports Go `<target>`?
If so, paste the full `version@sha256:...` reference."_

- **If provided:** replace **every** pinned occurrence (grep first so none are missed):

  ```bash
  grep -rn 'okteto/golang-ci:' .circleci/config.yml okteto.yml
  ```

  Update both to `okteto/golang-ci:<ver>@sha256:<digest>`. Step 9 then runs the tests on the new
  toolchain (since `okteto test unit` reads the image from `okteto.yml`).

- **If not provided:** warn the user clearly — _"CI (golangci-lint + unit tests) will fail until a
  golang-ci image supporting Go `<target>` is published and these references are updated."_
  You may still open the PR, but record this as an explicit **blocker** in the description, and note
  that the Step 9 results do not reflect the target toolchain.

### 9. Run the tests

- **Preferred:** `okteto test unit` (runs the `test.unit` suite → `make test`).
- ⚠️ This runs inside the `okteto/golang-ci` image from Step 8. If Step 8 produced an updated image,
  the suite exercises the new toolchain. If not, the image still ships the old toolchain (Go may
  auto-download the target via `GOTOOLCHAIN`, or silently run on the old one) — call this out in the
  results and re-run once the updated golang-ci image lands.
- **Local fallback:** `make test` (root) and `cd tools && make test`.

### 10. Open the PR

Only after build + tests pass (note any golang-ci caveat from Step 8).

- Use `AskUserQuestion`: _"Is there a Jira ticket associated with this upgrade? If so, paste the
  ticket key (e.g. `OKT-1234`) or its URL."_
  - **If provided:** include the ticket key in the PR **title** (suffix: `… (OKT-1234)`) and add a
    `Jira: <key or URL>` line near the top of the PR **description**.
  - **If not provided:** skip it — no placeholder.
- Make sure you are on a feature branch, not `master`: `git checkout -b chore/upgrade-go-<target>`.
- Commit signed (DCO), ending the message with a `Co-Authored-By` trailer for the running Claude model:

  ```bash
  git commit -s \
    -m "chore: upgrade Go to <target>" \
    -m "Co-Authored-By: <running Claude model> <noreply@anthropic.com>"
  ```

- Create the PR with the required labels (drop the `(<ticket>)` suffix if there is no ticket):

  ```bash
  gh pr create \
    --label "release/internal" \
    --label "run-e2e" \
    --title "chore: upgrade Go to <target> (<ticket>)" \
    --body "<description>"
  ```

#### Required labels

- `release/internal`
- `run-e2e`

#### PR description must include

- The Jira ticket (`Jira: <key or URL>`) if the user provided one.
- Old → new Go version (both modules + Dockerfile `GOLANG_VERSION`).
- Dockerfile base image + the new `sha256` digest.
- golang-ci image: updated to `<ref>`, **or** a clear **blocker** note that it still needs a
  Go-`<target>` image before CI can pass.
- Breaking changes reviewed from the release notes, and why they do / don't affect us.
- Any code changes made to compile or pass tests on the new version.
- Build result (`okteto build`) and test result (`okteto test unit`).
- End the PR body with the Claude Code attribution footer.

## IMPORTANT CONSTRAINTS

- Keep `go.mod`, `tools/go.mod`, and the Dockerfile `GOLANG_VERSION` on the **same** version.
- Always pin the Dockerfile base image by `sha256` (the multi-arch manifest-list digest).
- Never modify sample or test-fixture Go versions (see CONTEXT).
- Read before edit; keep changes minimal. Any new `.go` file needs the Apache 2.0 header from
  `.copyright-header.tmpl` with a year accepted by the `goheader` linter — match the format of
  existing files rather than assuming a fixed year range.
- Sign every commit (`git commit -s`).
- If the upgrade goes sideways (build won't pass, unexpected breakage), STOP and re-plan rather
  than pushing through.

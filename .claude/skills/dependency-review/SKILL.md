---
name: dependency-review
description: Use when a pull request changes go.mod or go.sum in this repo (the root module or tools/), such as a Renovate dependency bump, and needs a dependency-safety review with a 0-10 score
compatibility: Requires the okteto CLI with OKTETO_CONTEXT and OKTETO_TOKEN set (tests and the image build run in Okteto, so Go itself is not needed), git, curl, unzip, and gh authenticated with write access to okteto/okteto pull requests and contents
argument-hint: '[pr-number] [dry-run]'
triggers: ['user']
disable-model-invocation: true
---

# Dependency-safety review for Go module changes

Review a PR that changes Go module dependencies. First **research the actual
changes** (identify them, then gather evidence on what moved between the old
and new versions), then answer one question: **will the code paths this
repository actually uses keep working after these changes?** Judge breaking
changes, not style. Post the answer as a 0-10 score and approve the PR when it
is safe.

## Arguments

Arguments: `$ARGUMENTS`

- The first number is the PR number. If there is none, take it from the GitHub
  event payload appended to the prompt (`pull_request.number`). If there is
  neither, ask for it.
- **Automatic mode:** the PR number came from the event payload (the Devin
  automation). **Manual mode:** the PR number was passed as an argument.
- **Dry run:** `dry-run` appears in the arguments. Do the full review, including
  writing and running tests, but commit nothing and write nothing to GitHub
  (see Step 8).

## Workflow

Do every GitHub operation with `gh api` (REST) and `git`, as written below.
Devin's shell disables `gh pr` subcommands; `gh api` works in both Devin and
Claude Code. `<n>` is the PR number; `<sha>`, `<head>`, and `<base>` come from
Step 1; `<short-sha>` is `git rev-parse --short <sha>`.

Go is not needed locally. The unit tests run in the CI image with
`okteto test unit --timeout 30m` (the `unit` test container: `make test`, with
`-race` and coverage, for the root module), and the production image builds
with `okteto build cli` (the `Dockerfile`: the CLI plus the `remote`,
`supervisor`, and `clean` binaries from `tools/`). Both upload the local
repository, so they see local edits such as new tests or a swapped `go.mod`.
Always pass `--timeout 30m` to `okteto test`: its 5-minute default is too short.

Every run that gets past Step 2 ends with a comment on the PR saying what
happened, including runs that stop early; a dry run writes it to a file instead
(Step 8). Runs that stop at Step 1 or Step 2 stay silent: the PR is not one to
review, or its head commit already has a review.

### Step 1: Check the PR qualifies

```bash
gh api repos/okteto/okteto/pulls/<n> \
  --jq '{state, author: .user.login, head: .head.ref, sha: .head.sha, base: .base.ref}'
gh api --paginate repos/okteto/okteto/pulls/<n>/files --jq '.[].filename'
```

Continue only if all of these hold; otherwise stop without writing anything to
GitHub (in manual mode, say why):

- `state` is `open`.
- At least one changed file is `go.mod`, `go.sum`, `tools/go.mod`, or
  `tools/go.sum`.
- Automatic mode only: the PR comes from Renovate (author `oktetobot`, head
  `renovate/*`, which includes the security updates on
  `renovate/go-*-vulnerability`).

### Step 2: Skip commits already reviewed

A report starts with the marker `<!-- dependency-review sha=<sha> -->` when its
outcome is final for that commit. Stop if a PR comment already carries the
marker for the current head commit. In a dry run, report the result and
continue.

Leave the marker out when the outcome may change without a new commit, so a
later run can review the same commit again: the "Could not run" report, and a
"Not approved" report whose only reason is a CI check still pending, a command
that could not run, or a step that failed to publish.

```bash
gh api --paginate repos/okteto/okteto/issues/<n>/comments --jq '.[].body' \
  | grep -F "dependency-review sha=<sha>"
```

### Step 3: Check the environment and check out the PR

If `git rev-parse --is-shallow-repository` prints `true`, run
`git fetch --unshallow origin` first: the base comparisons need the merge base.

```bash
okteto ctx show
git fetch origin "+refs/heads/<base>:refs/remotes/origin/<base>" \
  "+refs/pull/<n>/head:refs/remotes/origin/pr-<n>"
git checkout --detach origin/pr-<n>
git rev-parse HEAD
```

If `okteto ctx show` fails (no context or token), stop and publish (Step 8) the
short "Could not run" report from section 5 instead of the full one. It has no
marker, so a later run can still review this commit.

`refs/pull/<n>/head` works for pull requests from forks too, and the detached
checkout leaves no local branch behind (Step 8 pushes with `HEAD:<head>`).

`git rev-parse HEAD` must equal `<sha>`. If it differs, Renovate pushed in
between: in automatic mode stop without commenting (the new push triggers its
own run); in manual mode restart from Step 1.

### Step 4: Review

Apply rubric sections 1-2 to every changed module.

### Step 5: Test the changed APIs

If the head commit's subject starts with `test(deps): cover`
(`git log -1 --format=%s`), this run is the re-review after this skill pushed
tests: write no more tests and list any remaining gaps under **Actions**.

Otherwise, list the APIs of the changed **direct** dependencies of the root
module that production code calls and that the evidence from section 2 shows
changed: signature, behavior, default, or deprecation. If there are none, skip
this step: section 4 needs no new tests then. A dependency used only for its
types has no call site to test. Only the root module gets tests: no test
container runs the tests of `tools/`. For each listed API whose call sites no
test exercises:

1. Write tests in `_test.go` files next to the code, following
   `.claude/context/testing.md` (testify `require`, no branching in test
   bodies, table-driven when only inputs vary).
2. Fake the remote side; never reach a real service or use real credentials.
   For clients of a remote API, point the client at an `httptest.Server` that
   returns canned responses, so the library's own request building and
   response parsing run. Use one fake server per remote service, so handlers
   need no branching.
3. Change only test files, using only modules already in `go.mod`.
   Configuration the code already reads (environment variables, config files,
   an endpoint option) counts as injectable; set variables with `t.Setenv` and
   isolate the rest of the environment. If a call site cannot be tested without
   changing production code, write no test for it and list it under
   **Actions**.
4. Run them with `okteto test unit --timeout 30m` on the PR head and on the base
   revision (swap `go.mod` / `go.sum` as in section 3). Each run is the whole
   unit suite and takes several minutes, and compile errors only show up in its
   output: write the tests carefully, formatted as `gofmt` would (tabs for
   indentation), and read the output fully before trying again:
   - Pass on both: evidence the used paths behave the same.
   - Pass on base, fail on head: a breaking change; score it per section 4.
   - Fail on base: the test is wrong; fix it.
5. Commit all the new test files in a single commit on the PR branch, without
   pushing yet (Step 8): `git add <new _test.go files>`, then
   `git commit -m "test(deps): cover <dependency> call sites"`. Skip the commit
   in a dry run.

Push tests only to Renovate PRs. For any other PR, keep the tests local, use
their results as evidence, and list the missing tests under **Actions**.

### Step 6: Verify and score

Apply rubric sections 3-4. `okteto test unit` now includes the tests from
Step 5.

### Step 7: Decide

Approve only when **all** of these hold:

- The overall score is **9 or higher**.
- The section 3 verify commands ran in this session and passed for every
  changed module.
- `ci/circleci: golangci-lint` has succeeded on `<sha>`: lint does not run in
  this session, and a bump can bring new deprecation findings. If it is still
  pending, check again every few minutes for up to 20 minutes; if it has not
  finished by then, do not approve and give that as the reason in the report.
- No check on `<sha>` has failed. CircleCI reports commit statuses and GitHub
  Actions reports check runs, so read both. When a check has several runs,
  only the most recent one counts, i.e. the highest `id` across all pages (e.g.
  a cancelled `run-e2e / trigger` followed by a successful one). The combined
  status already keeps only the latest status per context:

  ```bash
  gh api --paginate "repos/okteto/okteto/commits/<sha>/check-runs?per_page=100" \
    --jq '.check_runs[] | [.id, .name, .status, (.conclusion // "-")] | @tsv' \
    | sort -t "$(printf '\t')" -k2,2 -k1,1n \
    | awk -F '\t' '{latest[$2] = $0} END {for (n in latest) print latest[n]}'
  gh api repos/okteto/okteto/commits/<sha>/status \
    --jq '.statuses[] | [.context, .state] | @tsv'
  ```

  A completed check run blocks approval unless it concluded `success`,
  `neutral`, or `skipped` (so `failure`, `timed_out`, `cancelled`,
  `action_required`, `startup_failure`, and `stale` all block), and a status
  blocks if it is `failure` or `error`. Check runs still in progress and
  `pending` statuses do not: the e2e jobs (`e2e-*`, started by the `run-e2e`
  label) take longer than the review, so they block only once they have
  failed.

This applies to every run, in automatic and manual mode alike; a manual run
approves with the `gh` account of whoever runs it.

Before dismissing or publishing anything, read the PR's head commit again
(`gh api repos/okteto/okteto/pulls/<n> --jq .head.sha`). The review takes many
minutes, and Renovate may have pushed meanwhile. If it is no longer `<sha>`,
this run is stale: in automatic mode stop without dismissing or publishing (the
new push triggers its own run); in manual mode say so and restart from Step 1.

When the decision is not to approve, dismiss earlier approvals made by this
skill (an `APPROVED` review whose body starts with `Dependency review:`). In a
dry run, only list their IDs for the `Would:` line:

```bash
gh api --paginate repos/okteto/okteto/pulls/<n>/reviews \
  --jq '.[] | select(.state=="APPROVED" and ((.body // "") | startswith("Dependency review:"))) | .id'
gh api -X PUT repos/okteto/okteto/pulls/<n>/reviews/<id>/dismissals \
  -f message="Superseded by the dependency review of <short-sha>" -f event=DISMISS
```

If the dismissal fails, say so in the report.

### Step 8: Publish

Write the report in the rubric section 5 format to
`/tmp/dependency-review-<n>-<short-sha>.md` (outside the repository, so it is
never committed). Its marker and the reviewed commit it names are `<sha>`, the
commit this run reviewed.

- **Dry run:** do not post it. Append one last line with what a real run would
  have done (e.g. `Would: comment + approve + push 2 test files`,
  `Would: comment`, or `Would: comment + dismiss approval <id>`), then finish
  by giving the file path. Stop.
- **Otherwise,** always post it as a PR comment, then approve and push when the
  earlier steps say so, in this order:

```bash
gh api repos/okteto/okteto/issues/<n>/comments -F body=@/tmp/dependency-review-<n>-<short-sha>.md
gh api repos/okteto/okteto/pulls/<n>/reviews -f event=APPROVE -f commit_id=<sha> \
  -f body="Dependency review: <N>/10 at <short-sha>. See the report comment."   # only if Step 7 approves
git push origin HEAD:<head>                 # only if Step 5 committed tests
```

If the approval fails after the comment was posted, edit the comment
(`gh api -X PATCH repos/okteto/okteto/issues/comments/<comment-id> -F body=@<file>`)
to drop the marker and give the failure as the reason for not approving.

When Step 5 pushed tests, the push creates a new head commit and the automation
runs this skill again on it. That run is not stopped by Step 2 (the marker
names the previous commit), adds no further tests (Step 5), and posts the new
result for the pushed commit. The approval given above covers `<sha>`, not the
commit with the tests; if branch protection dismisses stale approvals, the push
cancels it, and the re-review approves the new commit if it still qualifies.

### Never

- Merge or close the PR.
- Commit or push anything other than the Step 5 test files, or push to any
  branch other than the PR's. Local edits made for base comparisons are
  expected; restore them with `git checkout HEAD -- <go.mod> <go.sum>`.
- Run `okteto deploy`, `okteto test --deploy`, or any test container other than
  `unit`.
- Request changes; a low score is reported in the comment only.
- Write to GitHub in a dry run, or for a run that stopped at Step 1 or Step 2.

# Rubric

This repo has **two independent modules**, each with its own `go.mod` and no
`go.work` workspace. Scope your analysis per changed module and verify each one
separately:

- `github.com/okteto/okteto` — `go.mod` (the CLI)
- `github.com/okteto/tools` — `tools/go.mod` (the `remote`, `supervisor`, and
  `clean` binaries shipped in the CLI image)

A bump landing green in CI is not by itself proof the code paths this repo uses
still behave.

## Evidence rules (read first)

- Use evidence in this priority order, and cite which you used for every claim:
  **(1)** the PR diff; **(2)** this repository's own code (imports/call sites);
  **(3)** the module's release notes / CHANGELOG / tagged VCS diff /
  pkg.go.dev / its source from the Go module proxy; **(4)** changelog links the
  bot injected into the PR body.
- **Never assert an API, signature, default, or version delta without one of
  those sources.** Do not rely on unverified recall of "what changed" in a
  release. If you cannot establish the old or new version or the delta from real
  evidence, say so and **cap that dependency in the Caution band** (see scoring).
- Prefer claims you can back with a command you actually ran.
- To get source (3), download both versions from the Go module proxy into an
  empty scratch directory and unzip them:
  `curl -sSfL -o old.zip https://proxy.golang.org/<module>/@v/<old>.zip` (same
  for `<new>`), then `unzip -q old.zip -d old`. In `<module>`, write each
  uppercase letter as `!` plus its lowercase (`github.com/Masterminds` →
  `github.com/!masterminds`). Read `CHANGELOG.md` and `diff` the files that
  define the APIs this repo uses. In monorepo CHANGELOGs, read only the entries
  of the changed modules, looking for breaking, behavior, default, deprecation,
  and removal notes.

## 1. Research the changes (per changed module)

- Determine old → new for every dependency. Read the PR diff:
  `git diff origin/<base>...HEAD -- go.mod go.sum tools/go.mod tools/go.sum`.
  Mark each as **direct** or **indirect** (the `// indirect` comment), and mark
  **adds** (old = `(new)`) and **removals**.
- Flag any change to the `go` directive, a `toolchain` line, or `replace` /
  `exclude` entries. Read the current values from each `go.mod` on the base
  branch; a newly added `toolchain` or `exclude` line is noteworthy.
- Check `go.sum` against `go.mod`: every required module should resolve to a
  matching hash with no **unexplained** extra or missing entries. Validate the
  hashes of a `replace`d module against its **resolved target**
  (`github.com/okteto/buildkit`, not `github.com/moby/buildkit`); that is not a
  defect. Unexplained hash churn on an otherwise-unchanged version is a
  supply-chain red flag.
- **No-op short-circuit:** if the only change is `go.sum` reordering with no
  version delta, score it 10 and skip to the report — keep it to a few lines.
  Any version change gets the full review: a transitive patch can still change
  behavior reached through a direct dependency.

## 2. Assess each changed dependency

- **Semver jump:** classify patch / minor / major. Majors are high-risk by
  default. Versions marked `+incompatible` (e.g. `github.com/docker/cli`) do
  not follow semantic import versioning: read their release notes as if every
  bump could be a major.
- **Go semantic import versioning:** v2+ modules import under a `/vN` path. For
  a **direct, imported** module bumped across a major with **no `/vN` import
  paths or call sites changed** in the PR, the code either won't compile or
  silently keeps the old version — flag it as a hard failure. An
  **indirect-only** major bump needs no import change here; judge it through
  the direct dependency that pulls it in (see the usage bullet), not as a hard
  failure. The rule does not apply to `+incompatible` versions (e.g.
  `github.com/docker/cli`, imported without `/vN`): a major bump keeps the same
  import path, so judge it on its release notes. For `gopkg.in/<name>.vN`, the
  major is part of the module path: a new major shows up as a different module
  (an add and a removal), not as a bump; grep for the `.vN` path.
- **Repository usage (make this concrete):** grep the repo for the module's
  **import path** (which for v2+ is `.../vN`, not the bare module path), e.g.
  `grep -rn "<import-path>" --include='*.go' .`. Classify each hit as
  **production** or **`_test.go`-only** (test-only has a smaller blast radius),
  and note whether it is in the root module or in `tools/`. For an **indirect**
  dep with no direct import, there are no call sites here: judge it as a
  behavior-only risk routed through the direct dependency that requires it, not
  automatically "safe" because grep finds nothing.
- **API / behavior deltas:** from the evidence sources above, identify removed,
  renamed, or re-signatured functions, changed defaults, and deprecations, and
  map each to the files that use them.
- **Suspicious moves:** call out downgrades of a used dep, pseudo-versions
  pointing at raw commits, and new/modified `replace` directives — each in one
  line.
- **The buildkit fork:** the `replace` of `github.com/moby/buildkit` with
  `github.com/okteto/buildkit` (a pseudo-version, commented
  `<upstream>-okteto<n>`) points at Okteto's fork, which carries patches the
  CLI relies on; Renovate ignores
  `github.com/moby/buildkit` on purpose. Removing this `replace` is a hard
  failure. Changing its target is a human decision: score it in the 4-5 band
  and say so. A bump of a module that the fork also requires (check the fork's
  `go.mod` at the pinned commit) can conflict with it: `okteto build cli` must
  pass, and mention the overlap.
- Any **newly added** `replace` is noteworthy regardless of target.
- **Security & retractions:** a bump that raises a known-vulnerable dep to a
  fixed version is a positive (Renovate's `[SECURITY]` PRs do this); introducing
  a known-vulnerable, downgraded, or `retract`-ed version is a strong negative.
  For known vulnerabilities, query OSV for each changed version, old and new:
  `curl -sSf https://api.osv.dev/v1/query -d '{"package":{"name":"<module>","ecosystem":"Go"},"version":"<version>"}'`.
  For retractions, read the `retract` directives in the `go.mod` of the
  module's latest version:
  `curl -sSfL https://proxy.golang.org/<module>/@latest` gives the version, then
  `curl -sSfL https://proxy.golang.org/<module>/@v/<version>.mod`.
- **`go` directive / toolchain:** a bump of the `go` line can change language
  semantics and require a newer toolchain than CI and the Dockerfile provide.
  Verify the new version against `GOLANG_VERSION` in the `Dockerfile`, the
  `golang-ci` image of the `unit` test container in `okteto.yml`, and the
  `golang-ci` executor in `.circleci/config.yml`; score a bump beyond the
  available toolchain in the Unsafe band. Planned Go upgrades go through the
  `upgrade-go-version` skill.

## 3. Verify

Verify each changed module (one whose `go.mod` or `go.sum` the PR changes):

- **Root module:**
  - `okteto test unit --timeout 30m` runs `make test`
    (`go test -race -coverprofile ./...`) in the CI image. Packages with tests
    are compiled and tested; coverage also compiles the packages without tests
    (they report `coverage: 0.0%`).
  - `okteto build cli` builds the production image from the `Dockerfile`,
    which compiles the CLI with `make build`.
- **`tools/`:** `okteto build cli` compiles `remote`, `supervisor`, and
  `clean`. Nothing runs the tests of `tools/` (neither this session nor CI); say
  so in the report when `tools/` changed.

Not run in this session, covered by CI: lint (`ci/circleci: golangci-lint`,
required by Step 7), the Windows unit tests (`run-windows-unit-test`), the
schema check (`check-schema`), and the e2e suites. `go mod tidy` is not checked
anywhere: Renovate runs it when it updates (`postUpdateOptions: gomodTidy`);
for other PRs, say it was not verified.

To compare with the base revision, use the merge base, not the tip of
`<base>`, which may have moved on (even to the same dependency version):
`B=$(git merge-base origin/<base> HEAD)`, then
`git checkout "$B" -- <go.mod> <go.sum>` for the changed module, run the same
command again, and restore with `git checkout HEAD -- <go.mod> <go.sum>`.

A build or test failure **attributable to the dependency change** forces the
0-1 band — but first rule out a pre-existing, unrelated, or environment failure
(confirm the base revision fails the same way). A failure you cannot tie to the
change lowers your **confidence**, not the score. If a command cannot run, say
so in the report; the PR is then not eligible for approval (Step 7).

## 4. The 0-10 safety score (mandatory)

Report a single integer **0-10** where **0 = certainly breaks a code path this
repo uses, or introduces a confirmed unsafe security posture** and **10 =
certainly safe** (compatible, and no known-vulnerable regression), plus a
one-line justification (the report's summary sentence) and a confidence
(high/medium/low, at the end of "How it was verified"). Anchors:

- **10** — no functional change possible: `go.sum` **reordering** with no
  version or hash-content delta. (Unexplained checksum churn is a red flag, not
  a 10.)
- **8-9** — patch/minor bumps, all directives unchanged, build and tests pass,
  and every API the repo uses either unchanged or changed in a way shown to be
  compatible, all backed by evidence. Score **9** when every used API that the
  evidence shows changed (signature, behavior, default, or deprecation) has its
  call sites exercised by tests (existing or from Step 5) that pass on both base
  and head. If no API the repo uses changed, nothing needs extra tests: the
  existing suite passing on head is enough for 9. Score **8** when some changed
  API's call sites are not exercised by such tests (e.g. they cannot be tested
  without changing production code, or they live in `tools/`).
- **6-7** — minor bump to a heavily-used dep with only partially verified
  changes, OR an indirect major bump not on any used path.
- **4-5** — a major (v2+) bump whose `/vN` imports were correctly updated and
  used APIs appear compatible but aren't fully verified; OR a new/modified
  `replace` (including a new target for the buildkit fork), a pseudo-version,
  or a `go`-directive bump still within the available toolchain.
- **2-3** — a major bump likely touching a used API, a downgrade of a used dep,
  a `go`-directive bump beyond the CI/Docker toolchain, or a newly introduced
  known-vulnerable/retracted version.
- **0-1** — confirmed removal/signature change of a used **direct** API, a
  direct v2+ bump **without** the required `/vN` import update, removal of the
  buildkit fork `replace`, or an `okteto build cli` or test failure
  reproducibly caused by the change.

Two rules keep the number honest:

1. **The overall score is the MINIMUM** of the per-dependency scores across
   every changed module — one broken dep caps the whole PR.
2. **Confidence caps the score:** if a dep's version delta can't be verified
   from real evidence, it **cannot score above 5**, no matter how benign it
   looks. This eliminates "safe but unsure".

Label from the band (secondary to the number): **8-10 Safe**, **4-7 Caution**,
**0-3 Unsafe**.

## 5. Report format

Write for a reviewer who has 30 seconds. Use plain English and short sentences,
and none of this skill's terms ("source (3)", "band", "rubric", "section").
Keep the visible part to about 10 lines; everything else goes in collapsed
`<details>` blocks. Follow this template, leaving out the lines and blocks
marked as conditional when they do not apply:

```markdown
<!-- dependency-review sha=<sha> -->

### <headline>

<One or two sentences: what is updated, in plain words, and whether anything
this repo uses changed.>

- **What we use:** <what the repo relies on from these dependencies, and where>.
- **Checked:** <what was verified, in one line>.
- **Tests added:** <what they exercise>, commit `<short-sha of the test commit>`.
  A review of that commit follows; Renovate will no longer update this
  branch. <!-- only if Step 5 pushed tests -->
- **Needs your attention:** <exactly what to check, and where>.
  <!-- only if not approved -->

Approved at `<short-sha>`. | Not approved: <reason>. Reviewed `<short-sha>`.

<details><summary>Dependencies (<N> direct, <M> indirect)</summary>

| Dependency | Version       | Used in                   | Risk                |
| ---------- | ------------- | ------------------------- | ------------------- |
| <module>   | <old> → <new> | <packages, or "indirect"> | low / medium / high |

</details>

<details><summary>Release-note changes reviewed</summary>

- <each change that touches an API the repo uses, why it is or is not a
  problem, and where it comes from (CHANGELOG link or file)>

</details>

<details><summary>How it was verified</summary>

- <each command run, with pass/fail on head, and on base when compared; anything
  not run, and why>
- Confidence: <high | medium | low>

</details>

<details><summary>Not caused by this PR</summary>   <!-- only if any -->

- <pre-existing failures or vulnerabilities, with what would fix them>

</details>
```

Headlines:

| Situation                       | Headline                           |
| ------------------------------- | ---------------------------------- |
| Approved                        | `✅ Safe to merge — <N>/10`        |
| Not approved, score 4 or higher | `⚠️ Needs a human review — <N>/10` |
| Score 0-3                       | `❌ Breaks this repo — <N>/10`     |
| Stopped in Step 3               | `⛔ Could not run`                 |

Table rows: one per direct dependency. Indirect dependencies that move, appear,
or disappear only because a bumped direct dependency requires them go in that
row's "Used in" cell (e.g. `indirect: +12 (golang.org/x/net, …)`); give one its
own row only when it carries risk of its own. Name a dependency's own score
only when it is lower than the overall one.

The "Could not run" report is only the headline and one line saying what is
missing and who needs to fix it (e.g. "Devin's environment has no
`OKTETO_TOKEN`; an admin needs to add it"), with no marker.

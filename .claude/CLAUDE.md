# Okteto CLI

A Go CLI tool for Kubernetes development — syncs local code changes to remote clusters in real time.

## Workflow Guidelines

### 1. Plan Mode Default
- Enter plan mode for ANY non-trivial task (3+ steps or architectural decisions)
- If something goes sideways, STOP and re-plan — don't keep pushing
- Use plan mode for verification steps, not just building

### 2. Subagent Strategy
- Use subagents to keep main context window clean
- Offload research, exploration, and parallel analysis to subagents
- One focused task per subagent

**Available agents** (`.claude/agents/`):

**Usual workflow**:
```
1. explore        → find relevant code
2. reproducer     → run command, save "before" snapshot (confirms bug is reproducible)
── Claude principal fixes the bug ──
   └── test-writer → write tests for changed code
3. diff-reviewer  ┐
   test-verifier  ┘ in parallel
   └── if missing tests → test-writer → test-verifier
4. verify         → scoped go test + make lint
5. committer      → only if steps 3+4 are LGTM
6. reproducer     → same command, save "after" snapshot, diff vs before (confirms fix)
7. pr-writer      → draft → user approves → create PR
```

### 3. Self-Improvement Loop
- After ANY correction from the user: update `.claude/lessons.local.md` with the pattern
- Write rules that prevent the same mistake
- Review lessons at session start when relevant
- Verify if those corrections should be on one of the claude files (agents, context, skills...)

### 4. Verification Before Done
- Never mark a task complete without proving it works
- Run tests, check logs, demonstrate correctness
- Ask: "Would a staff engineer approve this?"

### 5. Demand Elegance (Balanced)
- For non-trivial changes: pause and ask "is there a more elegant way?"
- If a fix feels hacky: implement the elegant solution instead
- Skip for simple, obvious fixes — don't over-engineer

### 6. Autonomous Bug Fixing
- When given a bug report: just fix it — no hand-holding needed
- Point at logs, errors, failing tests — then resolve them

## Task Management

1. **Plan First**: Write plan to `.claude/tasks.local/todo.md` with checkable items
2. **Track Progress**: Mark items complete as you go
3. **Capture Lessons**: Update `.claude/lessons.local.md` after corrections — or run `/lessons`

## Core Principles

- **Simplicity First**: Make every change as simple as possible. Minimal impact.
- **No Laziness**: Find root causes. No temporary fixes. Senior developer standards.
- **Read Before Write**: Always read existing code before modifying it.

## Strict Rules

### NEVER:
- Invent new CLI commands, flags, or arguments without explicit requirements
- Modify code without reading it first
- Skip copyright headers (all `.go` files require Apache 2.0 header, years 2023-2025)
- Use `panic()` in production code — return errors instead
- Break existing CLI interfaces without discussion
- Add branching (`if/switch`) inside test function bodies — split into multiple tests

### ALWAYS:
- Run `make lint` before considering changes complete
- Write tests for new functionality or bug fixes
- Sign commits with `git commit -s` (DCO requirement)
- Use existing patterns from similar code as templates

## Priority Rules

1. **Follow this file**
2. **Follow existing code patterns**
3. **Ask before inventing new patterns**

## Context References

> **Architecture & structure**: [context/architecture.md](.claude/context/architecture.md)
> **Go conventions & CLI behavior**: [context/conventions.md](.claude/context/conventions.md)
> **Command implementation patterns**: [context/commands.md](.claude/context/commands.md)
> **Testing patterns & rules**: [context/testing.md](.claude/context/testing.md)
> **Build, lint & release**: [context/build-and-release.md](.claude/context/build-and-release.md)
> **CVE remediation**: use `/fix-cves` skill — see [skills/fix-cves/SKILL.md](.claude/skills/fix-cves/SKILL.md)
> **Self-improvement**: use `/lessons` skill — see [skills/lessons/SKILL.md](.claude/skills/lessons/SKILL.md) | log at [lessons.md](.claude/lessons.local.md)

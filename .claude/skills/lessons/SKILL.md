---
name: lessons
description: Record a lesson learned after a user correction, or review existing lessons at session start
---

# Lessons Skill

Manages the self-improvement loop. Use this skill when:
- The user corrects a mistake or gives feedback that should change future behavior
- You want to review past lessons at the start of a session
- After completing a task, to capture any patterns worth preserving

## Lesson file

All lessons live in `.claude/lessons.local.md`.

## Workflow: Recording a Lesson

### Step 1: Read the current lessons file

```bash
cat .claude/lessons.local.md
```

### Step 2: Identify the lesson

From the user correction, extract:
- **Mistake**: What went wrong (be specific, not generic)
- **Pattern**: The concrete rule to apply going forward
- **Trigger**: The condition that should activate this rule

### Step 3: Append the entry

Add a new entry at the bottom of `.claude/lessons.local.md`, above the closing comment:

```markdown
## 2026-03-17 — [short title]
**Mistake**: [What went wrong]
**Pattern**: [Rule to apply going forward]
**Trigger**: [When to apply this rule]
```

Use absolute dates (YYYY-MM-DD), never relative ("today", "last week").

### Step 4: Confirm

Report the new entry to the user briefly:

```
Lesson recorded: [short title]
```

## Workflow: Reviewing Lessons

At the start of a session, or when asked:

1. Read `.claude/lessons.local.md`
2. Identify lessons relevant to the current task
3. Internalize them — do not re-state them all back to the user unless asked
4. Apply them silently

## Workflow: Pruning Lessons

Lessons become stale when:
- They reference a pattern that's been fixed in the code
- They duplicate another lesson
- They're too generic to be actionable

When pruning, remove the entry entirely. Don't leave tombstones.

## Rules

- Never invent a lesson that wasn't triggered by an actual correction
- Be specific: "always read the file before editing" is better than "be careful"
- One lesson per entry — don't bundle unrelated corrections together
- Update an existing lesson if a correction refines a known pattern rather than creating a duplicate

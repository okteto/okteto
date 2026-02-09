---
name: new-feature
description: Start a new feature development session with branch setup and requirements gathering
disable-model-invocation: true
---

# New Feature Development Workflow

You are helping the user start development on a new feature for the Okteto CLI.

## Step 1: Branch Setup

1. **Check current branch**: Run `git branch --show-current`
2. **Switch to master**: Run `git checkout master` (if not already there)
3. **Pull latest code**: ALWAYS run `git pull origin master` to ensure we have the latest changes
4. **Create new branch**: Ask user for branch name using AskUserQuestion:
   - **Question**: "What should the new branch be named?"
   - **Header**: "Branch Name"
   - **Options**:
     - `feat/[feature-name]` (Recommended for new features)
     - `fix/[bug-name]` (For bug fixes)
     - `refactor/[description]` (For refactoring)
     - Custom (let user specify)
5. **Create and switch**: Run `git checkout -b [branch-name]`

## Step 2: Requirements Form

Use AskUserQuestion to gather all requirements in one structured form:

### Question 1: Feature Type

- **Header**: "Type"
- **Question**: "What type of feature are you adding?"
- **Options**:
  - New CLI command (e.g., `okteto newcmd`) (Recommended for new features)
  - Modify existing command
  - New package/library functionality
  - Internal tool (remote/supervisor/clean)

### Question 2: Scope

- **Header**: "Scope"
- **Question**: "How would you describe the scope of this feature?"
- **Options**:
  - Small - Single function or simple change
  - Medium - New command or significant enhancement (Recommended)
  - Large - Multi-file changes with architectural impact
  - Unknown - Need to explore first

### Question 3: User-Facing

- **Header**: "Visibility"
- **Question**: "Is this feature user-facing or internal?"
- **Options**:
  - User-facing - End users will interact with it (Recommended)
  - Internal - For maintainers/contributors only
  - Both - Has user and internal components

## Step 3: Get Feature Description

After the structured questions, ask:
**"Please describe what you want to implement. Include any specific requirements, expected behavior, or examples."**

Wait for the user's detailed description.

## Step 4: Exploration & Planning

Based on the scope and type:

**If Small:**

- Read relevant existing code
- Propose implementation directly

**If Medium:**

- Use Task tool with Explore agent to find similar patterns
- Use EnterPlanMode to create implementation plan
- Present plan for approval

**If Large:**

- Use Task tool with Explore agent to understand architecture
- Use EnterPlanMode to create detailed implementation plan
- Identify all files that need changes
- Present plan for approval

**If Unknown:**

- Use Explore agent to understand codebase
- Re-assess scope
- Then follow appropriate path above

## Step 5: Summary & Confirmation

Present a summary:

```
Branch: [branch-name]
Feature Type: [type]
Scope: [scope]
Visibility: [visibility]

Description: [user's description]

Proposed Approach:
- [bullet points of what you'll do]
```

Ask: **"Does this look correct? Should I proceed with [implementation/planning]?"**

## Important Reminders

- Always read existing code before proposing changes (CLAUDE.md)
- Follow patterns from similar commands in `cmd/`
- Add tests for new functionality (`*_test.go`)
- Check copyright headers in new files (`.copyright-header.tmpl`)
- Run `make lint` before completion
- All commits must be signed: `git commit -s`

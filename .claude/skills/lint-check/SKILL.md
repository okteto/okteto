---
name: lint-check
description: Run pre-commit hooks and golangci-lint to verify code quality before finishing work
---

# Lint Check

Run all quality checks (pre-commit hooks and golangci-lint) before considering work complete. Use this skill when finishing a task, before committing, or when preparing a PR.

## When to Use

- After completing a code change or task
- Before creating a commit
- Before pushing to remote
- When explicitly requested by the user
- When you've made changes to Go code or documentation

## Workflow

### Step 1: Check for Modified Files

First, check what files have been modified:

```bash
git status --short
```

If there are no modified files, inform the user and exit.

### Step 2: Run Pre-commit Hooks

Run pre-commit on all modified files:

```bash
pre-commit run --files <list-of-modified-files>
```

**Important**: If pre-commit modifies files (like prettier formatting), stage those changes:

```bash
git add <files-modified-by-pre-commit>
```

Then re-run pre-commit to verify all checks pass.

### Step 3: Run golangci-lint

If any Go files were modified, run the full lint suite:

```bash
make lint
```

This runs both pre-commit hooks and golangci-lint with the project's configuration.

### Step 4: Report Results

**If all checks pass:**

Present a success message:

```
✅ All quality checks passed!

Pre-commit: Passed
golangci-lint: 0 issues

Your changes are ready to commit.
```

**If checks fail:**

1. Show the specific errors/warnings
2. If the fixes are simple (formatting), apply them automatically
3. If the fixes require code changes, explain what needs to be fixed
4. Offer to help fix the issues

### Step 5: Suggest Next Steps

After successful checks, suggest appropriate next steps:

- If changes are staged: "Ready to commit. Would you like me to create a commit?"
- If not staged: "Would you like me to stage these changes?"
- If on a feature branch: "Would you like me to create a PR?"

## Error Handling

### Pre-commit Failures

- **Formatting issues** (prettier, trailing whitespace): Auto-fix by staging the changes pre-commit made
- **Linting issues** (markdownlint, yamllint): Show errors and offer to fix
- **Security issues** (gitleaks, detect-private-key): Alert immediately and DO NOT commit

### golangci-lint Failures

- **Code style issues**: Show the issues and offer to fix following the patterns in the codebase
- **Unused code**: Offer to remove unused functions/variables
- **Missing error handling**: Add proper error handling
- **Copyright headers**: Add missing headers using `.copyright-header.tmpl`

## Special Cases

### Large Output

If lint output is too large (>30KB), read the full output file and summarize:

- Total number of issues
- Breakdown by linter/severity
- Top 5 most common issues
- Actionable suggestions

### Tools Directory

If changes are in `tools/` directory, also run:

```bash
cd tools && make lint
```

### Documentation Only

If only documentation files (\*.md) were changed:

- Run pre-commit (includes markdownlint)
- Skip golangci-lint
- Report success faster

## Important Rules

- **Never skip checks**: Always run both pre-commit and golangci-lint for Go changes
- **Auto-stage formatting fixes**: If pre-commit reformats files, stage them automatically
- **Clear reporting**: Always show what passed/failed clearly
- **Actionable feedback**: If something fails, explain how to fix it
- **Context awareness**: If in tools/ directory, run tools-specific checks too

## Example Usage

**User says:** "I'm done with this feature"

1. Run git status to see changes
2. Run pre-commit on all modified files
3. Stage any formatting fixes from pre-commit
4. Run make lint for full quality check
5. Report results with ✅ or specific errors
6. Suggest next steps (commit, PR, etc.)

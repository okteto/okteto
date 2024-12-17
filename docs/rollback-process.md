# Rollback process

If post-release monitoring identifies critical issues, perform a rollback to revert to the previous stable state. Follow the steps below to execute a rollback safely and efficiently.

## When to Rollback
Consider initiating a rollback under the following circumstances:

- **Critical** Bugs: Presence of bugs that significantly impact functionality or user experience.
- **Performance Issues**: Degradation in application performance metrics.
- **Security Vulnerabilities**: Discovery of security flaws introduced in the release.
- **Negative User Feedback**: Substantial negative feedback indicating issues with the release.

## How to Rollback

The rollback process involves reverting Git tags across multiple repositories and updating the Docker image to a previous stable version. Use the provided `scripts/rollback-latest.sh` to automate this process.

### Example Rollback

Below is a step-by-step example of performing a rollback using the rollback_script.sh.

#### Scenario
- Current Tag to Move: 1.2.3
- Rollback to Tag: 1.2.2

#### Steps

1. Ensure Prerequisites are Met:
  - Verify access to Git repositories and Docker registry.
  - Confirm that `scripts/rollback_script.sh` is executable.

2. Execute the Rollback Script:
Open your terminal and navigate to the directory containing `scripts/rollback_script.sh`. Run the script with the appropriate arguments:

```bash
./rollback_script.sh 1.2.3 1.2.2
```

Options:

- `--dry-run`: Simulate the rollback without making changes.
- `--verbose`: Enable detailed output.

Example with Options:

```bash
./rollback_script.sh --dry-run --verbose 1.2.3 1.2.2
```

3. Monitor the Rollback Process:

- Logs: Check rollback.log for detailed logs of the rollback operations.
- Git Tags: Verify that Git tags have been moved to the rollback tag across all repositories.
- Docker Image: Ensure that the okteto/okteto:latest tag now points to 1.2.2.


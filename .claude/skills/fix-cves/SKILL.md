---
name: fix-cves
description: Fix all CVEs in the Okteto CLI Docker image by scanning with Trivy and updating vulnerable dependencies and binaries
---

# Fix CLI CVEs

Fix all vulnerabilities in the Okteto CLI Docker image using systematic vulnerability scanning and remediation.

## CONTEXT

- The source code and Dockerfile for this image are in this repository
- The Dockerfile includes multiple binaries: kubectl, helm, kustomize, git, syncthing, and the Okteto CLI itself
- Go dependencies for the Okteto CLI binary are in `go.mod` and `go.sum`
- Binary versions are defined as ARG variables at the top of the Dockerfile (lines 3-18)
- Focus on CRITICAL and HIGH severity vulnerabilities first, then address medium/low as needed

## WORKFLOW

### 1. Build and Scan Process

- Build the image: `docker build -t okteto-cli:test .` (do not use `--no-cache` unless necessary)
- Scan for vulnerabilities: `trivy image --severity CRITICAL,HIGH okteto-cli:test`
- For full scan: `trivy image okteto-cli:test`
- Analyze scan results to identify specific packages and CVEs that need attention

### 2. Generate CVE Summary Table

Before attempting any fixes, create a summary table showing the current state:

```markdown
## CVE Summary

| Binario                 | CVEs HIGH | CVEs CRITICAL | Estado                 |
| ----------------------- | --------- | ------------- | ---------------------- |
| syncthing               | X         | Y             | ✅/⚠️/❌ [Description] |
| kustomize               | X         | Y             | ✅/⚠️/❌ [Description] |
| kubectl                 | X         | Y             | ✅/⚠️/❌ [Description] |
| helm/helm3/helm4        | X         | Y             | ✅/⚠️/❌ [Description] |
| okteto                  | X         | Y             | ✅/⚠️/❌ [Description] |
| git                     | X         | Y             | ✅/⚠️/❌ [Description] |
| clean/remote/supervisor | X         | Y             | ✅/⚠️/❌ [Description] |

Total: X CRITICAL, Y HIGH
```

Status indicators:

- ✅ Clean - No vulnerabilities
- ⚠️ Warning - False positive or minor issue
- ❌ Action needed - Real CVEs requiring fixes

Then list each CVE found with details:

- CVE ID
- Component and version
- Vulnerability description
- Fixed version available
- Impact/severity

### 3. Vulnerability Remediation

- Repeat the build and scan process after each set of changes
- Continue until all CRITICAL and HIGH vulnerabilities are resolved or you cannot fix more CVEs

**Version update policy**:

- **Patch updates** (v1.2.3 → v1.2.4): Apply automatically
- **Minor updates** (v1.2.3 → v1.3.0): Ask user for confirmation before updating
- **Major updates** (v1.x → v2.x): Ask user for confirmation before updating

**For Dockerfile binaries** (kubectl, helm, kustomize, git, etc.):

- Check release pages for available versions:
  - kubectl: https://github.com/kubernetes/kubernetes/releases
  - helm: https://github.com/helm/helm/releases
  - kustomize: https://github.com/kubernetes-sigs/kustomize/releases
  - git: https://mirrors.edge.kernel.org/pub/software/scm/git/
- Update version ARG at top of Dockerfile (e.g., `ARG KUBECTL_VERSION=1.34.5`)
- Follow version update policy above

**For Go dependencies** (okteto binary):

- Update to specific patched version: `go get -u <module>@<patch-version>`
- Clean up: `go mod tidy`
- Follow version update policy above

### 4. Functionality Verification

Once vulnerabilities are fixed, verify the CLI still works correctly:

- Run unit tests: `make test`
- Run linting: `make lint`
- Test the Docker image: `docker run okteto-cli:test version`

## IMPORTANT CONSTRAINTS

- **kubectl maintenance**: Keep `kubectl` updated to latest patch of max minor Kubernetes version supported by Okteto
- **kubectl preservation**: Never remove `kubectl` from the image - it's required for functionality
- **Breaking changes**: Some updates may break compatibility - test thoroughly
- **Binary compatibility**: Ensure all binaries work on the Alpine/BusyBox base image

## PULL REQUEST REQUIREMENTS

If you did any change, create a PR for vulnerability fixes:

### Required Labels

- `area/security`

### Required Content

- **Clear status statement**: First line must clearly state whether ALL CRITICAL/HIGH vulnerabilities have been fixed or not
- **Before/after scans**: Include trivy scan results before and after changes using:

  ```
  trivy image --severity CRITICAL,HIGH okteto-cli:test
  ```

- **Summary of changes**: List specific updates (e.g., "kubectl 1.34.3 → 1.35.0", "github.com/foo/bar v1.2.3 → v1.2.4")

### Commit Message Format

```
fix: update vulnerable dependencies

- Update kubectl from 1.34.3 to 1.35.0 (fixes CVE-2024-XXXXX)
- Update github.com/example/module from v1.2.3 to v1.2.4 (fixes CVE-2024-YYYYY)

Resolves X CRITICAL and Y HIGH severity vulnerabilities.

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>
```

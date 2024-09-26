#!/usr/bin/env bash

# release-github-actions.sh - Automates the release of GitHub Actions to match a stable CLI release.
# It modifies the Dockerfile, updates tags and branches as per the specified release version.

set -euo pipefail

# Function to display usage information
usage() {
    echo "Usage: $0 [--dry-run] <RELEASE_TAG>"
    exit 1
}

# Check for required commands
required_commands=("semver" "gsutil" "git")
for cmd in "${required_commands[@]}"; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "Error: Required command '$cmd' not found."
        exit 1
    fi
done

# Check if GITHUB_TOKEN is set
if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "Error: GITHUB_TOKEN environment variable is not set."
    exit 1
fi

# Parse arguments
DRY_RUN=false
if [[ "${1:-}" == "--dry-run" ]]; then
    DRY_RUN=true
    shift
fi

RELEASE_TAG="${1:-}"

if [[ -z "$RELEASE_TAG" ]]; then
    usage
fi

REPO_OWNER="${REPO_OWNER:-okteto}"

echo "========================================"
echo "Running with dry-run: ${DRY_RUN}"
echo "Releasing GitHub Actions for version: ${RELEASE_TAG}"
echo "========================================"

# Validate RELEASE_TAG
if ! semver validate "$RELEASE_TAG" >/dev/null 2>&1; then
    echo "Error: '$RELEASE_TAG' is not a valid semantic version."
    exit 1
fi

PREREL=$(semver get prerel "$RELEASE_TAG")
MAJOR_VERSION=$(semver get major "$RELEASE_TAG")
MINOR_VERSION=$(semver get minor "$RELEASE_TAG")
MAJOR_TAG="v${MAJOR_VERSION}"
MINOR_TAG="v${MAJOR_VERSION}.${MINOR_VERSION}"
RELEASE_BRANCH="release-${MAJOR_VERSION}.${MINOR_VERSION}"

echo "Pre-release: $PREREL"
echo "Major version: $MAJOR_VERSION"
echo "Minor version: $MINOR_VERSION"
echo "Major tag: $MAJOR_TAG"
echo "Minor tag: $MINOR_TAG"
echo "Release branch: $RELEASE_BRANCH"
echo "========================================"

# Ensure it's a stable release
if [[ -n "$PREREL" ]]; then
    echo "Error: '$RELEASE_TAG' is not a stable release."
    exit 1
fi

VERSIONS_BUCKET_FILENAME="downloads.okteto.com/cli/stable/versions"
VERSIONS_FILE=$(mktemp)
trap 'rm -f "$VERSIONS_FILE"' EXIT

# Fetch versions from GCS
if ! gsutil cat "gs://${VERSIONS_BUCKET_FILENAME}" >"$VERSIONS_FILE"; then
    echo "Error: Failed to fetch versions from gs://${VERSIONS_BUCKET_FILENAME}"
    exit 1
fi

echo "========================================"
echo "Current stable versions (latest 10):"
tail -n 10 "$VERSIONS_FILE"
echo "========================================"

if ! grep -qFx "$RELEASE_TAG" "$VERSIONS_FILE"; then
    echo "Error: RELEASE_TAG '$RELEASE_TAG' not found in versions list."
    exit 1
fi

repos=(
    delete-namespace
    build
    destroy-preview
    deploy-preview
    namespace
    pipeline
    create-namespace
    destroy-pipeline
    apply
    context
    test
)

for repo in "${repos[@]}"; do
    echo "----------------------------------------"
    echo "Processing repository: ${REPO_OWNER}/${repo}"
    REPO_DIR=$(mktemp -d)
    # Ensure cleanup of the temporary directory
    trap 'rm -rf "$REPO_DIR"' EXIT

    git clone "git@github.com:${REPO_OWNER}/${repo}.git" "$REPO_DIR"
    pushd "$REPO_DIR" >/dev/null

    git config user.name "okteto"
    git config user.email "ci@okteto.com"

    git checkout main

    # Get the current highest tag in main branch
    current_tag=$(git tag --list --sort=-v:refname | grep -E "^[0-9]+\.[0-9]+\.[0-9]+$" | head -n1 || echo "0.0.0")

    echo "Current tag in repository '$repo': $current_tag"

    semver_diff=$(semver compare "${RELEASE_TAG}" "${current_tag}")
    if [ "${semver_diff}" -le 0 ]; then
        echo "RELEASE_TAG '$RELEASE_TAG' is not newer than current tag '$current_tag'. Skipping repository."
        popd >/dev/null
        rm -rf "$REPO_DIR"
        continue
    fi

    # Modify Dockerfile
    if [[ -f Dockerfile ]]; then
        echo "Updating Dockerfile to use okteto/okteto:${RELEASE_TAG}"
        sed -i.bak -E 's|(FROM okteto/okteto:)[^ ]*|\1'"${RELEASE_TAG}"'|g' Dockerfile
        rm Dockerfile.bak
        
        echo "Dockerfile changes:"
        git --no-pager diff Dockerfile

        git add Dockerfile 
        git commit -m "Release ${RELEASE_TAG}" || echo "No changes to commit in repository '$repo'."
    else
        echo "Warning: Dockerfile not found in repository '$repo'. Skipping Dockerfile update."
    fi

    # Create an annotated tag for RELEASE_TAG
    echo "Creating tag ${RELEASE_TAG}"
    if [ "$DRY_RUN" = false ]; then
        git tag -a "${RELEASE_TAG}" -m "Release ${RELEASE_TAG}"
    else
        echo "Dry run: Skipping tag creation for ${RELEASE_TAG}"
    fi

    # Function to update tags if RELEASE_TAG is greater
    update_tag_if_greater() {
        local TAG_NAME=$1
        if git rev-parse "${TAG_NAME}" >/dev/null 2>&1; then
            current_tag_version=$(git describe --tags "${TAG_NAME}" --abbrev=0 --match "*.*.*" 2>/dev/null || echo "0.0.0")
            diff=$(semver compare "${RELEASE_TAG}" "${current_tag_version}")
            if [ "${diff}" -gt 0 ]; then
                echo "Moving tag ${TAG_NAME} to new release ${RELEASE_TAG}"
                if [ "$DRY_RUN" = false ]; then
                    git tag -f "${TAG_NAME}" "${RELEASE_TAG}"
                else
                    echo "Dry run: Skipping moving tag ${TAG_NAME}"
                fi
            else
                echo "Existing ${TAG_NAME} tag points to an equal or newer version. Skipping."
            fi
        else
            echo "Creating tag ${TAG_NAME} for release ${RELEASE_TAG}"
            if [ "$DRY_RUN" = false ]; then
                git tag "${TAG_NAME}" "${RELEASE_TAG}"
            else
                echo "Dry run: Skipping creating tag ${TAG_NAME}"
            fi
        fi
    }

    # Update MAJOR_TAG
    update_tag_if_greater "${MAJOR_TAG}"

    # Update MINOR_TAG
    update_tag_if_greater "${MINOR_TAG}"

    # Create release branch
    if git rev-parse --verify --quiet "${RELEASE_BRANCH}"; then
        echo "Release branch ${RELEASE_BRANCH} already exists. Skipping branch creation."
    else
        echo "Creating release branch ${RELEASE_BRANCH}"
        if [ "$DRY_RUN" = false ]; then
            git branch "${RELEASE_BRANCH}" "${RELEASE_TAG}"
        else
            echo "Dry run: Skipping creation of release branch ${RELEASE_BRANCH}"
        fi
    fi

    # Update 'latest' and 'stable' tags if RELEASE_TAG is greater
    update_tag_if_greater "latest"
    update_tag_if_greater "stable"

    # Push changes
    if [ "$DRY_RUN" = false ]; then
        echo "Pushing commits, branches, and tags to remote for repository '$repo'"
        git push "git@github.com:${REPO_OWNER}/${repo}.git" main
        git push "git@github.com:${REPO_OWNER}/${repo}.git" "${RELEASE_BRANCH}"
        git push --tags --force
    else
        echo "Dry run: Skipping pushing commits, branches, and tags"
    fi

    popd >/dev/null
    rm -rf "$REPO_DIR"
    echo "Finished processing repository: ${REPO_OWNER}/${repo}"
    echo "----------------------------------------"
done

echo "========================================"
echo "Release process completed."
echo "========================================"

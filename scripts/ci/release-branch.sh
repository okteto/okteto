#!/usr/bin/env bash

# release-branch.sh takes care of releasing an LTS branch when new commits are
# pushed onto it. It does so by resolving the next git tag that must be created
# and pushing it to github.
#
# Release branches are created for each MAJOR.MINOR release. If the initial
# patch version of the release branch does not exist (2.4.0 for eg), a beta
# release is first created: 2.4.0-beta.1.
# All subsequent pushes to this branch will build a new beta release:
# 2.4.0-beta.2, 2.4.0-beta.3 etc.
# To release the first stable version, the 2.4.0 tag must be manually created
# and pushed from the tip of the release branch after acceptance criteria has
# been met (decided internally by the team). There is no automation in place
# for this at the moment.

{( # run in a subshell

set -e          # make any error fail the script
set -u          # make unbound variables fail the script
set -o pipefail # make any pipe error fail the script


# CURRENT_BRANCH is the branch being released.
# It is assumed: release-MAJOR.MINOR here. For eg: release-2.4
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# BASE_TAG is the MAJOR.MINOR portion of the current release, extracted from the
# current branch
BASE_TAG=$(echo $CURRENT_BRANCH | cut -d- -f2)

# stable_regex is a semver version regex for the stable channel
stable_regex='^v[0-9]+\.[0-9]+\.[0-9]+$'

# beta_regex is a semver version regex for the beta channel
beta_regex='^v[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$'

# ROOT_MINOR_TAG is the oldest relevent tag we should be able to reach from this
# release branch
ROOT_MINOR_TAG="${BASE_TAG}.0"

# PREVIOUS_TAGS are the tags to which the previous commit points to
PREVIOUS_TAGS="$(git tag --points-at HEAD^)"

# NEXT_TAG will be set below
NEXT_TAG=""

# logs the last 20 commits reachable from here for debugging purposes
git --no-pager log --pretty=oneline -n 20 --reverse --abbrev-commit

echo "current release branch: $CURRENT_BRANCH"
echo "base tag: $BASE_TAG"

# If the latest tag we can reach belongs to a previous minor/major
# version, it means that this is the first push to the branch so we create the
# first beta
ancestor=$(git describe --tags --abbrev=0)
if [ "$(semver compare $ROOT_MINOR_TAG $(semver get release $ancestor))" -eq "1" ]; then
  NEXT_TAG="${ROOT_MINOR_TAG}-beta.1"
  echo "Latest reacheable tag from release branch ${CURRENT_BRANCH} is from a previous release (${ancestor})"
  echo "Initializing beta for ${BASE_TAG} as ${NEXT_TAG}"

# All previous commits must be tagged after the first has been created
elif [ -z "${PREVIOUS_TAGS}" ]; then 
  prev_commit=$(git rev-parse --short HEAD^)
  echo "No tags point to the previous commit ($prev_commit)"
  echo "All commits from a release branch must be tagged"
  echo "Inspect the branch history and re-tag $prev_commit with the corresponding beta or stable tag"
  exit 1

# Our immediate ancestor can point to a beta AND a stable release at the same
# time so git describe won't work.
# This will happen when creating a patch release from an LTS branch and it's
# usually how promotion works.
elif echo "${PREVIOUS_TAGS}" | grep -Eq $stable_regex; then
  prev=$(echo "${PREVIOUS_TAGS}" | grep -E $stable_regex)
  NEXT_TAG="$(semver bump patch ${prev})-beta.1"
  echo "Previous tag is a stable release (${prev})"
  echo "Creating the new beta for the patch: ${NEXT_TAG}"

# If the previous release is not a stable release, simply bump the beta prerelease
elif echo "${PREVIOUS_TAGS}" | grep -Eq $beta_regex; then
  prev=$(echo "${PREVIOUS_TAGS}" | grep -E $beta_regex)
  NEXT_TAG=$(semver bump prerel ${prev})
  echo "Latest reacheable tag from release branch ${CURRENT_BRANCH} is a prerelease (${prev})"
  echo "Bumping prerel to ${NEXT_TAG}"

# this should never happen
else
  echo "Unknown error. Neither condition was met"
  printf "PREVIOUS_TAGS:\n${PREVIOUS_TAGS}\n"
  echo "ROOT_MINOR_TAG: ${ROOT_MINOR_TAG}"
  echo "BASE_TAG: ${BASE_TAG}"
  exit 1
fi

echo "Creating and pushing tag ${NEXT_TAG} to remote repository"
git config user.name "okteto"
git config user.email "ci@okteto.com"
git tag "${NEXT_TAG}" -a -m "Okteto CLI ${NEXT_TAG}"
git push origin "${NEXT_TAG}"
)}

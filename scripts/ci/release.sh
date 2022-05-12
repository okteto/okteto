#!/usr/bin/env bash

# release.sh is the release script to make Okteto CLI versions publicly
# available.
#
# This scripts is meant to be executed by circleci and makes a few assumptions about
# the environment it runs on. It assumes the golangci executor which has a few
# binaries required by this script.
# TODO: parameterize this script to make it able to run locally
#
# Releases can be pulled from three distinct channels: stable, beta and dev.
#
# Releases are represented by git annotated tags in semver version and have their
# respective Github release.
# Releases from the stable channel have the format: MAJOR.MINOR.PATCH
# Releases from the beta channel have the MAJOR.MINOR.PATCH-beta.n
# Releases from the dev channel have the MAJOR.MINOR.PATCH-dev.n
#

{(# run in a subshell

set -e          # make any error fail the script
set -u          # make unbound variables fail the script
set -o pipefail # make any pipe error fail the script

# RELEASE_TAG is the release tag that we want to release
RELEASE_TAG="${CIRCLE_TAG}"

# RELEASE_COMMIT is the commit being released
RELEASE_COMMIT=${CIRCLE_SHA1}

# REPO_OWNER is the owner of the repo (okteto)
REPO_OWNER="${CIRCLE_PROJECT_USERNAME}"

# REPO_NAME is the name of the repo (okteto)
REPO_NAME="${CIRCLE_PROJECT_REPONAME}"

# BIN_PATH is where the artifacts are stored. Usually precreated by the circleci
# workflow.
BIN_PATH="$PWD/artifacts/bin"

################################################################################
# Sanity check
################################################################################

if ! semver --version > /dev/null 2>&1; then 
  echo "Binary \"semver\" does is required from running this scripts"
  echo "Please install it and try again:"
  echo "  $ curl -o /usr/local/bin/semver https://raw.githubusercontent.com/fsaintjacques/semver-tool/master/src/semver"
  exit 1
fi

if ! which okteto-ci-utils > /dev/null 2>&1; then
  echo "Binary \"okteto-ci-utils\" from the golangci circleci executor is required to run this script"
  echo "If you are running this locally you can go into the golang-ci executor repo and build the script from source:"
  echo "  $ go build -o /usr/local/bin/okteto-ci-utils ."
  exit 1
fi

if ! which gsutil > /dev/null 2>&1; then
  echo "Binary \"gsutils\" from Google Cloud is required to run this script. Find installation instructions at https://cloud.google.com/sdk/docs/install"
  exit 1
fi

if ! which ghr > /dev/null 2>&1; then
  echo "Binary \"ghr\" is required to run this script. Install with:"
  echo "  $ GOPROXY=direct GOSUMDB=off go install github.com/tcnksm/ghr@latest"
  exit 1
fi

if [ ! -d $BIN_PATH ]; then
  echo "Release artifacts should be stored in $BIN_PATH but the directory does no exist"
  exit 1
elif [ -z "$(ls -A $BIN_PATH)" ]; then
  echo "Release artifacts should be stored in $BIN_PATH but the directory is empty"
  exit 1
fi

if [ -z $GITHUB_TOKEN ]; then
  echo "GITHUB_TOKEN envvar not provided. It is required to create the Github Release and the release notes"
  exit 1
fi

echo "Releasing tag: ${RELEASE_TAG}"

################################################################################
# Resolve release channel
################################################################################

# Resolve the channel from the tag that is being released
# If the channel is unknown the release will fail
beta_regex="^beta\.[0-9]+"
dev_regex="^dev\.[0-9]+"
prerel="$(semver get prerel ${RELEASE_TAG})"
CHANNELS=

# Stable releases are added to all channel
if [ -z $prerel ]; then
  CHANNELS=( "stable" "beta" "dev" )

elif [[ $prerel =~ $beta_regex ]]; then
  CHANNELS=( "beta" )

# TODO: Add beta releases to the dev channel.
# There's no deterministic way to sort beta and dev versions so dev versions
# will alway appear after betas. For the dev channel we may need to use the
# build portion of semver
elif [[ $prerel =~ $dev_regex ]]; then
  CHANNELS=( "dev" )
else
  echo "Unknown prerelease: $prerel"
  echo "Prerelease should be either blank (meaning stable channel) or in the format: beta.NUM or dev.NUM"
  echo "For eg: 1.2.3, 1.2.3-beta.1, 1.2.3-dev.3, etc"
  exit 1
fi

for chan in ${CHANNELS[@]}; do
  echo "---------------------------------------------------------------------------------"
  echo "Releasing ${RELEASE_TAG} into ${chan} channel"

  ##############################################################################
  # Update downloads.okteto.com
  ##############################################################################

  # BIN_BUCKET_NAME is the name of the bucket where the binaries are stored.
  # Starting at Okteto CLI 2.0, all these binaries are publicly accesible at:
  # https://downloads.okteto.com/cli/<channel>/<tag>
  BIN_BUCKET_NAME="downloads.okteto.com/cli/${chan}/${RELEASE_TAG}"

  # VERSIONS_BUCKET_NAME are all the available versions for a release channel.
  # This is also publicly accesible at:
  # https://downloads.okteto.com/cli/<channel>/versions
  VERSIONS_BUCKET_NAME="downloads.okteto.com/cli/${chan}/versions"

  # upload artifacts
  echo "Syncing artifacts from $BIN_PATH with $BIN_BUCKET_NAME"
  gsutil -m rsync -r $BIN_PATH gs://$BIN_BUCKET_NAME

  # Get the current versions file and add the current version being released.
  # These are the versions publicly accesible from this channel.
  # It is important to have them sorted so that the last version from the list
  # is always the latest and we can keep pushing older tags for maintenance and
  # whatnot.
  version_temp_file=$(mktemp)
  gsutil cat "gs://${VERSIONS_BUCKET_NAME}" > $version_temp_file
  echo "Current version list for ${chan} channel (showing latest 10):"
  tail $version_temp_file -n 10

  printf "${RELEASE_TAG}\n" >> $version_temp_file
  cat $version_temp_file | okteto-ci-utils semver-sort > "${BIN_PATH}/versions"
  echo "Added ${RELEASE_TAG} to the version list"
  echo "New version list for ${chan} channel (showing latest 10):"
  tail "${BIN_PATH}/versions" -n 10

  gsutil -m -h "Cache-Control: no-store" -h "Content-Type: text/plain" cp "${BIN_PATH}/versions" gs://${VERSIONS_BUCKET_NAME}
  echo "${chan} channel updated with ${RELEASE_TAG}"
done

################################################################################
# Update Github Release
################################################################################
previous_version=$(grep -F $RELEASE_TAG -B 1 "${BIN_PATH}/versions" | head -n1)
preferred_channel=$(echo $CHANNELS)

echo "Gathering ${RELEASE_TAG} release notes. Diffing from ${previous_version}"
notes=$(curl \
  -fsS \
  -X POST \
  -H "Authorization: Bearer ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  -d "{\"tag_name\":\"$RELEASE_TAG\",\"previous_tag_name\":\"$previous_version\"}" \
  https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/generate-notes | jq .body)

echo "RELEASE NOTES: "
printf "${notes}"

prerelease=false
name="Okteto CLI - ${RELEASE_TAG}"
if [ "${preferred_channel}" = "beta" ] || [ "${preferred_channel}" = "dev" ]; then
  prerelease=true
  name="Okteto CLI [${preferred_channel}] - ${RELEASE_TAG}"
fi

echo "Using ghr version: $(ghr -version)"
ghr \
  -u ${REPO_OWNER} \
  -n "${name}" \
  -r ${REPO_NAME} \
  -c ${RELEASE_COMMIT} \
  -token ${GITHUB_TOKEN} \
  -b ${notes} \
  -replace \
  -prerelease=${prerelease} \
  ${RELEASE_TAG} \
  ${BIN_PATH}

echo "Created Github release: '${name}'"

echo "All done"
)}

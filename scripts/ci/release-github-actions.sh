#!/usr/bin/env bash

# release-github-actions.sh takes care of create a github action release that
# matches a stable CLI release.
# It does so by creating a git tag in each of the action repos and updating the
# tip of LTS branches if necessary.
# Github Actions can be consumed through LTS branches or "latest". These
# branches are created for major releases and kept up to date with the latest
# changes.

# run in a subshell
{ (
        set -e # make any error fail the script
        set -u # make unbound variables fail the script

        # SC2039: In POSIX sh, set option pipefail is undefined
        # shellcheck disable=SC2039
        set -o pipefail # make any pipe error fail the script

        # RELEASE_TAG is the release tag that we want to release
        RELEASE_TAG="${1:-""}"

        # REPO_OWNER is the owner of the repo (okteto)
        REPO_OWNER="okteto"

        if [ "$RELEASE_TAG" = "" ]; then
                echo "RELEASE_TAG not provided"
                exit 1
        fi

        if ! semver validate "${RELEASE_TAG}" >/dev/null 2>&1; then
                echo "${RELEASE_TAG} is not a valid semver format"
                exit 1
        fi

        PREREL="$(semver get prerel "${RELEASE_TAG}")"
        MAJOR_VERSION="$(semver get major "${RELEASE_TAG}")"
        LTS_BRANCH="v${MAJOR_VERSION}"

        # stable release
        if [ -n "$PREREL" ]; then
                echo "Current version ${RELEASE_TAG} is not a stable semver release"
                exit 1
        fi

        VERSIONS_BUCKET_FILENAME="downloads.okteto.com/cli/stable/versions"

        VERSIONS_FILE=$(mktemp)
        gsutil cat "gs://${VERSIONS_BUCKET_FILENAME}" >"$VERSIONS_FILE"
        echo "Current version list for stable channel (showing latest 10):"
        tail -n 10 "$VERSIONS_FILE"

        if ! grep -qFx "${RELEASE_TAG}" "${VERSIONS_FILE}"; then
                echo "RELEASE_TAG: ${RELEASE_TAG} not included in the versions from https://${VERSIONS_BUCKET_FILENAME}"
                exit 1
        fi

        repos=(
                delete-namespace
                build
                destroy-preview
                deploy-preview
                deploy-stack
                namespace
                pipeline
                push
                create-namespace
                destroy-pipeline
                login
                destroy-stack
                apply
                context
                test
        )

        for repo in "${repos[@]}"; do
                echo "Releasing ${REPO_OWNER}/$repo"
                git clone "git@github.com:${REPO_OWNER}/${repo}.git"
                pushd "$repo"
                git config user.name "okteto"
                git config user.email "ci@okteto.com"

                if git tag --list | grep -qFx "$RELEASE_TAG"; then
                        echo "$RELEASE_TAG already exists"
                        exit 1
                fi

                # checkout the list branch. Create it if it doesn't exist
                git checkout "${LTS_BRANCH}" 2>/dev/null || git checkout -b "${LTS_BRANCH}"

                # get the latest tag from the lts branch
                current_tag="$(git describe --tags --abbrev=0 --match "*.*.*" || echo "0.0.0")"
                diff="$(semver compare "${current_tag}" "$RELEASE_TAG")"

                # RELEASE_TAG must be a newer release
                if [ "${diff}" != "-1" ]; then
                        echo "RELEASE_TAG ${RELEASE_TAG} is older than the current tag: ${current_tag}"
                        exit 0
                fi

                # NOTE: these sed commands will not work in OS X and will require `brew install gnu-sed` and updating the PATH
                sed -i -E 's_FROM\ okteto\/okteto\:latest_FROM\ okteto\/okteto\:'"$RELEASE_TAG"'_' Dockerfile
                sed -i -E 's_FROM\ okteto\/okteto\:[[:digit:]]*\.[[:digit:]]*\.[[:digit:]]*_FROM\ okteto\/okteto\:'"$RELEASE_TAG"'_' Dockerfile
                git add Dockerfile
                git commit -m "release ${RELEASE_TAG}"

                echo "Pushing LTS branch ${LTS_BRANCH}"
                git push "git@github.com:${REPO_OWNER}/${repo}.git" "${LTS_BRANCH}"

                echo "Creating release for tag ${RELEASE_TAG}"
                ghr \
                        -debug \
                        -name "${RELEASE_TAG}" \
                        -token "$GITHUB_TOKEN" \
                        -recreate \
                        -replace \
                        -commitish "$(git rev-parse "${LTS_BRANCH}")" \
                        "$RELEASE_TAG"

                # After updating the LTS branch, if the current release is the latest known
                # release, update latest
                latest="$(tail -n1 "${VERSIONS_FILE}")"
                if [ "${latest}" = "${RELEASE_TAG}" ]; then
                        echo "Updating latest tag"
                        ghr \
                                -debug \
                                -name "latest@${RELEASE_TAG}" \
                                -token "$GITHUB_TOKEN" \
                                -recreate \
                                -replace \
                                -commitish "$(git rev-parse "${LTS_BRANCH}")" \
                                latest
                fi

                popd
                rm -rf "$repo"
        done

); }

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

# run in a subshell
{ (

        set -e # make any error fail the script
        set -u # make unbound variables fail the script

        # SC2039: In POSIX sh, set option pipefail is undefined
        # shellcheck disable=SC2039
        set -o pipefail # make any pipe error fail the script

        # CURRENT_BRANCH is the branch being released.
        # It is assumed: release-MAJOR.MINOR here. For eg: release-2.4
        CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

        # BASE_TAG is the MAJOR.MINOR portion of the current release, extracted from the
        # current branch
        BASE_TAG=$(echo "$CURRENT_BRANCH" | cut -d- -f2)

        # stable_regex is a semver version regex for the stable channel
        stable_regex='^[0-9]+\.[0-9]+\.[0-9]+$'

        # beta_regex is a semver version regex for the beta channel
        beta_regex='^[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$'

        # PREVIOUS_TAGS are the tags to which the previous known ancestor
        # The known ancestor is that last commit a release points to. It can point to a
        # beta AND a stable release at the same time so git describe won't work.
        # This will happen when creating a patch release from an LTS branch and it's
        # usually how promotion works.
        known_ancestor="$(git rev-list -n 1 "$(git describe --tags --abbrev=0 --match "*.*.*")")"
        PREVIOUS_TAGS="$(git show-ref --tags -d | grep "^${known_ancestor}" | sed -e 's,.* refs/tags/,,' -e 's/\^{}//')"

        # ROOT_MINOR_TAG is the oldest relevant tag we should be able to reach from this
        # release branch
        ROOT_MINOR_TAG="${BASE_TAG}.0"

        # NEXT_TAG will be set below
        NEXT_TAG=""

        # logs the last 20 commits reachable from here for debugging purposes
        git --no-pager log --pretty=oneline -n 20 --reverse --abbrev-commit

        echo "current release branch: $CURRENT_BRANCH"
        echo "base tag: $BASE_TAG"
        printf "PREVIOUS_TAGS:\n%s\n" "${PREVIOUS_TAGS}"

        # All previous commits must be tagged after the first has been created
        if [ -z "${PREVIOUS_TAGS}" ]; then
                echo "No tags point to the previous commit ($known_ancestor)"
                echo "All commits from a release branch must be tagged"
                echo "Inspect the branch history and re-tag $known_ancestor with the corresponding beta or stable tag"
                exit 1
        fi

        # Select the tag from the list all of the tags that point to the last release
        previous_stable=$(echo "${PREVIOUS_TAGS}" | grep -E "$stable_regex" || echo "")
        previous_beta=$(echo "${PREVIOUS_TAGS}" | grep -E "$beta_regex" || echo "")

        # If the latest tag we can reach belongs to a previous minor/major
        # version, it means that this is the first push to the branch so we create the
        # first beta
        if [ "$previous_stable" != "" ]; then
                echo "Building from previous stable: $previous_stable"

                # make sure we are building within the bounds of patch/prerelease
                diff="$(semver diff "${ROOT_MINOR_TAG}" "$previous_stable")"
                case "$diff" in
                # If the major or minor version differ, we must be ahead meaning
                # this is first tag
                major | minor)
                        if [ "$(semver compare "${ROOT_MINOR_TAG}" "$previous_stable")" -eq "-1" ]; then
                                echo "Cannot build tag. Latest reachable tag from release branch ${CURRENT_BRANCH} is bigger than ${ROOT_MINOR_TAG}"
                                exit 1
                        fi
                        NEXT_TAG="${ROOT_MINOR_TAG}-beta.1"
                        ;;

                # normal bound of the release branch
                prerelease | patch | "")
                        if [ "$(semver compare "${ROOT_MINOR_TAG}" "$previous_stable")" -eq "1" ]; then
                                NEXT_TAG="${ROOT_MINOR_TAG}-beta.1"
                                echo "Latest reachable tag from release branch ${CURRENT_BRANCH} is a stable release from a previous release (${previous_stable})"
                                echo "Initializing beta for ${BASE_TAG} as ${NEXT_TAG}"
                        else
                                NEXT_TAG="$(semver bump patch "${previous_stable}")-beta.1"
                                echo "Previous tag is a stable release (${previous_stable})"
                                echo "Creating the new beta for the patch: ${NEXT_TAG}"
                        fi
                        ;;
                *)
                        echo "Unknown diff: $diff"
                        exit 1
                        ;;
                esac

        elif [ "$previous_beta" != "" ]; then
                echo "Building from previous beta: $previous_beta"
                # make sure we are building within the bounds of patch/prerelease
                diff="$(semver diff "${ROOT_MINOR_TAG}" "$previous_beta")"
                case "$diff" in
                # If the major or minor version differ, we must be ahead meaning
                # this is first tag
                major | minor)
                        if [ "$(semver compare "${ROOT_MINOR_TAG}" "$previous_beta")" -eq "-1" ]; then
                                echo "Cannot build tag. Latest reachable tag from release branch ${CURRENT_BRANCH} is bigger than ${ROOT_MINOR_TAG}"
                                exit 1
                        fi
                        echo "Latest reachable tag from release branch ${CURRENT_BRANCH} is a prerelease from a previous MAJOR.MINOR (${previous_beta})"
                        NEXT_TAG="${ROOT_MINOR_TAG}-beta.1"
                        echo "Initializing ${NEXT_TAG}"
                        ;;

                # normal bound of the release branch
                prerelease | patch)
                        NEXT_TAG=$(semver bump prerel "${previous_beta}")
                        echo "Latest reachable tag from release branch ${CURRENT_BRANCH} is a prerelease (${previous_beta})"
                        echo "Bumping prerel to ${NEXT_TAG}"

                        ;;
                *)
                        echo "Unknown diff: $diff"
                        exit 1
                        ;;
                esac

        # this should never happen
        else
                echo "Unclear what to build. Skipping release"
                exit 1
        fi
        echo "Pushing tag ${NEXT_TAG} to remote repository"
        git config user.name "okteto"
        git config user.email "ci@okteto.com"
        git tag "${NEXT_TAG}" -a -m "Okteto CLI ${NEXT_TAG}"
        git push origin "${NEXT_TAG}"
); }

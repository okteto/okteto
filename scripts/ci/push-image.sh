#!/usr/bin/env bash

# push-image.sh pushes the docker image to the docker registry

# run in a subshell
{ (

        set -e # make any error fail the script
        set -u # make unbound variables fail the script

        # SC2039: In POSIX sh, set option pipefail is undefined
        # shellcheck disable=SC2039
        set -o pipefail # make any pipe error fail the script

        # RELEASE_TAG is the release tag that we want to release
        RELEASE_TAG="${1}"
        # PLATFORMS is the arch's we want to release
        PLATFORMS="${2}"

        echo "DEBUG: RELEASE_TAG: ${RELEASE_TAG}"
        echo "DEBUG: PLATFORMS: ${PLATFORMS}"

        IFS=',' read -ra TAGS_ARRAY <<< "$RELEASE_TAG"

        if [ -z "${TAGS_ARRAY[0]}" ]; then
                commit=$(git rev-parse --short HEAD)
                TAGS_ARRAY=("$commit" "${TAGS_ARRAY[@]:1}")
                echo "DEBUG: First element of RELEASE_TAG was empty. Using git commit: ${commit}"
        fi

        echo "DEBUG: Final RELEASE_TAG: ${TAGS_ARRAY[*]}"

        echo "DEBUG: Splitting RELEASE_TAG into TAGS_ARRAY:"
        for tag in "${TAGS_ARRAY[@]}"; do
                echo "  DEBUG: Tag: ${tag}"
        done

        # The first tag is the version for the build
        VERSION_STRING="${TAGS_ARRAY[0]}"

        tags_array=()
        beta_added=false
        stable_added=false
        for tag in "${TAGS_ARRAY[@]}"; do
                echo "DEBUG: Processing tag: ${tag}"
                prerel="$(semver get prerel "${tag}" || true)"
                version="$(semver get release "${tag}" || true)"
                echo "  DEBUG: prerel: ${prerel}"
                echo "  DEBUG: version: ${version}"

                tags_array+=("okteto/okteto:${tag}")
                echo "  DEBUG: Added tag to tags_array: okteto/okteto:${tag}"
                echo "  DEBUG: tags_array so far: ${tags_array[*]}"

                if [ -n "$prerel" ]; then
                        if [ "$beta_added" = false ]; then
                                tags_array+=("okteto/okteto:beta")
                                beta_added=true
                                echo "  DEBUG: Added beta tag to tags_array"
                        else
                                echo "  DEBUG: Beta tag already added, skipping"
                        fi
                elif [ -n "$version" ]; then
                        # It's a stable version because it has $version and it's not a prerrelease
                        if [ "$stable_added" = false ]; then
                                tags_array+=("okteto/okteto:stable" "okteto/okteto:latest")
                                stable_added=true
                                echo "  DEBUG: Added stable and latest tags to tags_array"
                        else
                                echo "  DEBUG: Stable and latest tags already added, skipping"
                        fi
                else
                        # We don't push to dev because it's done just at the nightlies
                        echo "  DEBUG: Tag ${tag} is a dev tag"
                fi
        done

        tags=""
        for tag in "${tags_array[@]}"; do
                tags+="-t $tag "
        done
        echo "DEBUG: Final tags string: ${tags}"

        echo "DEBUG: Executing command:"
        echo "depot build --push --platform \"${PLATFORMS}\" --build-arg VERSION_STRING=\"${VERSION_STRING}\" ${tags} -f Dockerfile ."
        depot build --push --platform "${PLATFORMS}" --build-arg VERSION_STRING="${VERSION_STRING}" ${tags} -f Dockerfile .
); }

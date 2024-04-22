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

        if [ -z "$RELEASE_TAG" ]; then
                commit=$(git rev-parse --short HEAD)
                RELEASE_TAG="$commit"
        fi


        beta_prerel_regex="^beta\.[0-9]+"
        prerel="$(semver get prerel "${RELEASE_TAG}" || true)"
        version="$(semver get release "${RELEASE_TAG}" || true)"
        tags="okteto/okteto:${RELEASE_TAG},okteto/okteto:dev"

        # if release tag is  not empty, push the stable image
        if [ -n "$RELEASE_TAG" ]; then
                echo "prerel: ${prerel}"
                echo "version ${version}"
                if [[ $prerel =~ $beta_prerel_regex ]]; then
                        tags="${tags},okteto/okteto:beta"
                elif [ -n "$version" ]; then
                        tags="${tags},okteto/okteto:stable" 
                fi
        fi

        echo "Pushing ${tags} to Docker Hub"
        okteto build --platform "${PLATFORMS}" --build-arg VERSION_STRING="${RELEASE_TAG}" -t "${tags}" -f Dockerfile .

); }

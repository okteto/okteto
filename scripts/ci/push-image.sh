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

        okteto build --platform ${PLATFORMS} --build-arg VERSION_STRING="${RELEASE_TAG}" -t "okteto/okteto:${RELEASE_TAG}" -f Dockerfile .

); }

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

        if [ -z "$RELEASE_TAG" ]; then
                branch=$(git rev-parse --abbrev-ref HEAD)
                commit=$(git rev-parse --short HEAD)
                if [ "$branch" = "master" ]; then
                        RELEASE_TAG="latest"
                elif [ "$branch" = "main" ]; then
                        RELEASE_TAG="main"
                else
                        RELEASE_TAG="$commit"
                fi
        fi

        name="okteto/okteto:${RELEASE_TAG}"

        echo "Pushing ${name}"
        export DOCKER_BUILDKIT=1
        echo "$DOCKER_PASS" | docker login --username "$DOCKER_USER" --password-stdin
        docker build -t "$name" --build-arg VERSION_STRING="${RELEASE_TAG}" .
        docker push "$name"

); }

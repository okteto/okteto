#!/bin/bash
set -e

VERSION=$1

if [ -z "$VERSION" ]; then
        echo "missing version"
        exit 1
fi

actionsRepos=(delete-namespace
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

for repo in "${actionsRepos[@]}"; do
        echo "$repo"
        rm -rf "$repo"
        git clone git@github.com:okteto/"$repo".git
        pushd "$repo"
        git checkout "$VERSION"
        ghr \
                -debug \
                -name "latest@${VERSION}" \
                -token "$GITHUB_TOKEN" \
                -recreate \
                -replace \
                -commitish "$(git rev-parse main)" \
                latest
        popd
        rm -rf "$repo"
done

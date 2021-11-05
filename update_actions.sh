#!/bin/bash
set -e

VERSION=$1

if [ -z "$VERSION" ]; then
        echo "missing version"
        exit 1
fi

actionsRepos=(delete-namespace/master
        build/master
        destroy-preview/master
        deploy-preview/master
        deploy-stack/master
        namespace/master
        pipeline/master
        push/master
        create-namespace/master
        destroy-pipeline/master
        login/master
        destroy-stack/master
        apply/master
        context/main)

for actionRepo in "${actionsRepos[@]}"; do
        repo=${actionRepo%/*}
        branch=${actionRepo#*/}
        echo "$repo will be published @ $branch"
        git clone --depth 1 git@github.com:okteto/"$repo".git
        pushd "$repo"
        git config user.name "okteto"
        git config user.email "ci@okteto.com"
        sed -iE 's_FROM\ okteto\/okteto\:latest_FROM\ okteto\/okteto\:'"$VERSION"'_' Dockerfile
        sed -iE 's_FROM\ okteto\/okteto\:[[:digit:]]*\.[[:digit:]]*\.[[:digit:]]*_FROM\ okteto\/okteto\:'"$VERSION"'_' Dockerfile
        git add Dockerfile
        ret=0
        git commit -m "release $VERSION" || ret=1
        if [ $ret -ne 1 ]; then
                git push git@github.com:okteto/"$repo".git "$branch"
                git --no-pager log -1
        fi
        ghr -token "$GITHUB_TOKEN" -replace "$VERSION"
        ghr -token "$GITHUB_TOKEN" -delete "latest"
        popd
        rm -rf "$repo"
done

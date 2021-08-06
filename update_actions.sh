
#!/bin/bash
set -e

VERSION=$1

if [ -z "$VERSION" ]; then
        echo "missing version"
        exit 1
fi

actionsRepos=(okteto/delete-namespace
              okteto/build
              okteto/destroy-preview
              okteto/deploy-preview
              okteto/deploy-stack
              okteto/namespace
              okteto/pipeline
              okteto/push
              okteto/create-namespace
              okteto/destroy-pipeline
              okteto/login
              okteto/destroy-stack
              okteto/apply)
              
for repo in "${actionsRepos[@]}"
do
    echo "$repo"
    git clone --depth 1 https://github.com/$repo.git
    repoPath=${repo#"okteto/"}
    echo $repoPath
    pushd $repoPath
    git config user.name "okteto"
    git config user.email "ci@okteto.com"
    sed -iE 's_FROM\ okteto\/okteto\:latest_FROM\ okteto\/okteto\:'$VERSION'_' Dockerfile
    sed -iE 's_FROM\ okteto\/okteto\:[[:digit:]]*\.[[:digit:]]*\.[[:digit:]]*_FROM\ okteto\/okteto\:'$VERSION'_' Dockerfile
    git add Dockerfile
    git commit -m "release $VERSION"
    git push git@github.com:$repo.git master
    git --no-pager log -1
    ghr -u ${CIRCLE_PROJECT_USERNAME} -token $GITHUB_TOKEN -replace $VERSION
    popd
    rm -rf $repoPath
done

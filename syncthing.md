# Syncthing

Install git-lfs:
```
brew install git-lfs
```

To update syncthing:
```
version=1.3.0
git remote add parent git@github.com:syncthing/syncthing.git
git fetch parent
git checkout v${version}
git checkout -b v${version}-gen1
git lfs install
go generate github.com/syncthing/syncthing/lib/auto github.com/syncthing/syncthing/cmd/strelaypoolsrv/auto
git lfs track lib/auto/gui.files.go
git add .gitattributes
git add -f lib/auto/gui.files.go
git commit -m "generate for ${version}"
git push --set-upstream origin v${version}-gen1
```

The update go.mod with the new version tag.
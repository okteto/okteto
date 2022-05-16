tag=$(git describe --tags --abbrev=0)
prerel=$(semver get prerel $tag)

echo "Current tag: $tag"
echo "Current prerelease: $prerel"

TAG=
if [[ "$(semver get prerel $tag)" =~ dev\.[0-9]+  ]]; then
	TAG=$(semver bump prerel $tag)
else
	TAG=$(semver bump prerel dev.1 $tag)
fi

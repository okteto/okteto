# !/bin/sh
set -e

VERSION=$1
SHA=$2
GITHUB_TOKEN=$3
DRYRUN=$4

if [ -z "$VERSION" ]; then
  echo "missing version"
  exit 1
fi

if [ -z "$SHA" ]; then
  echo "missing sha"
  exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
  echo "missing github token"
  exit 1
fi

pushd $(mktemp -d)

git clone --depth 1 https://${GITHUB_TOKEN}@github.com/okteto/homebrew-cli.git
pushd homebrew-cli

cat << EOF > Formula/cli.rb
class Okteto < Formula
    desc "CLI for cloud native development"
    homepage "https://okteto.com"
    url "https://downloads.okteto.com/cli/okteto-Darwin-x86_64"
    sha256 "$SHA"
    version "$VERSION"
    
    devel do
        url "https://downloads.okteto.com/cli/master/okteto-Darwin-x86_64"
    end
    
    def install
        bin.install "okteto-Darwin-x86_64"
        mv bin/"okteto-Darwin-x86_64", bin/"okteto"
    end

    # Homebrew requires tests.
    test do
        assert_match "okteto version $VERSION", shell_output("#{bin}/okteto version 2>&1", 0)
    end
end
EOF

cat Formula/okteto.rb
git add Formula/okteto.rb
git config user.name "okteto"
git config user.email "ci@okteto.com"
git commit -m "$VERSION release"
git --no-pager log -1

if [ "$DRYRUN" -eq "1" ]; then
  echo "dry run: git push origin master"
else
  git push https://${GITHUB_TOKEN}@github.com/okteto/homebrew-okteto.git master
fi


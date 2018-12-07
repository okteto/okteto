# !/bin/sh
set -e

VERSION=$1
SHA=$2
GITHUB_TOKEN=$3

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

git clone --depth 1 https://${GITHUB_TOKEN}@github.com/okteto/homebrew-cnd.git
pushd homebrew-cnd
cat << EOF > Formula/cnd.rb
class Cnd < Formula
    desc "CLI for cloud native development"
    homepage "https://github.com/okteto/cnd"
    version "$VERSION"
    url "https://github.com/okteto/cnd/releases/download/#{version}/cnd-darwin-amd64"
    sha256 "$SHA"
    
    depends_on "syncthing"

    def install
        bin.install "cnd-darwin-amd64"
        mv bin/"cnd-darwin-amd64", bin/"cnd"
    end

    # Homebrew requires tests.
    test do
        assert_match "cnd version #{version}", shell_output("#{bin}/cnd version 2>&1", 0)
    end
end
EOF

git add Formula/cnd.rb
git config --global user.name "okteto"
git config --global user.email "ci@okteto.com"
git commit -m "$VERSION release"
git --no-pager log -1
git push origin master

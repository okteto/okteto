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

git clone --depth 1 https://${GITHUB_TOKEN}@github.com/okteto/homebrew-cnd.git
pushd homebrew-cnd

cat << EOF > Formula/cnd.rb
class Cnd < Formula
    desc "CLI for cloud native development"
    homepage "https://github.com/cloudnativedevelopment/cnd"
    url "https://github.com/cloudnativedevelopment/cnd.git",
      :tag      => "$VERSION",
      :revision => "$SHA"
    head "https://github.com/cloudnativedevelopment/cnd.git"

    depends_on "syncthing"
    depends_on "go" => :build

    def install
      ENV["GOPATH"] = buildpath
      ENV["VERSION_STRING"] = "$VERSION"
      contents = Dir["{*,.git,.gitignore}"]
      (buildpath/"src/github.com/cloudnativedevelopment/cnd").install contents
      cd "src/github.com/cloudnativedevelopment/cnd" do
        system "make"
        bin.install "bin/cnd"
      end
    end

    # Homebrew requires tests.
    test do
        assert_match "cnd version #{version}", shell_output("#{bin}/cnd version 2>&1", 0)
    end
end
EOF

cat Formula/cnd.rb
git add Formula/cnd.rb
git config --global user.name "okteto"
git config --global user.email "ci@okteto.com"
git commit -m "$VERSION release"
git --no-pager log -1

if [ "$DRYRUN" -eq "1" ]; then
  echo "dry run: git push origin master"
else
  git push https://${GITHUB_TOKEN}@github.com/okteto/homebrew-cnd.git master
fi


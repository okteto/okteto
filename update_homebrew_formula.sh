# !/bin/sh
set -e

VERSION=$1
SHA=$2
SHA_ARM=$3

if [ -z "$VERSION" ]; then
  echo "missing version"
  exit 1
fi

if [ -z "$SHA" ]; then
  echo "missing sha"
  exit 1
fi

if [ -z "$SHA_ARM" ]; then
  echo "missing sha for ARM"
  exit 1
fi

rm -rf homebrew-cli
git clone --depth 1 https://github.com/okteto/homebrew-cli.git
pushd homebrew-cli

cat << EOF > Formula/okteto.rb
class Okteto < Formula
  desc "Develop and test your code directly in Kubernetes"
  homepage "https://github.com/okteto/okteto"
  version "$VERSION"
  license "Apache-2.0"
  
  if Hardware::CPU.arm?
    sha256 "$SHA_ARM"
    url "https://github.com/okteto/okteto/releases/download/$VERSION/okteto-Darwin-arm64"
  else
    sha256 "$SHA"
    url "https://github.com/okteto/okteto/releases/download/$VERSION/okteto-Darwin-x86_64"
  end

  head do
    if Hardware::CPU.arm?
      url "https://downloads.okteto.com/cli/master/okteto-Darwin-arm64"
    else
      url "https://downloads.okteto.com/cli/master/okteto-Darwin-x86_64"
    end
  end
  
  def install
    if Hardware::CPU.arm?
      bin.install "okteto-Darwin-arm64"
      mv bin/"okteto-Darwin-arm64", bin/"okteto"
    else
      bin.install "okteto-Darwin-x86_64"
      mv bin/"okteto-Darwin-x86_64", bin/"okteto"
    end
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


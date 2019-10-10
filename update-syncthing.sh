git pul# !/bin/sh
set -e
set -x

VERSION=$1
if [ -z "$VERSION" ]; then
  echo "missing version"
  exit 1
fi

tmp=$(mktemp -d)

wget https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-linux-amd64-v${VERSION}.tar.gz -O $tmp/syncthing-linux-amd64-v${VERSION}.tar.gz
tar -zxvf $tmp/syncthing-linux-amd64-v${VERSION}.tar.gz -C $tmp

wget https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-macos-amd64-v${VERSION}.tar.gz -O $tmp/syncthing-macos-amd64-v${VERSION}.tar.gz
tar -zxvf $tmp/syncthing-macos-amd64-v${VERSION}.tar.gz -C $tmp

wget https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-windows-amd64-v${VERSION}.zip -O $tmp/syncthing-windows-amd64-v${VERSION}.zip
unzip $tmp/syncthing-windows-amd64-v${VERSION}.zip -d $tmp

gsutil cp $tmp/syncthing-linux-amd64-v${VERSION}/syncthing gs://downloads.okteto.com/cli/syncthing/${VERSION}/syncthing-Linux-x86_64
gsutil cp $tmp/syncthing-macos-amd64-v${VERSION}/syncthing gs://downloads.okteto.com/cli/syncthing/${VERSION}/syncthing-Darwin-x86_64
gsutil cp $tmp/syncthing-windows-amd64-v${VERSION}/syncthing.exe gs://downloads.okteto.com/cli/syncthing/${VERSION}/syncthing-Windows-x86_64


#!/bin/sh
set -ex
VERSION=1.3.4

wget -O syncthing-linux-amd64.tar.gz https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-linux-amd64-v${VERSION}.tar.gz
tar -zxvf syncthing-linux-amd64.tar.gz
cp syncthing-linux-amd64-v${VERSION}/syncthing syncthing-Linux-x86_64
rm -rf syncthing-linux-amd64*

wget -O syncthing-linux-arm64.tar.gz https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-linux-arm64-v${VERSION}.tar.gz
tar -zxvf syncthing-linux-arm64.tar.gz
cp syncthing-linux-arm64-v${VERSION}/syncthing syncthing-Linux-arm64
rm -rf syncthing-linux-arm64*

wget -O syncthing-macos-amd64.tar.gz https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-macos-amd64-v${VERSION}.tar.gz
tar -zxvf syncthing-macos-amd64.tar.gz
cp syncthing-macos-amd64-v${VERSION}/syncthing syncthing-Darwin-x86_64
rm -rf syncthing-macos-amd64*

wget -O syncthing-windows-amd64.zip https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-windows-amd64-v${VERSION}.zip
unzip syncthing-windows-amd64.zip
cp syncthing-windows-amd64-v${VERSION}/syncthing.exe syncthing-Windows-x86_64
rm -rf syncthing-windows-amd64*


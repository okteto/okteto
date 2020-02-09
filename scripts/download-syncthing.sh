#!/bin/sh
set -ex
VERSION=1.3.4

downloadLinux() {
  prefix=$1
  output=$2
  wget -qO- https://github.com/syncthing/syncthing/releases/download/v${VERSION}/${prefix}-v${VERSION}.tar.gz | tar xvz ${prefix}-v${VERSION}/syncthing
  mv ${prefix}-v${VERSION}/syncthing $output
  rm -rf ${prefix}*
  tar -czvf ${output}.tar.gz ${output}
}

downloadWindows() {
  wget -O syncthing-windows-amd64.zip https://github.com/syncthing/syncthing/releases/download/v${VERSION}/syncthing-windows-amd64-v${VERSION}.zip
  unzip -j syncthing-windows-amd64.zip syncthing-windows-amd64-v${VERSION}/syncthing.exe -d .
  mv syncthing.exe syncthing-Windows-x86_64
  rm -rf syncthing-windows-amd64*
  tar -czvf syncthing-Windows-x86_64.tar.gz syncthing-Windows-x86_64
}

downloadLinux syncthing-linux-amd64 syncthing-Linux-x86_64
downloadLinux syncthing-linux-arm64 syncthing-Linux-arm64
downloadLinux syncthing-macos-amd64 syncthing-Darwin-x86_64
downloadWindows




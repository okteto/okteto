#!/bin/sh

set -e

green="\033[32m"
red="\033[31m"
reset="\033[0m"

OS=$(uname | tr '[:upper:]' '[:lower:]')

case "$OS" in
    darwin) URL=https://downloads.okteto.com/cloud/cli/okteto-Darwin-x86_64;;
    linux) URL=https://downloads.okteto.com/cloud/cli/okteto-Linux-x86_64;;
    *) printf "$red> The OS (${OS}) is not supported by this installation script.$reset\n"; exit 1;;
esac

curl -L $URL -o /usr/local/bin/okteto
chmod +x /usr/local/bin/okteto

printf "$green> Successfully installed!\n$reset"
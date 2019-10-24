#!/bin/sh

{ # Prevent execution if this script was only partially downloaded

set -e

green="\033[32m"
red="\033[31m"
reset="\033[0m"
install_path='/usr/local/bin/okteto'
OS=$(uname | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | tr '[:upper:]' '[:lower:]')
cmd_exists() {
	command -v "$@" > /dev/null 2>&1
}

latestURL=https://github.com/okteto/okteto/releases/latest/download

case "$OS" in
    darwin) 
      URL=${latestURL}/okteto-Darwin-x86_64
      ;;
    linux)
      case "$ARCH" in
        x86_64) 
            URL=${latestURL}/okteto-Linux-x86_64
            ;;
        amd64) 
            URL=${latestURL}/okteto-Linux-x86_64
            ;;
        armv8*) 
            URL=${latestURL}/okteto-Linux-arm64
            ;;
        aarch64)
            URL=${latestURL}/okteto-Linux-arm64
            ;;
        *) 
            printf "$red> The architecture (${ARCH}) is not supported by this installation script.$reset\n" 
            exit 1 
            ;;
      esac
      ;;
    *)
      printf "$red> The OS (${OS}) is not supported by this installation script.$reset\n"
      exit 1
      ;;
esac

sh_c='sh -c'
if [ ! -w $install_path ]; then
    # use sudo if $user doesn't have write access to the path
    if [ "$user" != 'root' ]; then
        if cmd_exists sudo; then
            sh_c='sudo -E sh -c'
	elif cmd_exists su; then
            sh_c='su -c'
	else
            echo 'This script requires to run commands as sudo. We are unable to find either "sudo" or "su".'
            exit 1
	fi
    fi
fi

printf "> Installing $install_path\n"
$sh_c "rm -f $install_path"
$sh_c "curl -fSL $URL -o $install_path"
$sh_c "chmod +x $install_path"

printf "$green> Okteto successfully installed!\n$reset"

} # End of wrapping
#!/bin/sh

{ # Prevent execution if this script was only partially downloaded

        set -e

        install_dir='/usr/local/bin'
        install_path='/usr/local/bin/okteto'
        OS=$(uname | tr '[:upper:]' '[:lower:]')
        ARCH=$(uname -m | tr '[:upper:]' '[:lower:]')
        cmd_exists() {
                command -v "$@" >/dev/null 2>&1
        }

        latestURL=https://github.com/okteto/okteto/releases/latest/download

        case "$OS" in
        darwin)
                case "$ARCH" in
                x86_64)
                        URL=${latestURL}/okteto-Darwin-x86_64
                        ;;
                arm64)
                        URL=${latestURL}/okteto-Darwin-arm64
                        ;;
                *)
                        printf '\033[31m> The architecture (%s) is not supported by this installation script.\n\033[0m' "$ARCH"
                        exit 1
                        ;;
                esac
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
                        printf '\033[31m> The architecture (%s) is not supported by this installation script.\n\033[0m' "$ARCH"
                        exit 1
                        ;;
                esac
                ;;
        *)
                printf '\033[31m> The OS (%s) is not supported by this installation script.\n\033[0m' "$OS"
                exit 1
                ;;
        esac

        sh_c='sh -c'
        if [ ! -w "$install_dir" ]; then
                # use sudo if $USER doesn't have write access to the path
                if [ "$USER" != 'root' ]; then
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

        printf '> Downloading %s\n' "$URL"
        download_path=$(mktemp)
        curl -fSL "$URL" -o "$download_path"
        chmod +x "$download_path"

        printf '> Installing %s\n' "$install_path"
        $sh_c "mv -f $download_path $install_path"

        printf '\033[32m> Okteto successfully installed!\n\033[0m'

} # End of wrapping

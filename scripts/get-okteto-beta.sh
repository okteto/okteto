#!/bin/sh

{ # Prevent execution if this script was only partially downloaded
        printf '\033[0;31m'
        printf '\n'
        printf 'Installing okteto from https://beta.okteto.com has been deprecated\n'
        printf '\n'
        printf '\033[0m'
        printf 'Please install with:\n'
        printf '  $ curl https://get.okteto.com -sSfL | sh\n'
        printf '\n'
} # End of wrapping

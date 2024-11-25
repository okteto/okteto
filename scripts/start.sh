#!/bin/sh

log() {
        echo "$(date +%Y-%m-%dT%H:%M:%S)" "$1"
}
set -e

userID="$(id -u)"
echo "USER:$userID"
log "development container starting"
if [ -d "/var/okteto/cloudbin" ]; then
        if [ -w "/usr/local/bin" ]; then
                cp /var/okteto/cloudbin/* /usr/local/bin
        fi
fi

remote=""
reset=""
verbose="--verbose=false"

while getopts ":s:ervd" opt; do
        case $opt in
        e)
                reset="--reset"
                ;;
        r)
                remote="--remote"
                ;;
        v)
                verbose="--verbose"
                ;;
        d)
                if [ -z "${DOCKER_CONFIG}" ]; then
                        DOCKER_CONFIG="$HOME/.docker"
                fi
                if [ ! -d "${DOCKER_CONFIG}" ]; then
                        mkdir -p "${DOCKER_CONFIG}"
                fi
                if [ ! -f "${DOCKER_CONFIG}/config.json" ]; then
                        PASSWD=$(echo "${OKTETO_USERNAME}:${OKTETO_TOKEN}" | tr -d '\n' | base64 -i -w 0)
                        DOCKER_CONFIG_VALUE="{\n    \"auths\": {\n        \"${OKTETO_REGISTRY_URL}\": {\n            \"auth\": \"${PASSWD}\"\n        }\n    }\n}"
                        echo "${DOCKER_CONFIG_VALUE}" >"${DOCKER_CONFIG}/config.json"
                fi
                ;;
        s)
                sourceFILE="$(echo "$OPTARG" | cut -d':' -f1)"
                destFILE="$(echo "$OPTARG" | cut -d':' -f2)"
                dirName="$(dirname "$destFILE")"

                if [ ! -d "$dirName" ]; then
                        mkdir -p "$dirName"
                fi

                log "Copying secret $sourceFILE to $destFILE"
                if [ "/var/okteto/secret/$sourceFILE" != "$destFILE" ]; then
                        cp "/var/okteto/secret/$sourceFILE" "$destFILE"
                fi
                ;;
        \?)
                log "Invalid option: -$OPTARG" >&2
                exit 1
                ;;
        esac
done

syncthingHome=/var/syncthing
log "Copying configuration files to $syncthingHome"
cp /var/syncthing/secret/* $syncthingHome
chmod 644 $syncthingHome/cert.pem $syncthingHome/config.xml $syncthingHome/key.pem

log "Executing okteto-supervisor $remote $reset $verbose"
exec /var/okteto/bin/okteto-supervisor $remote $reset $verbose

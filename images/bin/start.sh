#!/bin/sh

userID="$(id -u)"
echo "USER:$userID"
if [ -d "/var/okteto/bin" ]; then
  if [ -w "/usr/local/bin" ]; then
    cp /var/okteto/bin/* /usr/local/bin
  else
    echo /usr/local/bin is not writeable by $userID
  fi
fi

set -e
remote=0
while getopts ":s:r" opt; do
  case $opt in
    r)
      remote=1
      ;;
    s)
      sourceFILE="$(echo $OPTARG | cut -d':' -f1)"
      destFILE="$(echo $OPTARG | cut -d':' -f2)"
      dirName="$(dirname $destFILE)"
      
      if [ ! -d "$dirName" ]; then
        mkdir -p $dirName
      fi
      
      echo "Copying secret $sourceFILE to $destFILE"
      cp -p /var/okteto/secret/$sourceFILE $destFILE
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
  esac
done

echo "Creating marker file $OKTETO_MARKER_PATH ..."
mkdir -p "$(dirname "$OKTETO_MARKER_PATH")"
touch $OKTETO_MARKER_PATH

syncthingHome=/var/syncthing
echo "Copying configuration files to $syncthingHome ..."
cp /var/syncthing/secret/* $syncthingHome

params=""
if [ $remote -eq 1 ]; then
params="--remote"
fi

echo "Executing supervisor..." 
exec /var/okteto/bin/supervisor $params
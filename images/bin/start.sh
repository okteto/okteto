#!/bin/sh

set -e

userID="$(id -u)"
echo "USER:$userID"
if [ -d "/var/okteto/bin" ]; then
  if [ -w "/usr/local/bin" ]; then
    cp /var/okteto/bin/* /usr/local/bin
  else
    echo /usr/local/bin is not writeable by $userID
  fi
fi

echo "Creating marker file $OKTETO_MARKER_PATH ..."
oktetoFolder=$(dirname "$OKTETO_MARKER_PATH")

if [ ! -w "${oktetoFolder}" ]; then
    echo \"${oktetoFolder}\" is not writeable by $userID
    exit 1
fi

mkdir -p "${oktetoFolder}"
touch $OKTETO_MARKER_PATH

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
      if [ "/var/okteto/secret/$sourceFILE" != "$destFILE" ]; then
        cp -p /var/okteto/secret/$sourceFILE $destFILE
      fi
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
  esac
done

syncthingHome=/var/syncthing
echo "Copying configuration files to $syncthingHome ..."
cp /var/syncthing/secret/* $syncthingHome

params=""
if [ $remote -eq 1 ]; then
params="--remote"
fi

echo "Executing supervisor..." 
exec /var/okteto/bin/supervisor $params
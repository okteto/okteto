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

remote=0
ephemeral=0
while getopts ":s:re" opt; do
  case $opt in
    e)
      ephemeral=1
      ;;
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
if [ $ephemeral -eq 1 ] && [ -f ${syncthingHome}/executed ]; then
    echo "failing: syncthing restarted and persistent volumes are not enabled in the okteto manifest. Run 'okteto down' and try again"
    exit 1
fi
touch ${syncthingHome}/executed
echo "Copying configuration files to $syncthingHome ..."
cp /var/syncthing/secret/* $syncthingHome

params=""
if [ $remote -eq 1 ]; then
    params="--remote"
fi

echo "Executing supervisor..." 
exec /var/okteto/bin/supervisor $params
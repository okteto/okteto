#!/bin/sh

set -e

if [ "$(ls -A /src)" ]; then
	echo "Volume was already initialized."
	exit 0 
fi

echo "Waiting for tarball to be uploaded..."
i=0
while [ ! -f /initialized ] && [ "$i" -lt 60 ]; do
	sleep 1
done

if [ "$i" -eq 60 ]; then
	echo "Tarball not uploaded after 1 minute."
	exit 0
fi

echo "Volume is now initialized."

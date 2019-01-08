#!/bin/sh

set -e

echo "Copying configuration files..."
cp /var/syncthing/secret/* /var/syncthing/config

echo "Executing syncthing..." 
/bin/syncthing -home /var/syncthing/config -gui-address 0.0.0.0:8384

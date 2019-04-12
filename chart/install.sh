#! /bin/sh
helm upgrade --install -f override.yaml $(whoami)-okteto ./okteto
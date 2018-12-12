#!/bin/sh
#
# The script fetches the latest cnd binary

OS="$(uname)"
if [ "x${OS}" = "xDarwin" ] ; then
  OSEXT="darwin"
else
  # TODO we should check more/complain if not likely to work, etc...
  OSEXT="linux"
fi

if [ "x${CND_VERSION}" = "x" ] ; then
  CND_VERSION=$(curl -L -s https://api.github.com/repos/okteto/cnd/releases/latest | \
                  grep tag_name | sed "s/ *\"tag_name\": *\"\\(.*\\)\",*/\\1/")
fi

if [ -f /usr/local/bin/cnd ]; then
    echo "/usr/local/bin/cnd already exists. Delete the existing file and run this script again if you want to install cnd version $CND_VERSION."
fi

NAME="cnd-$CND_VERSION"
URL="https://github.com/okteto/cnd/releases/download/${CND_VERSION}/cnd-${OSEXT}-amd64"
echo "Downloading $NAME from $URL ..."
curl -L "$URL" > $NAME
chmod +x $NAME
echo "Downloaded $NAME"
BINDIR="$(pwd)"

mv $NAME /usr/local/bin/cnd
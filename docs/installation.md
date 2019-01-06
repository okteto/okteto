# Installation

## Homebrew install

```console
brew tap okteto/cnd
brew install cnd
```

You can install to the latest unstable version by executing:
```console
brew tap okteto/cnd
brew install --HEAD cnd
```

## Manual install

The synching functionality of **cnd** is provided by [syncthing](https://docs.syncthing.net).

To install `syncthing`, download the corresponding binary from their [releases page](https://github.com/syncthing/syncthing/releases).

**cnd** assumes that synchting is in the path, to verify, run the following:
```console
which syncthing
```

Install **cnd** from source by executing:

```console
go get github.com/okteto/cnd
```

You can get also get a prebuilt binary [from our releases page](https://github.com/okteto/cnd/releases/latest)

# Upgrade

## Homebrew
Upgrade to the latest stable version by executing:
```console
brew upgrade cnd
```

Upgrade to the latest unstable version by executing:
```console
brew upgrade --HEAD cnd
```

## Manually 
You can get our latest available binary [from our releases page](https://github.com/okteto/cnd/releases/latest). 
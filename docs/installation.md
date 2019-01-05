# Installation

## Homebrew install

```console
brew tap okteto/cnd
brew install cnd
```

## Manual install

The synching functionality of **cnd** is provided by [syncthing](https://docs.syncthing.net).

To install `syncthing`, download the corresponding binary from their [releases page](https://github.com/syncthing/syncthing/releases).

**cnd** assumes that synchting is in the path, to verify, run the following:
```console
which syncthing
```

Install **cnd** from by executing:

```console
go get github.com/okteto/cnd
```

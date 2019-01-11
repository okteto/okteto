# Installation

You will need the following components to get started with cnd:

## CND

### Hombrew

```console
brew tap okteto/cnd
brew install cnd
```

You can install to the latest unstable version by executing:
```console
brew tap okteto/cnd
brew install --HEAD cnd
```

### Manual install

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

## Kubernetes
We've tested cnd with [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/), [GKE](https://cloud.google.com/kubernetes-engine/), and [Digital Ocean Kubernetes](https://www.digitalocean.com/products/kubernetes/) but any Kubernetes cluster will work. 


## Kubectl
cnd uses your local kubectl [installation and configuration](https://kubernetes.io/docs/tasks/tools/install-kubectl). Configure the current-context with your target cluster for development.

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

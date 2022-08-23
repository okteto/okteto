# Okteto: A Tool to Develop Applications on Kubernetes

[![CircleCI](https://circleci.com/gh/okteto/okteto.svg?style=svg)](https://circleci.com/gh/okteto/okteto)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3055/badge)](https://bestpractices.coreinfrastructure.org/projects/3055)
[![GitHub release](https://img.shields.io/github/release/okteto/okteto.svg?style=flat-square)](https://github.com/okteto/okteto/releases)
[![Apache License 2.0](https://img.shields.io/github/license/okteto/okteto.svg?style=flat-square)](https://github.com/okteto/okteto/blob/master/LICENSE)
![Total Downloads](https://img.shields.io/github/downloads/okteto/okteto/total?logo=github&logoColor=white)
[![Chat in Slack](https://img.shields.io/badge/slack-@kubernetes/okteto-red.svg?logo=slack)](https://kubernetes.slack.com/messages/CM1QMQGS0/)

## Overview

Kubernetes has made it very easy to deploy applications to the cloud at a higher scale than ever, but the development practices have not evolved at the same speed as application deployment patterns.

Today, most developers try to either run parts of the infrastructure locally or just test these integrations directly in the cluster via CI jobs, or the _docker build/redeploy_ cycle. It works, but this workflow is painful and incredibly slow.

`okteto` accelerates the development workflow of Kubernetes applications. You write your code locally and `okteto` detects the changes and instantly updates your Kubernetes applications.

## How it works

Okteto allows you to develop inside a container. When you run `okteto up` your Kubernetes deployment is replaced by a development container that contains your development tools (e.g. maven and jdk, or npm, python, go compiler, debuggers, etc). This development container can be any [docker image](https://okteto.com/docs/reference/development-environments/). The development container inherits the same secrets, configmaps, volumes or any other configuration value of the original Kubernetes deployment.

In addition to that, `okteto up` will:

1. Create a bidirectional file [synchronization service](https://okteto.com/docs/reference/file-synchronization/) to keep your changes up to date between your local filesystem and your development container.
1. Automatic local and remote port forwarding using [SSH](https://okteto.com/docs/reference/ssh-server/), so you can access your cluster services via `localhost` or connect a remote debugger.
1. Give you an interactive terminal to your development container, so you can build, test, and run your application as you would from a local terminal.

All of this (and more) can be configured via a [simple YAML manifest](https://okteto.com/docs/reference/manifest/).

The end result is that the remote cluster is seen by your IDE and tools as a local filesystem/environment. You keep writing your code on your local IDE and as soon as you save a file, the change goes to the development container, and your application instantly updates (taking advantage of any hot-reload mechanism you already have). This whole process happens in an instant. No docker images need to be created and no Kubernetes manifests need to be applied to the cluster.

![Okteto](docs/okteto-architecture.png)

## Why Okteto

`okteto` has several advantages when compared to more traditional development approaches:

- **Fast inner loop development**: build and run your application using your favorite tools directly from your development container. Native builds are always faster than the _docker build/redeploy_ cycle.
- **Realistic development environment**: your development container reuses the same variables, secrets, sidecars, volumes, etc... than your original Kubernetes deployment. Realistic environments eliminate integration issues.
- **Replicability**: development containers eliminate the need to install your dependencies locally, everything is pre-configured in your development image.
- **Unlimited resources**: get access to the hardware and network of your cluster when developing your application.
- **Deployment independent**: `okteto` decouples deployment from development. You can deploy your application with kubectl, Helm, a serverless framework, or even a CI pipeline and use `okteto up` to develop it. This is especially useful for cloud-native applications where deployment pipelines are not trivial.
- **Works anywhere**: `okteto` works with any Kubernetes cluster, local or remote. `okteto` is also available for macOS, Linux, and Windows.

## Getting started

All you need to get started is to [install the Okteto CLI](https://www.okteto.com/docs/getting-started/#installing-okteto-cli) and have access to a Kubernetes cluster.

Okteto CLI works with **any** Kubernetes cluster. If it's your first time using it, we'd recommend you [try it](https://www.okteto.com/docs/getting-started/) with the [Okteto Platform](https://cloud.okteto.com/) for a complete holistic developer experience. If you want to try it out with any other K8s cluster, you can also check out [this article](https://www.okteto.com/blog/developing-microservices-by-hot-reloading-on-kubernetes-clusters/) as a guide.

We created a [few guides to help you get started](https://github.com/okteto/samples) with `okteto` and your favorite programming language.

### Releases

Okteto is released into three channels: stable, beta, and dev. By default when okteto is installed the stable channel is used. If you need to access features not yet widely available you can install from the beta or dev channel. More info in the [release documentation](docs/RELEASE.md).

## Useful links

- [Getting started](https://www.okteto.com/docs/getting-started/)
- [CLI reference](https://okteto.com/docs/reference/cli)
- [Okteto manifest reference](https://okteto.com/docs/reference/manifest/)
- [Samples](https://github.com/okteto/samples)
- Frequently asked questions ([FAQs](https://okteto.com/docs/reference/faqs/))
- [Known issues](https://okteto.com/docs/reference/known-issues/)

## Support and Community

Got questions? Have feedback? Join the conversation in our [#okteto](https://kubernetes.slack.com/messages/CM1QMQGS0/) Slack channel! If you don't already have a Kubernetes Slack account, [sign up here](https://slack.k8s.io/).

Follow [@OktetoHQ](https://twitter.com/oktetohq) on Twitter for important announcements.

## Roadmap

We use GitHub [issues](https://github.com/okteto/okteto/issues) to track our roadmap. A [milestone](https://github.com/okteto/okteto/milestones) is created every month to track the work scheduled for that time period. Feedback and help are always appreciated!

## ✨ Contributions

We ❤️ contributions big or small. [See our guide](contributing.md) on how to get started.

### Thanks to all our contributors!

<a href="https://github.com/okteto/okteto/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=okteto/okteto" />
</a>
<!--  https://contrib.rocks -->

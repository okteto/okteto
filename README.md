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

Okteto allows you to develop inside a container. When you run `okteto up` your Kubernetes deployment is replaced by a development container that contains your development tools (e.g. maven and jdk, or npm, python, go compiler, debuggers, etc). This development container can be any [docker image](https://okteto.com/docs/reference/development-environment/). The development container inherits the same secrets, configmaps, volumes or any other configuration value of the original Kubernetes deployment.

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

All you need to get started is to [install the Okteto CLI](https://okteto.com/docs/getting-started/installation/) and have access to a Kubernetes cluster.

You can also use `okteto` with [Okteto Cloud](https://okteto.com/), a **Kubernetes Namespace as a Service** platform where you can deploy your Kubernetes applications and development containers for free.

### Super Quick Start

- Deploy your application on Kubernetes.
- Run `okteto init` from the root of your git repository to inspect your code and generate your [Okteto manifest](https://okteto.com/docs/reference/manifest/). The Okteto manifest defines your development container.
- Run `okteto up` to deploy your development container.

We created a [few guides to help you get started](https://github.com/okteto/samples) with `okteto` and your favorite programming language.

## Useful links

- [Installation guides](https://okteto.com/docs/getting-started/installation/)
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

## Contributions

We ❤️ contributions big or small. [See our guide](contributing.md) on how to get started.

### Thanks to all our contributors!

[//]: contributor-faces

<a href="https://github.com/pchico83"><img src="https://avatars.githubusercontent.com/u/7474696?v=4" title="pchico83" width="80" height="80"></a>
<a href="https://github.com/rberrelleza"><img src="https://avatars.githubusercontent.com/u/475313?v=4" title="rberrelleza" width="80" height="80"></a>
<a href="https://github.com/jLopezbarb"><img src="https://avatars.githubusercontent.com/u/25170843?v=4" title="jLopezbarb" width="80" height="80"></a>
<a href="https://github.com/jbampton"><img src="https://avatars.githubusercontent.com/u/418747?v=4" title="jbampton" width="80" height="80"></a>
<a href="https://github.com/rlamana"><img src="https://avatars.githubusercontent.com/u/237819?v=4" title="rlamana" width="80" height="80"></a>
<a href="https://github.com/marco2704"><img src="https://avatars.githubusercontent.com/u/12150248?v=4" title="marco2704" width="80" height="80"></a>
<a href="https://github.com/adhaamehab"><img src="https://avatars.githubusercontent.com/u/13816742?v=4" title="adhaamehab" width="80" height="80"></a>
<a href="https://github.com/danielhelfand"><img src="https://avatars.githubusercontent.com/u/34258252?v=4" title="danielhelfand" width="80" height="80"></a>
<a href="https://github.com/glensc"><img src="https://avatars.githubusercontent.com/u/199095?v=4" title="glensc" width="80" height="80"></a>
<a href="https://github.com/ifbyol"><img src="https://avatars.githubusercontent.com/u/3510171?v=4" title="ifbyol" width="80" height="80"></a>
<a href="https://github.com/kivi"><img src="https://avatars.githubusercontent.com/u/366163?v=4" title="kivi" width="80" height="80"></a>
<a href="https://github.com/aguthrie"><img src="https://avatars.githubusercontent.com/u/210097?v=4" title="aguthrie" width="80" height="80"></a>
<a href="https://github.com/alanmbarr"><img src="https://avatars.githubusercontent.com/u/760506?v=4" title="alanmbarr" width="80" height="80"></a>
<a href="https://github.com/AnesBenmerzoug"><img src="https://avatars.githubusercontent.com/u/27914730?v=4" title="AnesBenmerzoug" width="80" height="80"></a>
<a href="https://github.com/borjaburgos"><img src="https://avatars.githubusercontent.com/u/3640206?v=4" title="borjaburgos" width="80" height="80"></a>
<a href="https://github.com/Cirrith"><img src="https://avatars.githubusercontent.com/u/4418305?v=4" title="Cirrith" width="80" height="80"></a>
<a href="https://github.com/ironcladlou"><img src="https://avatars.githubusercontent.com/u/298299?v=4" title="ironcladlou" width="80" height="80"></a>
<a href="https://github.com/unthought"><img src="https://avatars.githubusercontent.com/u/1222558?v=4" title="unthought" width="80" height="80"></a>
<a href="https://github.com/drodriguezhdez"><img src="https://avatars.githubusercontent.com/u/29516565?v=4" title="drodriguezhdez" width="80" height="80"></a>
<a href="https://github.com/snitkdan"><img src="https://avatars.githubusercontent.com/u/15274429?v=4" title="snitkdan" width="80" height="80"></a>
<a href="https://github.com/fermayo"><img src="https://avatars.githubusercontent.com/u/3635457?v=4" title="fermayo" width="80" height="80"></a>
<a href="https://github.com/irespaldiza"><img src="https://avatars.githubusercontent.com/u/11633327?v=4" title="irespaldiza" width="80" height="80"></a>
<a href="https://github.com/jmacelroy"><img src="https://avatars.githubusercontent.com/u/30531294?v=4" title="jmacelroy" width="80" height="80"></a>
<a href="https://github.com/dekkers"><img src="https://avatars.githubusercontent.com/u/656182?v=4" title="dekkers" width="80" height="80"></a>
<a href="https://github.com/thatnerdjosh"><img src="https://avatars.githubusercontent.com/u/5251847?v=4" title="thatnerdjosh" width="80" height="80"></a>
<a href="https://github.com/freeman"><img src="https://avatars.githubusercontent.com/u/7547?v=4" title="freeman" width="80" height="80"></a>
<a href="https://github.com/tommyto-whs"><img src="https://avatars.githubusercontent.com/u/59745049?v=4" title="tommyto-whs" width="80" height="80"></a>
<a href="https://github.com/Wignesh"><img src="https://avatars.githubusercontent.com/u/26745858?v=4" title="Wignesh" width="80" height="80"></a>
<a href="https://github.com/zdog234"><img src="https://avatars.githubusercontent.com/u/17930657?v=4" title="zdog234" width="80" height="80"></a>
<a href="https://github.com/marov"><img src="https://avatars.githubusercontent.com/u/1968182?v=4" title="marov" width="80" height="80"></a>
<a href="https://github.com/xinxinh2020"><img src="https://avatars.githubusercontent.com/u/13103635?v=4" title="xinxinh2020" width="80" height="80"></a>

[//]: contributor-faces

# Okteto: A Tool to Develop Applications in Kubernetes

[![GitHub release](http://img.shields.io/github/release/okteto/okteto.svg?style=flat-square)][release]
[![CircleCI](https://circleci.com/gh/okteto/okteto.svg?style=svg)](https://circleci.com/gh/okteto/okteto)
[![Scope](https://app.scope.dev/api/badge/1fb9ca0d-7612-4ae9-b9c6-b901c39e8f7b/default)](https://app.scope.dev/external/v1/explore/57ca820b-5f4b-472c-a90c-b99f61c0f120/1fb9ca0d-7612-4ae9-b9c6-b901c39e8f7b/default?branch=master)
[![Apache License 2.0](https://img.shields.io/github/license/okteto/okteto.svg?style=flat-square)][license]

[release]: https://github.com/okteto/okteto/releases
[license]: https://github.com/okteto/okteto/blob/master/LICENSE
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3055/badge)](https://bestpractices.coreinfrastructure.org/projects/3055)

## Overview

Kubernetes has made it very easy to deploy applications to the cloud at a higher scale than ever, but the development practices have not evolved at the same speed as application deployment patterns.

Today, most developers try to either run parts of the infrastructure locally, or just test these integrations directly in the cluster via CI jobs or the "docker build, docker push, kubectl apply" cycle. It works, but this workflow is painful and incredibly slow.

Okteto makes this cycle a lot faster by launching development environment in Kubernetes. You write your code locally using your favorite tools and Okteto detects the code changes and instantly synchronizes your code to your development environment. There you can use your compilers and hot reloaders to see your changes live in seconds.

## Install

MacOS / Linux
```
curl https://get.okteto.com -sSfL | sh
```

Windows
```
Download https://downloads.okteto.com/cli/okteto.exe and add it to your $PATH.
```

You can also directly download the precompiled binary from [the releases page](https://github.com/okteto/okteto/releases).

## Getting started

All you need to get started is the [Okteto CLI](https://okteto.com/docs/getting-started/installation/index.html) and access to a Kubernetes cluster. 

We created a few guides to get you started with your favorite programming language:

1. [ASP.NET Core](https://github.com/okteto/aspnetcore-getting-started/)
1. [Golang](https://github.com/okteto/go-getting-started)
1. [Java Gradle](https://github.com/okteto/java-gradle-getting-started)
1. [Java Maven](https://github.com/okteto/java-maven-getting-started)
1. [Node](https://github.com/okteto/node-getting-started)
1. [PHP](https://github.com/okteto/php-getting-started)
1. [Python](https://github.com/okteto/samples/tree/master/python)

More samples on how to use Okteto [are available here](https://github.com/okteto/samples).

## Features

### Development environments on demand 

Your development environment is defined in a [simple yaml manifest](https://okteto.com/docs/reference/manifest).
- Run `okteto init` to inspect your project and generate your own config file.
- Run `okteto up` to launch your development environment in seconds. 

Add `okteto.yml` to your repo and make collaboration easier than ever. Clone the repository and simply run `okteto up` to launch a fully configured development environment.

### Developer Mode 

You can swap your development environment with an existing Kubernetes deployment, and develop directly in your cluster. This helps eliminate integration issues since you're developing the same way your application runs in production.

Okteto supports applications with one or with multiple services.

### Instant file synchronization

Okteto detects your code changes and synchronizes your code to your development environment (powered by [Syncthing](https://github.com/syncthing/syncthing)). Files are synchronized both ways. If you edit a file directly in your development environment, the changes will be reflected locally as well. Great for keeping your `package-lock.json` or `requirements.txt` up to date.

### Keep Your Own Tools

Okteto easily integrates and augments your existing tools. You write code locally, which means Okteto is compatible with any IDE. And you can configure your own development environment runtime using a docker image, with your own compilers, linters, debuggers or hot reloaders.

Okteto decouples deployment from development. Because of this, you can deploy your application with kubectl, Helm, a serverless framework or even a CI job and use Okteto later to develop any component of your application. No need to change your manifests or deployment scripts.

### Compatible with any Kubernetes cluster

Local or remote. If you can run `kubectl apply` you can use Okteto. Our community uses Okteto in all major Kubernetes distros, from Minikube and k3s all the way to GKE, Digital Ocean, AKS, EKS and Civio.

You can also use Okteto in [Okteto Cloud](https://okteto.com/), the best development platform for Kubernetes applications.

## Learn More
- [How does Okteto work?](docs/how-does-it-work.md)
- Get started following our [installation guides](https://okteto.com/docs/getting-started/installation/index.html).
- Check the Okteto [CLI reference](https://okteto.com/docs/reference/cli) and the [okteto.yml reference](https://okteto.com/docs/reference/manifest/index.html)
- [Explore our samples](https://github.com/okteto/samples) to learn more about the power of Okteto
- Frequently asked questions ([FAQs](https://okteto.com/docs/reference/faqs/index.html))
- [Known issues](https://okteto.com/docs/reference/known-issues/index.html)

## Roadmap and Contributions

Okteto is written in Go under the [Apache 2.0 license](LICENSE) - contributions are welcomed whether that means providing feedback, testing existing and new feature or hacking on the source.

## How do I become a contributor?

Please see the guide on [contributing](contributing.md).

## Roadmap

We use GitHub [issues](https://github.com/okteto/okteto/issues) to track our roadmap. A [milestone](https://github.com/okteto/okteto/milestones) is created every month to track the work scheduled for that time period. Feedback and help are always appreciated!

## Stay in Touch
Got questions? Have feedback? Join [the conversation in Slack](https://kubernetes.slack.com/messages/CM1QMQGS0/)! If you don't already have a Kubernetes slack account, [sign up here](http://slack.k8s.io/). 

Follow [@OktetoHQ](https://twitter.com/oktetohq) on Twitter for important announcements.

Or get in touch with the maintainers:

- [Pablo Chico de Guzman](https://twitter.com/pchico83)
- [Ramiro Berrelleza](https://twitter.com/rberrelleza)
- [Ramon Lamana](https://twitter.com/monchocromo)

## About Okteto
[Okteto](https://okteto.com) is licensed under the Apache 2.0 License.

This project adheres to the Contributor Covenant [code of conduct](code-of-conduct.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to hello@okteto.com.

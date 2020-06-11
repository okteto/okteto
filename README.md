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

Today, most developers try to either run parts of the infrastructure locally or just test these integrations directly in the cluster via CI jobs or the *docker build/redeploy* cycle. It works, but this workflow is painful and incredibly slow.

Okteto makes this cycle a lot faster. You write your code locally from your favorite IDE and Okteto detects the code changes and instantly updates your Kubernetes applications. **It works on any cluster, local or remote**.

## How it works

Okteto decouples deployment from development. You can deploy your application with kubectl, Helm, a serverless framework, or even a CI pipeline and use Okteto to develop it. This is especially useful for cloud-native applications where deployment pipelines are not trivial.

Okteto replaces a running deployment by a development container. You write code from your local IDE and Okteto instantly synchronizes your code changes to your development container. The development container can be any docker image, where you install all the dev tools needed by your application: compilers, debuggers, hot reloaders... Okteto gives you a terminal to your development container to build, test, and run your application as you would from a local terminal. Files are synchronized both ways.

![Okteto](docs/okteto-architecture.png)

## Why Okteto

Okteto has several advantages versus traditional development:
- **Fast inner loop development**: build and run with your favorite tools from your development container. Native builds are always faster than the *docker build/redeploy* cycle.
- **Production-like environment**: your development container reuses the same variables, secrets, sidecars, volumes... than your original Kubernetes deployment. Realistic environments eliminate integration issues.
- **Unlimited resources**: get access to the hardware and network of your cluster from development.

## Getting started

All you need to get started is to [install the Okteto CLI](https://okteto.com/docs/getting-started/installation/index.html) and have access to a Kubernetes cluster. You can also use Okteto in [Okteto Cloud](https://okteto.com/), the best development platform for Kubernetes applications.

To start using Okteto to develop your own applications:

- Deploy your application on Kubernetes.
- Run `okteto init` from the root of your git repository to inspect your code and generate your [Okteto manifest](https://okteto.com/docs/reference/manifest). The Okteto manifest defines your development container.
- Run `okteto up` to put your application in developer mode. 

We also created a [few guides to get you started](https://github.com/okteto/samples) with your favorite programming language.

## Useful links

- [Installation guides](https://okteto.com/docs/getting-started/installation/index.html)
- [CLI reference](https://okteto.com/docs/reference/cli)
- [Okteto manifest reference](https://okteto.com/docs/reference/manifest/index.html)
- [Samples](https://github.com/okteto/samples)
- Frequently asked questions ([FAQs](https://okteto.com/docs/reference/faqs/index.html))
- [Known issues](https://okteto.com/docs/reference/known-issues/index.html)

## Roadmap and Contributions

Okteto is written in Go under the [Apache 2.0 license](LICENSE) - contributions are welcomed whether that means providing feedback, testing a new feature, or hacking on the source.

### How do I become a contributor?

Please see the guide on [contributing](contributing.md).

### Roadmap

We use GitHub [issues](https://github.com/okteto/okteto/issues) to track our roadmap. A [milestone](https://github.com/okteto/okteto/milestones) is created every month to track the work scheduled for that time period. Feedback and help are always appreciated!

## Stay in Touch
Got questions? Have feedback? Join the conversation in our [#okteto](https://kubernetes.slack.com/messages/CM1QMQGS0/) Slack channel! If you don't already have a Kubernetes slack account, [sign up here](http://slack.k8s.io/). 

Follow [@OktetoHQ](https://twitter.com/oktetohq) on Twitter for important announcements.

Or get in touch with the maintainers:

- [Pablo Chico de Guzman](https://twitter.com/pchico83)
- [Ramiro Berrelleza](https://twitter.com/rberrelleza)
- [Ramon Lamana](https://twitter.com/monchocromo)

## About Okteto

Okteto is licensed under the Apache 2.0 License.

This project adheres to the Contributor Covenant [code of conduct](code-of-conduct.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to hello@okteto.com.

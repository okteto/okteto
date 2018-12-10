# Cloud Native Development (CND)

[![CircleCI](https://circleci.com/gh/okteto/cnd.svg?style=svg)](https://circleci.com/gh/okteto/cnd)

**Cloud Native Development** (CND) is about moving your entire development workflow to kubernetes, avoiding the time-consuming `docker build/push/pull/redeploy` cycle.

CND helps you achieve this with a mix of kubernetes automation, file synching between your local file system and kubernetes and hot reloading of containers.

## Quickstart

Let's start with something simple to show you the power of cloud native development. First, install [cnd locally](#installation).  

Clone the `voting-app` service.

```console
git clone https://github.com/okteto/welcome
```

Deploy the welcome service by running the following command.
```console
$ kubectl create -f k8-specifications 
deployment.apps/welcome created
service/welcome created

$ kubectl get service welcome
NAME      TYPE           CLUSTER-IP     EXTERNAL-IP      PORT(S)        AGE
welcome   LoadBalancer   10.15.255.73   35.204.101.246   80:30879/TCP   20s
```
It might take a minute or two for the service to be up and running depending on your cluster.

*If you're using minikube, you'll need to either change the service to use a NodePort, or enable load balancers in your instance.*

Open your browser and hit the external-ip. You should see our welcome service, a place to vote on whether you prefer dogs or cats. Go ahead and place a few votes on your preferred species. 

Now that your service is running, it's time to do some **cloud native development**. Run the following command on your terminal

```console
$ cnd up
Activating your cloud native development environment...
Linking '/Users/ramiro/okteto/welcome' to ramiro/welcome...
Ready! Go to your local IDE and continue coding!
```

Open your favorite IDE, go to `app.py` and change the value of  `option_a` (line 7) from `Cats` to `Otters` and save the file. Go to the browser, reload the page, and check the label on the first button. Your changes were applied instantly and automatically!

Congratulations, you're now a **cloud native developer** 😎.

For a more advanced scenario involving a microservices-based application, [check out our voting app cnd demo](https://github.com/okteto/cnd-voting-demo).

## Installation

### Homebrew install

```bash
brew tap okteto/cnd
brew install cnd
```

### Manual install

The synching functionality of **cnd** is provided by [syncthing](https://docs.syncthing.net).

To install `syncthing`, download the corresponding binary from their [releases page](https://github.com/syncthing/syncthing/releases).

**cnd** assumes that synchting is in the path, to verify, run the following:
```
which syncthing
```

Install **cnd** from by executing:

```bash
go get github.com/okteto/cnd
```

## How does it work

This is how a standard dev environment looks like:

<img align="left" src="docs/env.png">

&nbsp;

And this how it looks after converting it into a cloud native environment:

<img align="left" src="docs/cnd.png">
&nbsp;

The **cnd** container duplicates the manifest of the **api** pod, so it is fully integrated with every Kubernetes feature.

Local changes are synched to the **cnd** container via `syncthing`. As you save locally, it will be automatically synched in your **cnd** container in seconds.

Once you're ready to integrate, you can revert back to your original configuration for general end-to-end testing before sending a PR or pushing to production.




## Usage

Note: these instructions assume that you already have a kubernetes-based application running.

Define your Cloud Native Development file (`cnd.yml`). A `cnd.yml` looks like this:

```yaml
swap:
  deployment:
    name: webserver
    container: nginx
    image: nginx:alpine
mount:
  source: .
  target: /src
```

For more information about the Cloud Native Development file, see its [reference](/docs/cnd-file.md).

To convert your dev environment to a cloud native environment, execute:

```bash
cnd up
```

by default, it uses a `cnd.yml` in your current folder. For using a different file, execute:

```bash
cnd up -f path-to-cnd-file
```

To create a long-running session to your cloud native environment, execute:

```bash
cnd exec sh
```

You can also execute standalone commands like:

```bash
cnd exec go test
```

In order to revert back to your original configuration, execute:

```bash
cnd down
```

For a full demo of Cloud Native Development, check the [Voting App demo](https://github.com/okteto/cnd-voting-demo).

## Troubleshooting

### Files are not syncing
cnd uses  [syncthing](https://docs.syncthing.ne) to sync files between your environments. If your cloud native environment is not being updated correctly, review the following:

1. The `cnd up` process is running
1. Verify that syncthing is running on your environment (there should be two processes per cnd environment running)
1. Rerun `cnd up` (give it a few minutes to reestablish synchronization)

### Files syncing is slow
Please follow [syncthing's docs](https://docs.syncthing.net/users/faq.html#why-is-the-sync-so-slow) to troubleshoot this.

# About cnd
cnd was originally created by [Okteto](https://okteto.com) and is licensed under the Apache 2.0 License.

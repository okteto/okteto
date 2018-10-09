# Cloud Native Development (CND)

## Develop with Kubernetes at the speed of light

Leverage the power of Docker and Kubernetes without changing the way you code.

**Cloud Native Development** (CND) runs container-based environment in your Kubernetes cluster,
reducing integration efforts and synching local code changes to your remote environment.
The synchying process avoids the time-consuming `docker build/push/pull/redeploy` cycle.

This is how a standard dev environment looks like:

<img align="left" src="docs/env.png">

And this how it looks after running a cloud native environment:

<img align="left" src="docs/cnd.png">

Note that the **cnd** container duplicates the manifest of the **api** pod, so it is fully integrated with every Kubernetes feature.
Also, local changes are synched to the **cnd** container. Write code locally but run it in your Kubernetes clusters in just a few seconds.
Once you are happy with your feature, you can always swap back the cloud native environment and check your changes with your original dev environment.


## Installation

The synching functionality of **cnd** is provided by [ksync](https://github.com/vapor-ware/ksync), which is a **cnd** pre-requisite.

To install `ksync`, execute:

```bash
curl https://vapor-ware.github.io/gimme-that/gimme.sh | bash
```

and:

```
ksync init --image=vaporio/ksync:0.3.2-hotfix
```

check the `ksync` installation by running:

```bash
ksync doctor
```

If `ksync` is successfully installed, you can install **cnd** from the sources by running:

```bash
go get github.com/okteto/cnd
```

## Usage

**cnd** assumes you have defined the Kubernetes manifests that make up your app and run them in your Kubernetes cluster.

Now define your your Cloud Native Development file (`cnd.yml`). A `cnd.yml` looks like this:

```yaml
name: dev
swap:
  deployment:
    file: nginx-deployment.yml
    container: nginx
    image: ubuntu
  service:
    file: nginx-service.yml
mount:
  source: .
  target: /src
```

For more information about the Cloud Native Development file, see its [reference](/docs/cnd-file.md).

In order to create a cloud native environment, execute:

```bash
cnd up
```

by default, it uses a `cnd.yml` in your current folder. For using a different file, execute:

```bash
cnd up -f path-to-cnd-file
```

Now, you can create a long running session to your cloud native environment by running:

```bash
cnd exec sh
```

or execute a standalone command like this one:

```bash
cnd exec go test
```

Finally, in order to destroy your cloud native environment, execute:

```bash
cnd down
```
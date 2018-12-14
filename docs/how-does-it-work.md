## How does CND work?

This is how a standard dev environment looks like:

<img align="left" src="env.png">

&nbsp;

And this how it looks after converting it into a cloud native environment:

<img align="left" src="cnd.png">
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

For more information about the Cloud Native Development file, see its [reference](cnd-file.md).

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
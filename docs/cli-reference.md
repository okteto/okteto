# Useful commands

Define your cloud native development environment using a `cnd.yml` file. A `cnd.yml` file looks like this:

```yaml
swap:
  deployment:
    name: welcome
    container: welcome
mount:
  source: .
  target: /src
```

For more information about the Cloud Native Development file, see its [reference](docs/cnd-file.md).

To convert your dev environment to a cloud native environment, execute:

```console
cnd up
```

by default, it uses a `cnd.yml` in your current folder. For using a different file, execute:

```console
cnd up -f path-to-cnd-file
```

From this moment, your local changes will be synched to the remote container.

To create a long-running session to your cloud native environment, execute:

```console
cnd exec sh
```

You can also execute standalone commands like:

```console
cnd exec go test
```

In order to revert back to your original configuration, execute:

```console
cnd down
```

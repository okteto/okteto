# CND Yaml Reference

A Cloud Native Development file (`cnd.yml`) defines the container to be swapped in your dev environment by a container that hot reloads your local changes.

Below is an example of a `cnd.yml`:

```yaml
swap:
  deployment:
    name: welcome
    container: welcome
    image: okteto/welcome
mount:
  source: .
  target: /src
scripts:
  test: "python -m test"
```

## swap.deployment.name (required)

The name of the deployment to be replaced.

## swap.deployment.container (required)

The name of the container to be replaced.

## swap.deployment.image (optional)

The docker image to use by the cloud native environment. (default: the existing container image).

## swap.deployment.command (optional)

The command to be executed by the cloud native environment.

It has to be a non-finishing command (default: `tail -f /dev/null`)

## mount.source (optional)

The local folder synched to the remote container. (default: the current folder)

## mount.target (required)

The remote folder path synched with the local file system.

## swap.scripts (optional)

You may define scripts in your cnd file to run directly in your cloud native environment via the `cnd run SCRIPT` command. Each script must have a unique name.
```yaml
...
scripts:
  test: "python -m test"
  lint: "pylint app"
...
```

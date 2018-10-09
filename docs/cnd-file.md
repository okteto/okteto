# CND Yaml Reference

A Cloud Native Development file (`cnd.yml`) defines the container to be swapped in your dev environment by a container that hot reloads your local changes.

Below is an example of a `cnd.yml`:

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

Each key is documented below:

## name (required)

The name of the cloud native development. Must be unique across your set of cloud native environments.

## swap.deployment.file (required)

The path to the  manifest of the deployment to be replaced by the cloud native environment.

## swap.deployment.container (required)

The name of the container of to be replaced.

## swap.deployment.image (required)

The container image to be used by the cloud native environment.

## swap.deployment.command (optional)

The command to be executed by the cloud native environment.

It has to be a non-finishing command (default: `tail -f /dev/null`)

## swap.service.file (required)

The path to the manifest of the service to be replaced by the cloud native environment.

## mount.source (required)

The local folder synched to the remote container.

## mount.target (required)

The remote folder path synched with the local file system.

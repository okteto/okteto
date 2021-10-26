## How does Okteto work?

Okteto replaces your application pods by a development container. Let's explain this process with a diagram:

<img align="left" src="okteto-architecture.png">

When you run `okteto up`, okteto scales to zero the **api** deployment and creates a mirror development container **api-okteto**. This development container is a copy of the **api** deployment manifest with the following improvements:

- The **api-okteto** deployments can override any field defined in your [okteto manifest](https://okteto.com/docs/reference/manifest/) for development purposes. In particular, you might want to use a different container image with all the dev tools you need pre-installed.
- A bidirectional file [synchronization service](https://okteto.com/docs/reference/file-synchronization/) is started to keep your changes up to date between your local filesystem and your development container.
- Automatic local and remote port forwarding using [SSH](https://okteto.com/docs/reference/ssh-server/), so you can access your cluster services via `localhost` or connect a remote debugger.

Note that your development container inherits the original **api** manifest definition. Therefore, the development container uses the same service account, environment variables, secrets, volumes, sidecars, ... than the original **api** deployment, providing a fully integrated development environment.

Finally, okteto configures a watcher to stream any change to the **api** deployment definition into your **api-okteto** development container.

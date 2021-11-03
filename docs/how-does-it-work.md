## How does Okteto work?

Okteto replaces your application's container with a development container. Let's explain this process with a diagram:

<img align="left" src="okteto-architecture.png">

When you run `okteto up`, okteto scales to zero the **api** deployment and creates a mirror deployment **api-okteto**. This deployment is a copy of the **api** deployment manifest with the following development-time improvements:

- Okteto overrides the container-level configuration of the **api-okteto** deployment with the values defined in your [okteto manifest](https://okteto.com/docs/reference/manifest/). A typical example of this is to replace the production container image with one that contains your development runtime.
- A bidirectional file [synchronization service](https://okteto.com/docs/reference/file-synchronization/) is started to keep your changes up to date between your local filesystem and your development container.
- Automatic local and remote port forwarding using [SSH](https://okteto.com/docs/reference/ssh-server/). This allows you to do things like access your cluster services via `localhost` or connect a remote debugger.
- A watcher service to keep the definition of **api-okteto** up to date with **api**

It's worth noting that your development deployment inherits the original **api** manifest definition. Therefore, the development deployment uses the same service account, environment variables, secrets, volumes, sidecars, ... than the original **api** deployment, providing a fully integrated development environment.

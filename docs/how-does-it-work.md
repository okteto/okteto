## How does Okteto work?

Okteto replaces your application pods by a development container. Let's explain this process with a diagram:

<img align="left" src="okteto-architecture.png">


In the diagram, [okteto up](https://okteto.com/docs/reference/cli/#up) is executed against the **api** deployment. As a result, the  **api** deployment is scaled to zero, and a mirror deployment **api-okteto** is created. The **api-okteto** deployment manifest is a combination of the **api** deployment manifest and the overrides defined in your [okteto manifest](https://okteto.com/docs/reference/manifest/).

The okteto manifest overrides include things like:

- A different container image (with all the dev tools you need pre-installed).
- Environment variables needed for development.
- Remote paths where your local code is synchronized.
- Port forwards and reverse tunnels to access your application in localhost.
- And much more. Pretty much every deployment manifest field is overridable using the [okteto manifest](https://okteto.com/docs/reference/manifest/).

Note that the **api-okteto** deployment inherits the original **api** manifest definition: same service account, environment variables, secrets, volumes, sidecars, ... [okteto up](https://okteto.com/docs/reference/cli/#up) also configures a watcher to stream any change to the deployment **api** definition into your **api-okteto**  development container.

Finally, local code changes are immediately synchronized into your **api-okteto** development container via [syncthing](https://github.com/syncthing/syncthing). To accomplish this, Okteto launches syncthing both locally and in your **api-okteto**  development container. Both processes are securely connected via SSH tunnels.
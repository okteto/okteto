## How does Okteto work?

Okteto swaps your applications pods by a development container. Lets explain this process with a diagram:

<img align="left" src="okteto-architecture.png">

At a high level, Okteto works by swapping pods in your application with a development container. In the diagram above, you can see how Okteto replaced the **api** pods by the development container **api-dev**. 

The development container has a different container image (your development image, with all the tools you need pre-installed) but it keeps the rest of the configuration of the original pods (same identity, environment variables, start command, sidecars, etc…). Although you can override pretty much every configuration of the pod via the Okteto manifest.

Local code changes are automatically synchronized to the development container via [syncthing](https://github.com/syncthing/syncthing). To accomplish this, Okteto launches syncthing both locally and in the development container. Both processes are securely connected via Kubernetes' port forwarding capabilities. 

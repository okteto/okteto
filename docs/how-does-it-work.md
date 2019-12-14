## How does Okteto work?

Okteto swaps your applications pods by a development environment. Lets explain this process with a diagram:

<img align="left" src="okteto-architecture.png">

At a high level, Okteto works by swapping a pod in your application with a development environment. In the diagram above, you can see how Okteto replaced the **api** pod for the development environment **api-dev**. 

The development environment has a different container image (your development image, with all the tools you need pre-installed) but it keeps the rest of the configuration of the original pod (same identity, environment variables, start command, etc…). Although you can override pretty much every configuration of the pod via the Okteto yaml manifest.

Local code changes are automatically synchronized to the development environment via [syncthing](https://github.com/syncthing/syncthing). To accomplish this, Okteto launches syncthing both locally and in the development environment pod.  Both processes are securely connected via Kubernetes' port forwarding capabilities. 

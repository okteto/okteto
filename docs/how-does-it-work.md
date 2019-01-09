## How does CND work?

This is how a standard dev environment looks like:

<img align="left" src="env.png">

&nbsp;

And this how it looks after converting it into a cloud native environment:

<img align="left" src="cnd.png">
&nbsp;

The **cnd** container duplicates the manifest of the **api** pod, so it is fully integrated with every Kubernetes feature.
For example, the **cnd** container has access to the same envvars, secrets, volumes, ...

Local changes are synched to the **cnd** container via [syncthing](https://github.com/syncthing/syncthing). As you save locally, it will be automatically synched in your **cnd** container in seconds. To this end, `cnd` injects a new **syncthing** container into the original **api** pod.

This container is exposed locally using *port-forwarding* and it is connected by a local *syncthing* process responsible of sending local changes to the remote container. This way, the original **api** container is not polluted with syncthing dependencies.

The **syncthing** and the **cnd** containers share a common volume where local changes are synched, making them available to the **cnd** container in a container local path.

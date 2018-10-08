/*
Package cluster provides tools to interact with the k8s cluster. These include:

- A k8s client to the remote api server.
- The ksync daemonset definition and a way to launch it on the cluster.
- Tunnels from the remote pod to the localhost.
- Connections between each remote container and the localhost.
*/
package cluster

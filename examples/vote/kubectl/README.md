# Voting App deployed with raw kubernetes manifests

This repository contains a Cloud Native Development ([CND](https://github.com/okteto/cnd)) demo for the well known Docker [Voting App](https://github.com/dockersamples/example-voting-app) deployed using raw kubernetes manifests.

The demo works in any kubernetes cluster. Cloud Native Development provides more value in remote kubernetes clusters, but in order to make it simple to follow this guide, we recommend to use Docker for Mac (with Kubernetes support) or [minikube](https://github.com/kubernetes/minikube).

## Deploy the Voting App

Clone this repo and move to this example folder.

```console
git clone https://github.com/okteto/cnd
cd cnd/examples/vote/kubectl
```

run the Voting App by executing:

```console
kubectl apply -f manifests
```

Wait for one or two minutes until the application is running.

If you are running in minikube, the Voting App is available on port 31000 in the minikube ip (`minikube ip`).

## Develop as a Cloud Native Developer

In order to activate your Cloud Native Development, execute:

```console
cnd up
```

this will create a remote container which is synchronized with your local code changes and hot reloads these changes without rebuilding containers (eliminating the **docker build/push/pull/redeploy** cycle).

Edit the file `vote/app.py` and change the `option_a` in line 8 from "Cats" to "Otters". Save your changes.

Finally, refresh the Voting App UI, and cool! your code changes are live!

*review [cnd's usage](https://github.com/okteto/cnd/docs/cli-reference.md) guide to see other commands available to help you speed you up your development.*

## Cleanup

Cancel the `cnd up` command by pressing `ctrl + c` and run the following command to deactivate the cloud native environment:

```console
cnd down
``` 

Run the following command to remove the resources created by this guide: 

```console
kubectl delete -f manifests
```




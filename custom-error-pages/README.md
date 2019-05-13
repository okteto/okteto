# custom-error-pages

These pages are displayed when a user's service returns an error. The most typical one is 502, when the environment is up but the service is not started, or is started in the wrong port. 

To configure this:
- build the docker image for okteto/custom-error-pages:$TAG
- deploy  the service and deployment in the same namespace as the ingress
- update the nginx-ingress configuration to use this image as the default backend.

More information [available here](https://kubernetes.github.io/ingress-nginx/user-guide/custom-errors/)
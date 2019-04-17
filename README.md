# Okteto Cloud

[![CircleCI](https://circleci.com/gh/okteto/app.svg?style=svg)](https://circleci.com/gh/okteto/app)

## Setup
In production, we use tiller with SSL. You'll need to copy the tiller certificates from the keybase to your helm folder for it to work 

        cp /keybase/team/riberaproject/private/okteto-cloud/*.pem ~/.helm

Make sure you have helm 2.13+

        helm version

The Makefile requires docker, and for you to be signed to the docker registries in our GCP account (run `gcloud auth configure-docker`). it also requires keybase to be installed and available on `/keyabase`

## Upgrade
1. Select the production context 
        
        gke_okteto-prod_us-central1_production/okteto

1. Export the tag
        
        export TAG=0.1.4

1. Build the docker images
        
        make prod

1. Push the docker images 
        
        make push-prod

1. Update `/keybase/team/riberaproject/private/okteto-cloud/override-prod.yaml` as necessary.

1. Upgrade prod
        
        make upgrade-prod

1. If CLI upgrade is needed
        
        make upgrade-prod-cli

1. Send a PR with the changed files, and merge.
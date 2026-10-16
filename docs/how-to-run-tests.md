# How to run tests guidelines

In this document we will cover how to run unit tests and e2e tests locally

- [How to run tests guidelines](#how-to-run-tests-guidelines)
  - [How to run unit tests locally?](#how-to-run-unit-tests-locally)
    - [Requirements:](#requirements)
    - [Run all unit tests](#run-all-unit-tests)
    - [Run package tests](#run-package-tests)
    - [Run specific test](#run-specific-test)
  - [How to run e2e tests locally?](#how-to-run-e2e-tests-locally)
    - [Requirements:](#requirements-1)
    - [Run all e2e tests](#run-all-e2e-tests)
    - [Run specific e2e tests](#run-specific-e2e-tests)
  - [How to run pre-commit on your local branch](#how-to-run-pre-commit-on-your-local-branch)
    - [Install pre-commit](#install-pre-commit)
    - [How to run pre-commit locally](#how-to-run-pre-commit-locally)
  - [How to run e2e tests from a PR?](#how-to-run-e2e-tests-from-a-pr)

## How to run unit tests locally?

Unit tests will run against the code you have on your workspace

### Requirements:

You don't need to have any special prerequisite to run unit tests locally.

### Run all unit tests

You can run all tests by running the following command:

```bash
make test
```

### Run package tests

You can run the tests of a package by running the following command:

```bash
go test packageName
# for example
go test github.com/okteto/okteto/cmd/deploy
```

### Run specific test

You can run a specific test by running the following command:

```bash
go test -run testRegex packageName
# for example
go test -run '^(TestDeployWithErrorReadingManifestFile)$' github.com/okteto/okteto/cmd/deploy
```

## How to run e2e tests locally?

E2E tests will run against an okteto cluster that you must be logged in

### Requirements:

You will need to set some environment variables to start running e2e tests

- `OKTETO_URL`: This is the url of your okteto cluster. For example: `https://product.okteto.dev`
- `OKTETO_TOKEN`: The token of your okteto user. You can get it from the okteto UI.
- `OKTETO_APPS_SUBDOMAIN`: The subdomain of the okteto cluster. For example: `product.okteto.dev`
- `OKTETO_NAMESPACE_PREFIX`: The prefix for namespaces created in `OKTETO_URL` instance.
- `OKTETO_PATH`: The path of the okteto binary (It will default to `/usr/local/bin/okteto`).

### Run all e2e tests

You can run all tests by running the following command:

```bash
make integration
```

### Run specific e2e tests

There are different e2e tests that can be run individually:

- Run actions: Run all e2e tests for actions

``` bash
    make integration-actions # which is equivalent to run go test github.com/okteto/okteto/integration/actions -tags="actions" --count=1 -v -timeout 15m
```

- Run build: Run all e2e tests that build

``` bash
    make integration-build # which is equivalent to run go test github.com/okteto/okteto/integration/build -tags="integration" --count=1 -v -timeout 10m
```

- Run deploy: Run all e2e tests that deploy

``` bash
    make integration-deploy # which is equivalent to run go test github.com/okteto/okteto/integration/deploy -tags="integration" --count=1 -v -timeout 30m
```

- Run okteto: Run all e2e tests that are only valid on okteto clusters

``` bash
    make integration-okteto # which is equivalent to run go test github.com/okteto/okteto/integration/okteto -tags="integration" --count=1 -v -timeout 30m
```

- Run up: Run all e2e tests that use okteto up as main command

``` bash
    make integration-up # which is equivalent to run go test github.com/okteto/okteto/integration/up -tags="integration" --count=1 -v -timeout 45m
```

## How to run pre-commit on your local branch

Okteto CLI uses [pre-commit](https://pre-commit.com) to detect misspellings, yaml and markdown errors, merge conflicts and private keys.

### Install pre-commit

Using pip:

```bash
pip install pre-commit
```

Using brew:

```bash
brew install pre-commit
```

### How to run pre-commit locally

You'll need to set your working directory on the root of your Okteto CLI project and run:

```bash
pre-commit run --all-files
```

## How to run e2e tests from a PR?

Use the following labels to trigger integration tests on this branch:

- `run-e2e`: triggers the workflow for unix and windows
- `run-e2e-unix`: triggers the workflow for unix
- `run-e2e-windows`: triggers the workflow just for windows

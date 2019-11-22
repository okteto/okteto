# Contributions

Interested in contributing? As an open source project, we'd appreciate any help and contributions! 

We follow the standard [github pull request process](https://help.github.com/articles/about-pull-requests/). We'll try to review your contributions as soon as possible. 

# Development

Okteto is developed using the [Go](https://golang.org/) programming language. The current version of Go being used is [v1.13](https://golang.org/doc/go1.13). 

To start working on Okteto, simply fork this repository, clone the okteto repository locally, and run the following command at the root of the project:

```
go build
```

This should create the `okteto` binary. You can execute the binary by running the following:

```
./okteto
```

After you make changes, simply run `go build` again to recomplile your changes.

Unit tests for the project can be executed by running:

```
make test
```

## File an Issue
Not ready to contribute code, but see something that needs work? While we encourage everyone to contribute code, it is also appreciated when someone reports an issue. We use [github issues](https://github.com/okteto/okteto/issues) for this.
Also, check our [troubleshooting section](docs/troubleshooting.md) for known issues.

## Code of Conduct
Please make sure to read and observe our [code of conduct](code-of-conduct.md).

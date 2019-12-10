# Contributions

Interested in contributing? As an open source project, we'd appreciate any help and contributions! 

We follow the standard [github pull request process](https://help.github.com/articles/about-pull-requests/). We'll try to review your contributions as soon as possible. 

## File an Issue
Not ready to contribute code, but see something that needs work? While we encourage everyone to contribute code, it is also appreciated when someone reports an issue. We use [github issues](https://github.com/okteto/okteto/issues) for this.

## Report security issues

If you want to report a sensitive security issue or a security exploit, you can directly contact the project maintainers via [Twitter DM](https://twitter.com/oktetoHQ) or via hello@okteto.com.

## Pull Requests

When submitting a pull request, please make sure that it adheres to the following standard:

1. Commits are signed-off (`git commit --signoff or -s`).
1. It includes the whole template for issues and pull requests.
1. It [references addressed issues](https://help.github.com/en/github/managing-your-work-on-github/closing-issues-using-keywords) in the PR description & commit messages.
1. It includes pertinent unit tests.
1. It has clear commit messages.

## Code of Conduct
Please make sure to read and observe our [code of conduct](code-of-conduct.md).

# Development Guide

Okteto is developed using the [Go](https://golang.org/) programming language. The current version of Go being used is [v1.13](https://golang.org/doc/go1.13). It uses go modules for dependency management. 

To start working on Okteto, simply fork this repository, clone the okteto repository locally, and run the following command at the root of the project:

```
make
```

This will create the `okteto` binary in the `bin` folder. You can execute the binary by running the following:

```
bin/okteto
```

After you make changes, simply run `make` again to recomplile your changes.

Unit tests for the project can be executed by running:

```
make test
```
This command will run all the unit tests, will try to detect race conditions, and will generate a test coverage report.
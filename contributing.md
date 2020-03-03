# Contributions

Interested in contributing? As an open source project, we'd appreciate any help and contributions! 

We follow the standard [github pull request process](https://help.github.com/articles/about-pull-requests/). We'll try to review your contributions as soon as possible. 

## File an Issue
Not ready to contribute code, but see something that needs work? While we encourage everyone to contribute code, it is also appreciated when someone reports an issue. We use [github issues](https://github.com/okteto/okteto/issues) for this.

## Report security issues

If you want to report a sensitive security issue or a security exploit, you can directly contact the project maintainers via [Twitter DM](https://twitter.com/oktetoHQ) or via hello@okteto.com.

## Pull Requests

When submitting a pull request, please make sure that it adheres to the following standard.

1. Code should be go fmt compliant.
1. Public types, structs and funcs should be documented.
1. It includes pertinent unit tests.
1. Commits are signed-off (`git commit --signoff or -s`).
1. It includes the whole template for issues and pull requests.
1. It [references addressed issues](https://help.github.com/en/github/managing-your-work-on-github/closing-issues-using-keywords) in the PR description & commit messages.
1. It has clear commit messages.

## Sign your work

The sign-off is a simple line at the end of the explanation for a patch. Your signature certifies that you wrote the patch or otherwise have the right to pass it on as an open-source patch. The rules are pretty simple: if you can certify the below (from [developercertificate.org](https://developercertificate.org)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

```
Signed-off-by: Cindy Lopez <cindy.lopez@okteto.com>
```

If you set your `user.name` and `user.email` git configs, you can sign your commit automatically with `git commit -s`.

## Code of Conduct
Please make sure to read and observe our [code of conduct](code-of-conduct.md).

# Development Guide

Okteto is developed using the [Go](https://golang.org/) programming language. The current version of Go being used is [v1.14](https://golang.org/doc/go1.14). It uses go modules for dependency management. 

## Build 
To start working on Okteto, simply fork this repository, clone the okteto repository locally, and run the following command at the root of the project:

```
make
```

This will create the `okteto` binary in the `bin` folder. You can execute the binary by running the following:

```
bin/okteto
```

After you make changes, simply run `make` again to recompile your changes.

## Test 

Unit tests for the project can be executed by running:

```
make test
```

This command will run all the unit tests, will try to detect race conditions, and will generate a test coverage report.

Integration tests can be executed by running:

```
make integration
```

These tests will use your Kubernetes context to create a namespace and all the required k8s resources.

## Lint

Before submitting your changes, we recommend to lint the code by running:

```
make lint
```

The same command runs as part of CI on every PR.

> This command requires that you have [golangci-lint](https://github.com/golangci/golangci-lint#install) available on your `$PATH`.
# Contributing to Okteto CLI

Thank you for showing interest in contributing to Okteto CLI! We appreciate all kinds of contributions, suggestions, and feedback.

## Code of Conduct

This project adheres to the Contributor Covenant [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report any unacceptable behavior to hello@okteto.com.

## Ways To Contribute

### Reporting Issues

Reporting issues is a great way to help the project! This isn't just limited to reporting bugs but can also include feature requests or suggestions for improvements in current behavior. We use [GitHub issues](https://github.com/okteto/okteto/issues) for tracking all such things. But if you want to report a sensitive security issue or a security exploit, you can directly contact the project maintainers on hello@okteto.com or via [a Twitter DM](https://twitter.com/oktetoHQ).

### Contributing Code

When contributing features or bug fixes to Okteto CLI, it'll be helpful to keep the following things in mind:

- Communicating your changes before you start working
- Including unit tests whenever relevant
- Making sure your code passes the [lint checks](#lint)
- Signing off on all your git commits by running `git commit -s`
- Documenting all Go public types, structs, and functions in your code

Discussing your changes with the maintainers before implementation is one of the most important steps, as this sets you in the right direction before you begin. The best way to communicate this is through a detailed GitHub issue. Another way to discuss changes with maintainers is using the [#okteto](https://kubernetes.slack.com/messages/CM1QMQGS0/) channel on the Kubernetes slack.

#### Making a Pull Request

The following steps will walk you through the process of opening your first pull request:

##### Fork the Repository

Head over to the project repository on GitHub and click the **"Fork"** button. This allows you to work on your own copy of the project without being affected by the changes on the main repository. Once you've forked the project, clone it using:

```
git clone https://github.com/YOUR-USERNAME/okteto.git
```

##### Create a Branch

Creating a new branch for each feature/bugfix on your project fork is recommended. You can do this using:

```
git checkout -b <branch-name>
```

##### Commit and Push Your Changes

Once you've made your changes, you can stage them using:

```
git add .
```

After that, you'll need to commit them. For contributors to certify that they wrote or otherwise have the right to submit the code they are contributing to the project, we require them to acknowledge this by signing their work, which indicates they agree to the DCO found [here](https://developercertificate.org/).

To sign your work, just add a line like this at the end of your commit message:

```
Signed-off-by: Cindy Lopez <cindy.lopez@okteto.com>
```

This can easily be done with the `-s' command-line option to append this automatically to your commit message.

```
git commit -s -m 'Meaningful commit message'
```

> In order to use the `-s` flag for auto signing the commits, you'll need to set your `user.name`and`user.email` git configs

Finally, you can push your changes to GitHub using:

```
git push origin <branch-name>
```

Once you do that and visit the repository, you should see a button on the GitHub UI prompting you to make a PR.

## Development Guide

Okteto is developed using the [Go](https://golang.org/) programming language. The current version of Go being used is [v1.17](https://golang.org/doc/go1.17). It uses go modules for dependency management.

### Building

Once you've made your changes, you might want to build a binary of the Okteto CLI containing your changes to test them out. This can be done by running the following command at the root of the project:

```
make
```

This will create the `okteto` binary in the `bin` folder. You can execute the binary by running the following:

```
bin/okteto
```

After you make more changes, simply run `make` again to recompile your changes.

### Testing

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

### Linting

Before making a PR, we recommend contributors to run a lint check on their code by running:

```
make lint
```

The same command also runs as part of CI on every PR.

> This command requires that you have [golangci-lint](https://github.com/golangci/golangci-lint#install) available on your `$PATH`.

We also use `pre-commit` to lint our Markdown and YAML files using the following linters:

- [markdowlint-cli](https://github.com/igorshubovych/markdownlint-cli)
- [yamllint](https://yamllint.readthedocs.io/en/stable/index.html)

### pre-commit

A framework for managing and maintaining multi-language pre-commit hooks.
Pre-commit can be [installed](https://pre-commit.com/#installation) with
`pip`, `curl`, `brew` or `conda`.

You need to first install pre-commit and then install the pre-commit hooks
with `pre-commit install`. Now pre-commit will run automatically on git
commit!

It's usually a good idea to run the hooks against all the files when
adding new hooks (usually pre-commit will only run on the changed files
during git hooks). Use `pre-commit run --all-files` to check all files.

To run a single hook use `pre-commit run --all-files <hook_id>`

To update use `pre-commit autoupdate`

- [Quick start](https://pre-commit.com/#quick-start)
- [Usage](https://pre-commit.com/#usage)
- [pre-commit-autoupdate](https://pre-commit.com/#pre-commit-autoupdate)

### Spell checking

We are running [misspell](https://github.com/client9/misspell) to check for spelling errors using [GitHub Actions](.github/workflows/lint.yml). You can run this locally against all files using `misspell .` after [grabbing](https://github.com/client9/misspell#install) the `misspell` binary.

Some useful `misspell` flags:

- `-i` string: ignore the following corrections, comma-separated
- `-w`: Overwrite file with corrections (default is just to display)

We also run [codespell](https://github.com/codespell-project/codespell) to check spellings against a [small custom dictionary](codespell.txt).

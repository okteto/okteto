# General code style guidelines and trends of the project.

## Avoid augmenting `model` package. Instead, create a package structure according to the domain.

See example [here](https://github.com/okteto/okteto/tree/master/pkg/externalresource).

## Mock filesystem calls

The CLI has a lot of `os.XXX` calls that should be testable. This can be achieved either by:

- using libraries such as [afero](https://github.com/spf13/afero). We could initialize it at the beginning of the main.go and pass it through the commands and functions to be able to mock it without having to create tempdir/tempfiles

- defining an interface for the filesystem calls used and mock it in the tests

## Remove global variables

Global variables such as `okteto.ContextStore` or `Logger` should be instantiated at the `main.go` level and passed as struct parameters through the commands.

We should also remove all `okteto.Context().XXX` calls since they also refer to the global context.

## Create commands structs

Creating command structs allows us to get rid of the global variables and mock some external dependency calls with dependency injection. This will also allow us to have a cleaner and more organized code.

## Refactoring the build command

We should slowly be refactoring the build command, decreasing the structural complexity of the command. Currently its logic is distributed by several packages such as `cmd/build`, `pkg/cmd/build` and `pkg/model/build`.

## Segregate immutable inputs from other mutable values

When adding new commands or refactor existing ones differentiate between the flags supported by the command and specified by the user vs the options internally used by the command. Avoid as much as possible to modify the values specified by the user

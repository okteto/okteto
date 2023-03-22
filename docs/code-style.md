# General code style guidelines and trends of the project.

- Avoid augmenting `model` package. Instead, create a package structure according to the domain. See example [here](https://github.com/okteto/okteto/tree/master/pkg/externalresource).
- [Mock filesystem calls](https://www.notion.so/Go-Chapter-01c694f5f149425280d14431840dd715)
- [Remove global variables](https://www.notion.so/Remove-global-variables-a08141e572c34830a6d892d8db9634c4)
- [Create commands structs](https://www.notion.so/Create-commands-structs-da8f8c79ae324cfda74be9789a19e1bf)
- Refactoring the build command decreasing the structural complexity of the command. Currently its logic is distributed by several packages such as `cmd/build`, `pkg/cmd/build` and `pkg/model/build`.
- When adding new commands or refactor existing ones differentiate between the flags supported by the command and specified by the user vs the options internally used by the command. Avoid as much as possible to modify the values specified by the user

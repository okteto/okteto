# Okteto CLI code review comments

- [Purpose of code reviews](#purpose-of-code-reviews)
- [Agreements](#agreements)
- [Expectations](#expectations)
  - [As an Author](#as-an-author)
  - [As a Reviewer](#as-a-reviewer)
- [What to look for in a code review?](#what-to-look-for-in-a-code-review)
  - [Code review checklist](#code-review-checklist)
  - [PR Description](#pr-description)
  - [PR comments](#pr-comments)
  - [PR scope](#pr-scope)
  - [PR Merge](#pr-merge)
  - [Tests](#tests)
    - [Unit tests](#unit-tests)
    - [E2E tests](#e2e-tests)
  - [How to know if my PR affects other services?](#how-to-know-if-my-pr-affects-other-services)
  - [Logs](#logs)
  - [New dependencies](#new-dependencies)
  - [Analytics](#analytics)
- [Common anti-patterns](#common-anti-patterns)
  - [Errors](#errors)
    - [Don't Panic](#dont-panic)
    - [Discarded errors](#discarded-errors)
    - [Adding context](#adding-context)
    - [Indent Error Flow](#indent-error-flow)
  - [Naming convention](#naming-convention)
    - [Variable Names](#variable-names)
    - [Receiver's name](#receivers-name)
    - [Be consistent](#be-consistent)
    - [Search for an element](#search-for-an-element)

## Purpose of code reviews

We believe our code review process is foundational in ensuring that:

- Potential Bugs are Identified: Each review acts as an added layer of validation for the implementation
- Knowledge Sharing: It offers an opportunity for collective learning and improvement in coding practices
- Enhanced Quality: Reviews ensure that our output is of the highest quality
- Broader Feedback: Opening changes for review allows a wider set of contributors to provide their insights
- Documentation & Traceability: Reviews create a historical record

## Agreements

- **Clear Actionable Feedback**: We provide unambiguous, actionable feedback in our comments. "Nice-to-have" suggestions should lead to new issues instead of blocking PRs.
- **Reviewers**: PRs typically require 2 approvals for merging. Use discretion: some might need more approvers, depending on the scope.
- **PR Scope**: Keep PRs concise and focused.

## Expectations

### As an Author:

- **Clarity & Context**: Ensure PRs have a clear title, description, and details on any alternative approaches considered.
- description.
- **Drafts**: Use Draft status to signal ongoing work.
- **Self-review**: Inspect your PR before opening it for team review.
- **Testing**: Provide detailed testing steps.
- **Critical** Paths: Notify if crucial parts of the system are affected, urging extra testing.
- **Scope**: Keep PRs focused and narrow.
- **Tests**: Always include unit and integration tests for changes.

### As a Reviewer:

- **Seek Clarification**: Don’t hesitate to ask if something is unclear.
- **Hands-on Testing**: Checkout the branch, build the CLI, and test as instructed.
- **User Experience**: Review with a focus on the end-user experience, like spotting opportunities for clearer error messages.
- **Guard the Codebase**: Be vigilant for potential edge cases or impacts from the change.
- **Testing Insights**: Point out untested scenarios or overlooked cases (e.g., potential issues on Windows).
- **Scope Vigilance**: Ensure the PR's scope is relevant and isn’t overly broad or risky.

## What to look for in a code review?

### Code review checklist

Here is a summary of the things every reviewer should be aware of while doing a CLI code review:

- [ ] Does the PR have unit tests/e2e tests that cover all the scenarios?
  *We should look forward to having all scenarios covered.*
- [ ] Does the PR description explain what it does/solves?
  *In the future, we may need to know why the PR was created and what issues it solved, so this should be made clear in the description of the PR.*
- [ ] Does it affect other services (actions/vscode plugin/pipeline/graphql/json logs)?
  *It can affect other services that are not the CLI itself and break scenarios that are not contemplated on the CLI repository, so we need to bear in mind all those services when reviewing a PR*
- [ ] Does it need to add analytics?
  *We need to add analytics to know the adoption of new features and how the users use the product*
- [ ] Does the code have any code smells?
  we should develop clean code so that if a new developer starts working on the project, he/she can understand the code as soon as possible*

### PR Description

The description of the pull request should state what issue it solves (linking the issue on the PR) and how it is solved.

If required, the author could be asked to provide screenshots or videos explaining what was wrong and how it was fixed.

### PR comments

Comments are the basis of all pull requests. In comments, the reviewers will ask questions about the code, request changes, and discuss what is the best approach to improve the CLI code.

A respectful tone will be used when making comments and supporting documentation can be helpful for certain comments.

We prefer giving unambiguous, actionable feedback in our comments. "Nice-to-have" suggestions should lead to new issues instead of blocking PRs, unless strictly required.

### PR scope

The logic introduced/modified in the pull request should be just enough to close the issue with the minimum acceptable quality. If during the implementation of the PR the developer encounters new issues not directly related to the current one, a new issue should be created. In this way, developers are totally focused on solving the issue they are working on, we avoid possible confusion to the reviewers (with less context than the developer) and we can locate more clearly possible bugs introduced.

This approach can be summarized by the term **Minimum Viable Change (MVC)**. MVC means that each issue and pull request should contain the minimum amount of change/scope possible that is needed to address, in accordance to internal quality standards, the core need that was originally identified. Other needs, even if related to the original one, should be addressed in different issues and pull requests.

If the developer decides to open a new issue from the current one or is not completely sure whether it is necessary to create a new issue or not, this must be commented in the original issue/PR.

### PR Merge

#### **If the author is a maintainer**

Once the PR is approved and the checks are passing, the author will be responsible for merging the PR or delegating the responsibility to another developer. There could be exceptions to this rule, for example, if the author is on vacation and the merge is "urgent", other developers can merge the PR to unblock the situation.

#### **If the author is not a maintainer**

Any maintainer can merge the PR when:

- The PR has approvals from at least two maintainers to ensure that the changes are aligned with the project's goals and quality standards.
- All requests from the maintainers have been addressed, and those maintainers have approved the PR.

### Tests

All new features or bug fixes must have unit tests that cover the added code and the functionality it provides. When adding new features it is highly recommended that integration tests are also created and may be deemed required by a reviewer.

Tests should fail with helpful messages saying what was wrong, what inputs were provided, the expected result, and the actual result received in testing. Assume that the person debugging your failing test is not you, and is potentially new to the entire project.

To run unit/e2e tests locally, please check our [How to run tests guidelines](how-to-run-tests.md)

#### Unit tests

The code should be testable by unit functions, which means that each function should perform an action and this action should be testable.

When creating unit tests, try to use table-driven tests when the same result is expected (the function must return an error) for different inputs. Table-driven tests allow us to create a boilerplate in which we define the name of the test (explanation of what it does), the test inputs and the output to be returned. For example:

```golang
func Test_FunctionNameWithError(t *testing.T) {
    var tests = []struct {
        name         string
        resource     Resource
    }{
        {
            name:        "when resource is nil then error",
            resource:    nil,
        },
        {
            name:        "when resource is not valid then error",
            resource:    MalformedResource{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Setenv("ENV_REQUIRED_FOR_TEST", "testValue")
            testDir := t.TempDir()
            err := functionName(testDir)
            assert.Error(t, err)
        })
    }
}

func Test_FunctionNameThenNoError(t *testing.T) {
    t.Setenv("ENV_REQUIRED_FOR_TEST", "testValue")
    testDir := t.TempDir()
    err := functionName(MalformedResource{})
    assert.NoError(t, err)
}
```

If the test has many use cases, it will be grouped into several tests per result, for example:

```golang
func Test_FunctionNameThenErr(t *testing.T) {
    var tests = []struct {
        name         string
        resource     Resource
        expectedErr  bool
    }{
        {
            name:        "when resource is nil then error",
            resource:    nil,
        },
        {
            name:        "when resource is not valid then error",
            resource:    MalformedResource{},
        },
        {
            name:        "when resource is not valid because of X",
            resource:    MalformedResource{},
        },
        {
            name:        "when resource fails because of proxy",
            resource:    MalformedResource{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Setenv("ENV_REQUIRED_FOR_TEST", "testValue")
            testDir := t.TempDir()
            err := functionName(testDir)
            assert.Error(t, err)
        })
    }
}
func Test_FunctionNameThenNoErr(t *testing.T) {
    var tests = []struct {
        name         string
        resource     Resource
    }{
        {
            name:        "when resource is type 1",
            resource:    CorrectResource1{},
        },
        {
            name:        "when resource is type 1 and has x feature",
            resource:    CorrectResource1{
                FeatureX: true,
            },
        },
        {
            name:        "when resource is type 2",
            resource:    CorrectResource2{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Setenv("ENV_REQUIRED_FOR_TEST", "testValue")
            testDir := t.TempDir()
            err := functionName(testDir)
            assert.NoError(t, err)
        })
    }
}
```

Each test should have the capability of running in isolation, eliminating and leaving the state of the machine in the same state as it was at the start of the test.

#### E2E tests

Each command should have its own set of end-to-end tests to prove that the main functionality works correctly. An e2e test should be added to a new feature if it adds a new use case or breaks an existing use case.

In order to speed up the CI process, all tests are executed in parallel including any new tests

### How to know if my PR affects other services?

- `deploy/destroy` command: This command has the most dependencies, as it is used by all other services:

  - The information stored on the `configmap` is used on the UI to show information like structured logs, and repository information or to execute actions like redeploying or destroying the application.
  - The logs that are written in the buffer(`JSON`) will be the ones stored in the `configmap`, to later show them in the UI.
  - The following actions directly or indirectly use this command: [preview deploy](https://github.com/okteto/deploy-preview)/[pipeline deploy](https://github.com/okteto/pipeline)/[destroy pipeline](https://github.com/okteto/destroy-pipeline)/[preview destroy](https://github.com/okteto/destroy-preview)
  - Changes on this command can affect other commands like `preview` or `pipeline`
- `up/down` command: It's mostly used on the `vscode` and `docker extension`
- `stack` command: This command is used by `okteto deploy` command.
- Changes on `graphql` calls: You have to check that your changes should work also with the previous `okteto chart` version.

### Logs

Logs are an important part of the CLI because if a user has a problem, we will have to review the file generated by `okteto doctor`, so these logs can be used as a tool to help us identify the problem.

You should be aware that there are different types of logs (messages that are returned to the user or saved in the log file).

The functions that return a message to the user are:

```golang
oktetoLog.Error(args ...interface{})
oktetoLog.Errorf(format string, args ...interface{})
oktetoLog.Fail(format string, args ...interface{})
oktetoLog.Fatalf(format string, args ...interface{})
oktetoLog.Yellow(format string, args ...interface{})
oktetoLog.Green(format string, args ...interface{})
oktetoLog.Success(format string, args ...interface{}) //
oktetoLog.Information(format string, args ...interface{})
oktetoLog.Question(format string, args ...interface{}) error
oktetoLog.Warning(format string, args ...interface{})
oktetoLog.FWarning(w io.Writer, format string, args ...interface{})
oktetoLog.Hint(format string, args ...interface{})
oktetoLog.Println(args ...interface{})
oktetoLog.FPrintln(w io.Writer, args ...interface{})
oktetoLog.Print(args ...interface{})
oktetoLog.Fprintf(w io.Writer, format string, a ...interface{})
oktetoLog.Printf(format string, a ...interface{})
```

The functions that store logs in the log file are:

```golang
oktetoLog.Debug(args ...interface{})
oktetoLog.Debugf(format string, args ...interface{})
oktetoLog.Info(args ...interface{})
oktetoLog.Infof(format string, args ...interface{})
```

Through these, you should be able to follow the execution of the okteto command without problems, so they should be as concise as possible. Errors not returned to the user must be logged, without exception, as this is the only trace we may have when debugging an issue.

### New dependencies

It is ok to add new dependencies, as long as we know the reason why they are added and the implications of adding a dependency ( when they update it, check if it has any vulnerability). So if a PR imports a new dependency, we need to agree that is necessary to add it.

### Analytics

With each release we fix bugs, and release new features and the only way to see how all these changes are adopted is by analyzing the data collected by the users, so we should collect as much useful data as possible to see how many people use new features if the percentage of bugs per user and command is decreasing.

Usually, new properties will be added to existing events, but if there is new functionality and we need to track a new event, for example, this is how new command analytics could look like:

```json
{
    "execution time": "5m",
    "error": nil,
    "use-of-X-flag": true,
    "use-of-Y-flag": false,
}
```

## Common anti-patterns

### Errors

Handling errors. Make sure to always follow Go error handling [best practices](http://golang.org/doc/effective_go.html#errors)

#### Don't Panic

Don't use panic for normal error handling. Use error and multiple return values.

#### Discarded errors

Do not discard errors using `_` variables. If a function returns an error, check it to make sure the function succeeded. Handle the error, return it, or, in truly exceptional situations, log it as soon as it happens.

#### Adding context

Adding context before you return the error can be helpful, instead of just returning the error. This allows developers to understand what the program was trying to do when it entered the error state making it much easier to debug.
For example:

```golang
// Wrap the error
return nil, fmt.Errorf("get cache %s: %w", f.Name, err)

// Just add context
return nil, fmt.Errorf("saving cache %s: %s", f.Name, err)
```

A few things to keep in mind when adding context:

- Decide if you want to expose the underlying error to the caller. If so, use %w, if not, you can use %v.
- Don’t use words like *failed, error, or didn't*. As it’s an error, the user already knows that something failed and this might lead to having strings like *failed xx*. Explain what failed instead.
- Error strings should not be capitalized or end with punctuation or a newline. You can use `golint` to check for this.

#### Indent Error Flow

Try to keep the normal code path at a minimal indentation, and indent the error handling, dealing with it first. This improves the readability of the code by permitting visual scanning of the normal path. For instance, don't write:

```golang
if err != nil {
    // error handling
} else {
    // normal code
}

```

Instead, write:

```golang
if err != nil {
    // error handling
    return // or continue, etc.
}
// normal code
```

If the if statement has an initialization statement, such as:

```golang
if x, err := f(); err != nil {
    // error handling
    return
} else {
    // use x
}
```

then this may require moving the short variable declaration to its own line:

```golang
x, err := f()
if err != nil {
    // error handling
    return
}
// use x
```

### Naming convention

#### Variable Names

Variable names in Go should be short rather than long. This is especially true for local variables with limited scope. Prefer `c` to `lineCount`. Prefer `i`to`sliceIndex`.

The basic rule: the further from its declaration that a name is used, the more descriptive the name must be. For a method receiver, one or two letters are sufficient. Common variables such as loop indices and readers can be a single letter (`i`, `r`). More unusual things and global variables need more descriptive names.

#### Receiver's name

The name of a method's receiver should be a reflection of its identity; often a one or two-letter abbreviation of its type suffices (such as `c` or `cl` for `Client`). Don't use generic names such as "me", "this" or "self", identifiers typical of object-oriented languages that place more emphasis on methods as opposed to functions. The name need not be as descriptive as that of a method argument, as its role is obvious and serves no documentary purpose. It can be very short as it will appear on almost every line of every method of the type; familiarity admits brevity.

#### Be consistent

If you call the receiver `c` in one method, don't call it `cl` in another.

#### Search for an element

If it is necessary to search several times if an element is found in another list, it is best to create a map where the key is something that identifies the element. For example:

```golang
portsToAdd := []int{8080, 5005, 3000, 8081, 9001, 9000}
newPortsComingFromDockerfile := []int{3000, 1234}

// Create map with all the ports:
alreadyAddedPorts := map[int]bool{}
for _, p := range portsToAdd{
    alreadyAddedPorts[p] = true
}

for _, p := range newPortsComingFromDockerfile{
    // Check if it's already added
    if _, ok := alreadyAddedPorts[p]; ok {
        // port is already added
    } else {
        // port is missing in the list
    }
}
```

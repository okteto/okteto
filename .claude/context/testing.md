# Testing

## Test Organization

### Unit Tests

- Located alongside source: `*_test.go` in same directory as `*.go`
- Use standard `testing` package + `github.com/stretchr/testify/require`
- Run via `make test` or `okteto test` (includes race detection and coverage)
- No external dependencies (use mocks/fakes)

### Integration Tests

- Located in `integration/` directory
- Organized by command: `integration/build/`, `integration/deploy/`, `integration/up/`, etc.
- Require build tag: `//go:build integration` at top of file
- Run via `make integration` or specific target (e.g., `make integration-deploy`)
- Require live Kubernetes cluster and environment variables
- Use `integration.GetOktetoPath()` to locate built binary
- Execute CLI commands via `commands.Run*()` helpers

## Testing Framework

### Required Package

Use `github.com/stretchr/testify/require` (NOT `assert`):

```go
require.NoError(t, err)
require.Equal(t, expected, actual)
require.True(t, condition)
```

### Why `require` over `assert`

- `require` stops test execution on failure
- `assert` continues after failure (can cause cascading errors)

## Test Structure

### Table-Driven Tests

Preferred pattern for multiple scenarios:

```go
tests := []struct {
    name     string
    input    InputType
    expected ExpectedType
    err      error
}{
    {
        name:     "successful case",
        input:    validInput,
        expected: expectedOutput,
    },
    {
        name:  "error case",
        input: invalidInput,
        err:   expectedError,
    },
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result, err := FunctionUnderTest(tt.input)
        require.ErrorIs(t, err, tt.err)
        require.Equal(t, tt.expected, result)
    })
}
```

### No Branching in Tests

**CRITICAL RULE**: Test bodies must be linear, no `if/switch/for` logic:

- Each test case should have a single execution path
- If logic is needed, split into multiple test cases
- Use table-driven tests to cover multiple scenarios

**Bad:**

```go
func TestFoo(t *testing.T) {
    if someCondition {
        require.Equal(t, a, b)
    } else {
        require.Equal(t, c, d)
    }
}
```

**Good:**

```go
func TestFoo_ConditionA(t *testing.T) {
    require.Equal(t, a, b)
}

func TestFoo_ConditionB(t *testing.T) {
    require.Equal(t, c, d)
}
```

## Mocking & Fakes

### Test Helpers Location

`internal/test/` contains reusable test utilities:

- `internal/test/client/` - Fake Kubernetes and Okteto clients
- `internal/test/` - Context setup, test filesystem helpers

### Mock Pattern

Implement fake structs for interfaces:

```go
type fakeExecutor struct {
    err      error
    executed []model.DeployCommand
}

func (fe *fakeExecutor) Execute(cmd model.DeployCommand, args []string) error {
    fe.executed = append(fe.executed, cmd)
    if fe.err != nil {
        return fe.err
    }
    return nil
}
```

### Using testify/mock

For complex mocking, use `github.com/stretchr/testify/mock`:

```go
type fakeDivertDriver struct {
    mock.Mock
}

func (fd *fakeDivertDriver) Deploy(ctx context.Context) error {
    args := fd.Called(ctx)
    return args.Error(0)
}
```

## Integration Test Patterns

### Build Tags

Always include at top of integration test files:

```go
//go:build integration
// +build integration
```

### Cluster Access

Integration tests assume:

- Kubernetes cluster is accessible
- Environment variables configured (see below)
- Sufficient permissions to create/delete resources

**IMPORTANT for AI agents**: Before running integration tests, verify that the required environment variables are configured. If not available, ask the user to provide:

- `OKTETO_USER` - Okteto username
- `OKTETO_PATH` - Path to okteto binary (typically `bin/okteto`)
- `OKTETO_APPS_SUBDOMAIN` - Apps subdomain
- `OKTETO_TOKEN` - Authentication token
- `OKTETO_URL` - Okteto instance URL
- `OKTETO_NAMESPACE_PREFIX` - Namespace prefix

Never run integration tests without confirming these environment variables are set.

### Test Isolation

- Use `t.TempDir()` for temporary directories
- Create unique namespaces per test
- Clean up resources in deferred functions

### Running Okteto CLI

```go
oktetoPath, err := integration.GetOktetoPath()
require.NoError(t, err)

err = commands.RunOktetoDeploy(oktetoPath, &commands.DeployOptions{
    Namespace:    namespace,
    ManifestPath: manifestPath,
})
require.NoError(t, err)
```

## Test Execution

- `make test` - All unit tests with race detection
- `make integration` - All integration tests (requires cluster)
- `make integration-build` - Only build integration tests
- `make integration-deploy` - Only deploy integration tests
- `make integration-up` - Only up integration tests

## Coverage

- Unit tests generate `coverage.txt`
- View HTML coverage: `make codecov` (opens `coverage.html`)
- Aim for meaningful coverage of business logic, not 100% line coverage

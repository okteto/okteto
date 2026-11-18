// Copyright 2026 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"errors"
	"fmt"
	"strings"

	"al.essio.dev/pkg/shellescape"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

const commandOverrideHint = `Name the test container to run and the commands to run instead of the ones in the Okteto Manifest, for example:
    okteto test unit -- go test ./pkg/api -run TestLogin
    okteto test unit --command "go mod download" --command "go test ./pkg/api -run TestLogin"`

var (
	errCommandOverrideConflict = errors.New("'--' and '--command' cannot be used together")
	errEmptyCommandAfterDash   = errors.New("no command provided after '--'")
	errEmptyCommandFlag        = errors.New("'--command' cannot be empty")
	errCommandOverrideTarget   = errors.New("overriding the commands requires exactly one test container name")
)

// commandOverride replaces the commands of a single test container with the
// ones received in the command line
type commandOverride struct {
	testName string
	commands []model.TestCommand
}

// isSet returns true when the commands of a test container are overridden
func (o commandOverride) isSet() bool {
	return len(o.commands) > 0
}

// appliesTo returns true when the commands of the test container with the given name are overridden
func (o commandOverride) appliesTo(name string) bool {
	return o.isSet() && o.testName == name
}

// resolveTest returns the test definition to run. When the commands of the test
// container are overridden, it returns a copy of the test with the overridden
// commands, keeping the rest of the definition. Otherwise, it returns the test unchanged
func (o commandOverride) resolveTest(name string, test *model.Test) *model.Test {
	if !o.appliesTo(name) {
		return test
	}
	overridden := *test
	overridden.Commands = o.commands
	return &overridden
}

// parseTestArgs splits the arguments of 'okteto test' into the names of the test containers
// to run and the command override. argsLenAtDash is the number of arguments before '--', or -1
// if '--' is not present. The override comes from the arguments after '--' or from the
// '--command' flag values, and it is only allowed when exactly one test container name is given
func parseTestArgs(args []string, argsLenAtDash int, commandFlags []string) ([]string, commandOverride, error) {
	names := args
	var dashArgs []string
	hasDash := argsLenAtDash >= 0
	if hasDash {
		names = args[:argsLenAtDash]
		dashArgs = args[argsLenAtDash:]
	}

	// Filter out empty strings, as 'okteto test ""' passes an empty string as argument, provoking a failure
	testNames := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		testNames = append(testNames, name)
	}

	if !hasDash && len(commandFlags) == 0 {
		return testNames, commandOverride{}, nil
	}

	commands, err := getOverrideCommands(hasDash, dashArgs, commandFlags)
	if err != nil {
		return nil, commandOverride{}, newCommandOverrideError(err)
	}

	if len(testNames) != 1 {
		return nil, commandOverride{}, newCommandOverrideError(fmt.Errorf("%w, got %d", errCommandOverrideTarget, len(testNames)))
	}

	return testNames, commandOverride{testName: testNames[0], commands: commands}, nil
}

// getOverrideCommands returns the commands that replace the ones of the test container.
// The arguments after '--' are joined into a single command, quoting the ones that are not
// safe shell words so each one reaches the program unchanged through the remote shell ('sh -c'),
// including spaces, '|', '$', '#' or quotes. Each '--command' flag value is a command run as is
func getOverrideCommands(hasDash bool, dashArgs, commandFlags []string) ([]model.TestCommand, error) {
	if hasDash && len(commandFlags) > 0 {
		return nil, errCommandOverrideConflict
	}

	if hasDash {
		if strings.TrimSpace(strings.Join(dashArgs, "")) == "" {
			return nil, errEmptyCommandAfterDash
		}
		command := shellescape.QuoteCommand(dashArgs)
		return []model.TestCommand{{Name: command, Command: command}}, nil
	}

	commands := make([]model.TestCommand, 0, len(commandFlags))
	for _, command := range commandFlags {
		if strings.TrimSpace(command) == "" {
			return nil, errEmptyCommandFlag
		}
		commands = append(commands, model.TestCommand{Name: command, Command: command})
	}
	return commands, nil
}

func newCommandOverrideError(err error) error {
	return oktetoErrors.UserError{
		E:    err,
		Hint: commandOverrideHint,
	}
}

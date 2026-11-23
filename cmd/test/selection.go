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
	"slices"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

const (
	groupFlagHint = `Name the group of test containers to run, for example:
    okteto test --group pre-commit`

	noGroupsDefinedHint = `None of the test containers in your Okteto Manifest belongs to a group.
    For more information, check the documentation: https://okteto.com/docs/core/okteto-manifest/#test`
)

var (
	errEmptyGroupFlag           = errors.New("'--group' cannot be empty")
	errGroupWithCommandOverride = errors.New("'--group' cannot be used when overriding the commands of a test container")
	errGroupNotFound            = errors.New("no test container belongs to the group")
)

// validateGroupFlags checks the values of the --group flag. They cannot be blank, and they cannot be
// combined with a command override ('--' or --command), which applies to exactly one test container
func validateGroupFlags(groups []string, overrideRequested bool) error {
	for _, group := range groups {
		if strings.TrimSpace(group) == "" {
			return oktetoErrors.UserError{
				E:    errEmptyGroupFlag,
				Hint: groupFlagHint,
			}
		}
	}

	if overrideRequested && len(groups) > 0 {
		return newCommandOverrideError(errGroupWithCommandOverride)
	}
	return nil
}

// selectTests returns the names of the test containers to run: the ones named in the command
// line plus the ones that belong to any of the given groups, without duplicates. The test
// containers they depend on are added later when resolving the dependency tree. When there are
// no groups, it returns the names unchanged, so no names and no groups run all the test containers.
// With groups, the result is never empty, as every group must have at least one test container:
// an empty result would run all the test containers instead of the ones of the groups.
// The tests must be validated first, so none of them is nil
func selectTests(tests model.ManifestTests, names, groups []string) ([]string, error) {
	if len(groups) == 0 {
		return names, nil
	}

	selected := make([]string, 0, len(names))
	for _, name := range names {
		if !slices.Contains(selected, name) {
			selected = append(selected, name)
		}
	}

	for _, group := range groups {
		members := testsInGroup(tests, group)
		if len(members) == 0 {
			return nil, newGroupNotFoundError(tests, group)
		}
		for _, name := range members {
			if !slices.Contains(selected, name) {
				selected = append(selected, name)
			}
		}
	}
	return selected, nil
}

// testsInGroup returns the sorted names of the test containers that belong to the group
func testsInGroup(tests model.ManifestTests, group string) []string {
	members := make([]string, 0)
	for name, test := range tests {
		if slices.Contains(test.Groups, group) {
			members = append(members, name)
		}
	}
	slices.Sort(members)
	return members
}

// definedGroups returns the sorted groups defined in the test containers, without duplicates
func definedGroups(tests model.ManifestTests) []string {
	groups := make([]string, 0)
	for _, test := range tests {
		groups = append(groups, test.Groups...)
	}
	slices.Sort(groups)
	return slices.Compact(groups)
}

func newGroupNotFoundError(tests model.ManifestTests, group string) error {
	hint := noGroupsDefinedHint
	if groups := definedGroups(tests); len(groups) > 0 {
		hint = fmt.Sprintf("The groups defined in your Okteto Manifest are: %s", strings.Join(groups, ", "))
	}
	return oktetoErrors.UserError{
		E:    fmt.Errorf("%w '%s'", errGroupNotFound, group),
		Hint: hint,
	}
}

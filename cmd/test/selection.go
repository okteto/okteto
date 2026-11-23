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

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
)

const (
	profileFlagSource = "'--profile'"
	profileEnvSource  = "'" + constants.OktetoProfilesEnvVar + "'"

	profileFlagHint = `Name the profile of the test containers to run, for example:
    okteto test --profile pre-commit`

	profileWithTestNamesHint = `Run the test containers either by name or by profile, for example:
    okteto test unit lint
    okteto test --profile pre-commit`

	profileEnvConflictHint = `Unset the OKTETO_PROFILES environment variable to run test containers by name or replace their commands`

	noProfilesDefinedHint = `None of the test containers in your Okteto Manifest has a profile.
    For more information, check the documentation: https://okteto.com/docs/core/okteto-manifest/#test`

	dependencyProfilesHint = `A test container can only depend on test containers without profiles or with an enabled profile.
    Enable one of the profiles of the dependency too, or update the profiles in your Okteto Manifest`
)

var (
	errEmptyProfileFlag             = errors.New("'--profile' cannot be empty")
	errProfileWithCommandOverride   = errors.New("cannot be used when overriding the commands of a test container")
	errProfileWithTestNames         = errors.New("cannot be used when naming the test containers to run")
	errProfileNotFound              = errors.New("no test container has the profile")
	errNoTestsWithoutProfiles       = errors.New("all the test containers have profiles, so none runs without enabling a profile")
	errDependencyProfilesNotEnabled = errors.New("none of its profiles is enabled")
)

// profileSelection holds the enabled profiles and the option that sets them
type profileSelection struct {
	profiles []string
	// source is the option that sets the profiles: the --profile flag or the OKTETO_PROFILES environment variable
	source string
}

// resolveProfiles returns the enabled profiles. The OKTETO_PROFILES environment variable, a
// comma-separated list of profiles, overrides the values of the --profile flag. The variable is
// ignored when it has no profiles, so an empty value doesn't override the flag
func resolveProfiles(flagProfiles []string, envValue string) (profileSelection, error) {
	if envProfiles := parseProfilesEnv(envValue); len(envProfiles) > 0 {
		return profileSelection{profiles: envProfiles, source: profileEnvSource}, nil
	}

	for _, profile := range flagProfiles {
		if strings.TrimSpace(profile) == "" {
			return profileSelection{}, oktetoErrors.UserError{
				E:    errEmptyProfileFlag,
				Hint: profileFlagHint,
			}
		}
	}
	return profileSelection{profiles: flagProfiles, source: profileFlagSource}, nil
}

// parseProfilesEnv splits the value of the OKTETO_PROFILES environment variable into profiles,
// trimming the spaces around them and ignoring the empty ones
func parseProfilesEnv(value string) []string {
	profiles := make([]string, 0)
	for _, profile := range strings.Split(value, ",") {
		if profile = strings.TrimSpace(profile); profile != "" {
			profiles = append(profiles, profile)
		}
	}
	return profiles
}

func (s profileSelection) isSet() bool {
	return len(s.profiles) > 0
}

func (s profileSelection) fromEnv() bool {
	return s.source == profileEnvSource
}

// checkCommandOverride returns an error when profiles are enabled and the commands of a test
// container are replaced ('--' or --command), as that applies to exactly one named test container
func (s profileSelection) checkCommandOverride(overrideRequested bool) error {
	if !s.isSet() || !overrideRequested {
		return nil
	}
	err := fmt.Errorf("%s %w", s.source, errProfileWithCommandOverride)
	if s.fromEnv() {
		return oktetoErrors.UserError{E: err, Hint: profileEnvConflictHint}
	}
	return newCommandOverrideError(err)
}

// checkTestNames returns an error when profiles are enabled and test containers are named,
// as the test containers to run are selected either by name or by profile
func (s profileSelection) checkTestNames(names []string) error {
	if !s.isSet() || len(names) == 0 {
		return nil
	}
	hint := profileWithTestNamesHint
	if s.fromEnv() {
		hint = profileEnvConflictHint
	}
	return oktetoErrors.UserError{
		E:    fmt.Errorf("%s %w", s.source, errProfileWithTestNames),
		Hint: hint,
	}
}

// selectTests returns the names of the test containers to run. Named test containers run whatever
// their profiles, so the names are returned unchanged. Otherwise, as in Docker Compose, the test
// containers without profiles run along with the ones that have any of the enabled profiles. The test
// containers they depend on must run too, so depending on one whose profiles are not enabled is an
// error instead of running it. Without names, the result is never empty, as an empty selection would
// run all the test containers. Names and profiles are never combined (see checkTestNames), and the
// tests must be validated first, so none of them is nil
func selectTests(tests model.ManifestTests, names, profiles []string) ([]string, error) {
	if len(names) > 0 {
		return names, nil
	}

	defined := definedProfiles(tests)
	for _, profile := range profiles {
		if !slices.Contains(defined, profile) {
			return nil, newProfileNotFoundError(defined, profile)
		}
	}

	selected := make([]string, 0, len(tests))
	for name, test := range tests {
		if isEnabled(test, profiles) {
			selected = append(selected, name)
		}
	}
	if len(selected) == 0 {
		return nil, newNoTestsWithoutProfilesError(defined)
	}
	slices.Sort(selected)

	for _, name := range selected {
		for _, dependency := range tests[name].DependsOn {
			// Unknown dependencies are reported when building the dependency tree
			if test, ok := tests[dependency]; ok && !isEnabled(test, profiles) {
				return nil, newDependencyNotEnabledError(name, dependency, test.Profiles)
			}
		}
	}
	return selected, nil
}

// isEnabled returns true when the test container has no profiles or any of them is enabled
func isEnabled(test *model.Test, profiles []string) bool {
	if len(test.Profiles) == 0 {
		return true
	}
	return slices.ContainsFunc(test.Profiles, func(profile string) bool {
		return slices.Contains(profiles, profile)
	})
}

// definedProfiles returns the sorted profiles defined in the test containers, without duplicates
func definedProfiles(tests model.ManifestTests) []string {
	profiles := make([]string, 0)
	for _, test := range tests {
		profiles = append(profiles, test.Profiles...)
	}
	slices.Sort(profiles)
	return slices.Compact(profiles)
}

func newProfileNotFoundError(defined []string, profile string) error {
	hint := noProfilesDefinedHint
	if len(defined) > 0 {
		hint = fmt.Sprintf("The profiles defined in your Okteto Manifest are: %s", strings.Join(defined, ", "))
	}
	return oktetoErrors.UserError{
		E:    fmt.Errorf("%w '%s'", errProfileNotFound, profile),
		Hint: hint,
	}
}

// newNoTestsWithoutProfilesError returns the error when every test container has a profile and none is enabled
func newNoTestsWithoutProfilesError(defined []string) error {
	return oktetoErrors.UserError{
		E: errNoTestsWithoutProfiles,
		Hint: fmt.Sprintf(`Enable a profile with '--profile' or the OKTETO_PROFILES environment variable, for example:
    okteto test --profile <name>
    The profiles defined in your Okteto Manifest are: %s`, strings.Join(defined, ", ")),
	}
}

func newDependencyNotEnabledError(name, dependency string, dependencyProfiles []string) error {
	return oktetoErrors.UserError{
		E:    fmt.Errorf("test container '%s' depends on '%s', but %w: %s", name, dependency, errDependencyProfilesNotEnabled, strings.Join(dependencyProfiles, ", ")),
		Hint: dependencyProfilesHint,
	}
}

// Copyright 2024 The Okteto Authors
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
	"context"
	"fmt"
	"testing"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommandRejectsInvalidOverride checks that the cobra command passes the arguments after '--'
// and the --command flag to parseTestArgs. These errors are returned before connecting to any cluster
func TestCommandRejectsInvalidOverride(t *testing.T) {
	t.Setenv(constants.OktetoProfilesEnvVar, "")
	tests := []struct {
		name        string
		args        []string
		expectedErr error
	}{
		{
			name:        "dash without test name",
			args:        []string{"--", "go", "test"},
			expectedErr: errCommandOverrideTarget,
		},
		{
			name:        "dash with several test names",
			args:        []string{"lint", "unit", "--", "go", "test"},
			expectedErr: errCommandOverrideTarget,
		},
		{
			name:        "dash without command",
			args:        []string{"unit", "--"},
			expectedErr: errEmptyCommandAfterDash,
		},
		{
			name:        "command flag without test name",
			args:        []string{"--command", "go test ./..."},
			expectedErr: errCommandOverrideTarget,
		},
		{
			name:        "empty command flag",
			args:        []string{"unit", "--command", ""},
			expectedErr: errEmptyCommandFlag,
		},
		{
			name:        "dash and command flag together",
			args:        []string{"unit", "--command", "go test ./...", "--", "go", "test"},
			expectedErr: errCommandOverrideConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Test(context.Background(), io.NewIOController(), nil, nil, nil)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

// TestCommandRejectsInvalidProfile checks that the cobra command validates the --profile flag.
// These errors are returned before connecting to any cluster
func TestCommandRejectsInvalidProfile(t *testing.T) {
	t.Setenv(constants.OktetoProfilesEnvVar, "")
	tests := []struct {
		name        string
		args        []string
		expectedErr error
	}{
		{
			name:        "empty profile flag",
			args:        []string{"--profile", ""},
			expectedErr: errEmptyProfileFlag,
		},
		{
			name:        "blank profile flag",
			args:        []string{"--profile", " "},
			expectedErr: errEmptyProfileFlag,
		},
		{
			name:        "profile flag with test names",
			args:        []string{"unit", "lint", "--profile", "pre-commit"},
			expectedErr: errProfileWithTestNames,
		},
		{
			name:        "profile flag with dash override",
			args:        []string{"unit", "--profile", "pre-commit", "--", "go", "test"},
			expectedErr: errProfileWithCommandOverride,
		},
		{
			name:        "profile flag with command flag",
			args:        []string{"unit", "--profile", "pre-commit", "--command", "go test ./..."},
			expectedErr: errProfileWithCommandOverride,
		},
		{
			name:        "profile flag with dash override and no test name",
			args:        []string{"--profile", "pre-commit", "--", "go", "test"},
			expectedErr: errProfileWithCommandOverride,
		},
		{
			name:        "profile flag with command flag and no test name",
			args:        []string{"--profile", "pre-commit", "--command", "go test ./..."},
			expectedErr: errProfileWithCommandOverride,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Test(context.Background(), io.NewIOController(), nil, nil, nil)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

// TestCommandRejectsProfilesFromEnvWithTestNames checks that the profiles of the OKTETO_PROFILES
// environment variable cannot be combined with test container names, even when the --profile flag is
// not used. The variable overrides the flag, so an empty flag value is not validated
func TestCommandRejectsProfilesFromEnvWithTestNames(t *testing.T) {
	t.Setenv(constants.OktetoProfilesEnvVar, "ci")
	tests := []struct {
		name        string
		args        []string
		expectedErr string
	}{
		{
			name:        "test names",
			args:        []string{"unit"},
			expectedErr: "'OKTETO_PROFILES' cannot be used when naming the test containers to run",
		},
		{
			name:        "test names and an empty profile flag",
			args:        []string{"unit", "--profile", ""},
			expectedErr: "'OKTETO_PROFILES' cannot be used when naming the test containers to run",
		},
		{
			name:        "dash override",
			args:        []string{"unit", "--", "go", "test"},
			expectedErr: "'OKTETO_PROFILES' cannot be used when overriding the commands of a test container",
		},
		{
			name:        "command flag",
			args:        []string{"unit", "--command", "go test ./..."},
			expectedErr: "'OKTETO_PROFILES' cannot be used when overriding the commands of a test container",
		},
		{
			name:        "dash override and no test name",
			args:        []string{"--", "go", "test"},
			expectedErr: "'OKTETO_PROFILES' cannot be used when overriding the commands of a test container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := Test(context.Background(), io.NewIOController(), nil, nil, nil)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs(tt.args)

			err := cmd.Execute()

			require.EqualError(t, err, tt.expectedErr)
		})
	}
}

// TestResolveEnabledProfiles checks the profiles resolved once the context loads the .env file and
// the Okteto Variables into the environment, which can set OKTETO_PROFILES
func TestResolveEnabledProfiles(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		flagProfiles []string
		expected     []string
	}{
		{
			name: "no profiles",
		},
		{
			name:         "profiles from the flag",
			flagProfiles: []string{"pre-commit"},
			expected:     []string{"pre-commit"},
		},
		{
			name:         "the environment variable overrides the flag",
			envValue:     "ci, release",
			flagProfiles: []string{"pre-commit"},
			expected:     []string{"ci", "release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(constants.OktetoProfilesEnvVar, tt.envValue)
			options := &Options{Profiles: tt.flagProfiles}

			err := resolveEnabledProfiles(nil, options, io.NewIOController())

			require.NoError(t, err)
			require.Equal(t, tt.expected, options.enabledProfiles)
		})
	}
}

func TestResolveEnabledProfilesRejectsConflicts(t *testing.T) {
	tests := []struct {
		name        string
		options     *Options
		expectedErr error
	}{
		{
			name:        "test names",
			options:     &Options{},
			expectedErr: errProfileWithTestNames,
		},
		{
			name: "command override",
			options: &Options{
				override: commandOverride{testName: "unit", commands: []model.TestCommand{{Command: "go test"}}},
			},
			expectedErr: errProfileWithCommandOverride,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(constants.OktetoProfilesEnvVar, "ci")

			err := resolveEnabledProfiles([]string{"unit"}, tt.options, io.NewIOController())

			require.ErrorIs(t, err, tt.expectedErr)
			require.Nil(t, tt.options.enabledProfiles)
		})
	}
}

type fakeSvcFromRegex struct {
	err error
	svc string
}

type fakeSvcToBuildDuringDeploy struct {
	err  error
	svcs []string
}

type fakeBuilderV2 struct {
	build                  error
	svcFromRegex           fakeSvcFromRegex
	svcToBuildDuringDeploy fakeSvcToBuildDuringDeploy
}

func (fb *fakeBuilderV2) GetSvcToBuildFromRegex(manifest *model.Manifest, imgFinder model.ImageFromManifest) (string, error) {
	return fb.svcFromRegex.svc, fb.svcFromRegex.err
}
func (fb *fakeBuilderV2) GetServicesToBuildDuringExecution(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error) {
	return fb.svcToBuildDuringDeploy.svcs, fb.svcToBuildDuringDeploy.err
}
func (fb *fakeBuilderV2) Build(ctx context.Context, options *types.BuildOptions) error {
	return fb.build
}

func TestDoBuild(t *testing.T) {
	ctx := context.TODO()
	manifest := &model.Manifest{
		Build: build.ManifestBuild{
			"svc1": {
				Image: "image1",
			},
			"svc2": {
				Image: "image2",
			},
		},
		Test: map[string]*model.Test{
			"svc1": {
				Image: "image1",
			},
			"svc2": {
				Image: "image2",
			},
			"svc3": {
				Image: "$OKTETO_BUILD_NOIMAGE_IMAGE",
			},
		},
	}

	ioCtrl := io.NewIOController()

	type input struct {
		builder *fakeBuilderV2
		svcs    []string
	}
	type expected struct {
		err      error
		wasBuilt bool
	}

	tt := []struct {
		expected expected
		name     string
		input    input
	}{
		{
			name: "image on okteto variable is not in build section",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: buildv2.ErrOktetBuildSyntaxImageIsNotInBuildSection,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      fmt.Errorf("test 'svc3' needs image '$OKTETO_BUILD_NOIMAGE_IMAGE' but it's not defined in the build section of the Okteto Manifest. See: https://www.okteto.com/docs/core/okteto-variables/#built-in-environment-variables-for-images-in-okteto-registry"),
				wasBuilt: false,
			},
		},
		{
			name: "no images to build",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: buildv2.ErrImageIsNotAOktetoBuildSyntax,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      nil,
				wasBuilt: false,
			},
		},
		{
			name: "another error on get svc to build from regex",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: assert.AnError,
						svc: "",
					},
				},
				svcs: []string{"svc3"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: false,
			},
		},
		{
			name: "error checking services to build during deploy",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  assert.AnError,
						svcs: []string{},
					},
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: false,
			},
		},
		{
			name: "error on build",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  nil,
						svcs: []string{"svc1"},
					},
					build: assert.AnError,
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      assert.AnError,
				wasBuilt: true,
			},
		},
		{
			name: "buildSuccess",
			input: input{
				builder: &fakeBuilderV2{
					svcFromRegex: fakeSvcFromRegex{
						err: nil,
						svc: "svc1",
					},
					svcToBuildDuringDeploy: fakeSvcToBuildDuringDeploy{
						err:  nil,
						svcs: []string{"svc1"},
					},
					build: nil,
				},
				svcs: []string{"svc1"},
			},
			expected: expected{
				err:      nil,
				wasBuilt: true,
			},
		},
	}

	for _, tt := range tt {
		t.Run(tt.name, func(t *testing.T) {
			wasBuilt, err := doBuild(ctx, manifest, tt.input.svcs, tt.input.builder, ioCtrl)
			if tt.expected.err != nil {
				assert.ErrorContains(t, err, tt.expected.err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected.wasBuilt, wasBuilt)
		})
	}
}

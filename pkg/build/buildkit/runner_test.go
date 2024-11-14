// Copyright 2023 The Okteto Authors
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

package buildkit

import (
	"context"
	"fmt"
	"testing"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
)

type fakeBuildkitWaiter struct {
	err []error
}

func (m *fakeBuildkitWaiter) WaitUntilIsUp(context.Context) error {
	if len(m.err) > 0 {
		err := m.err[0]
		m.err = m.err[1:]
		return err
	}
	return nil
}

type fakeRegistryImageChecker struct {
	err []error
}

func (m *fakeRegistryImageChecker) GetImageTagWithDigest(string) (string, error) {
	if len(m.err) > 0 {
		err := m.err[0]
		m.err = m.err[1:]
		return "", err
	}
	return "digest", nil
}

type fakeBuildkitClientFactory struct {
	err []error
}

func (m *fakeBuildkitClientFactory) GetBuildkitClient(context.Context) (*client.Client, error) {
	if len(m.err) > 0 {
		err := m.err[0]
		m.err = m.err[1:]
		return nil, err
	}
	return &client.Client{}, nil
}

func TestRunnerRun(t *testing.T) {
	type input struct {
		buildkitWaiter           buildkitWaiterInterface
		buildkitClientFactory    buildkitClientFactory
		fakeRegistryImageChecker registryImageChecker
		fakeSolver               SolveBuildFn
	}
	type output struct {
		err      error
		attempts int
	}
	var solveAttempts int
	tests := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "failed to wait for buildkit available",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{assert.AnError},
				},
			},
			output: output{
				err:      assert.AnError,
				attempts: 1,
			},
		},
		{
			name: "buildkit client fails to retrieve after waiting and fail",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{nil, assert.AnError},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{assert.AnError},
				},
			},
			output: output{
				err:      assert.AnError,
				attempts: 2,
			},
		},
		{
			name: "non retryable error",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{},
				},
				fakeSolver: func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error {
					return assert.AnError
				},
			},
			output: output{
				err:      assert.AnError,
				attempts: 1,
			},
		},
		{
			name: "retryable error until error",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{},
				},
				fakeSolver: func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error {
					solveAttempts++
					if solveAttempts <= 3 {
						return fmt.Errorf("transport: error while dialing: dial tcp: i/o timeout")
					}
					return assert.AnError
				},
			},
			output: output{
				err:      assert.AnError,
				attempts: 4,
			},
		},
		{
			name: "retryable error to wait failure",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{nil, assert.AnError},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{},
				},
				fakeSolver: func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error {
					return fmt.Errorf("transport: error while dialing: dial tcp: i/o timeout")
				},
			},
			output: output{
				err:      assert.AnError,
				attempts: 2,
			},
		},
		{
			name: "image not pushed correctly",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{},
				},
				fakeSolver: func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error {
					return nil
				},
				fakeRegistryImageChecker: &fakeRegistryImageChecker{
					err: []error{assert.AnError, assert.AnError, assert.AnError, assert.AnError, assert.AnError},
				},
			},
			output: output{
				err:      ErrBuildConnecionFailed,
				attempts: 5,
			},
		},
		{
			name: "image pushed correctly",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{},
				},
				fakeSolver: func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error {
					return nil
				},
				fakeRegistryImageChecker: &fakeRegistryImageChecker{
					err: nil,
				},
			},
			output: output{
				err:      nil,
				attempts: 1,
			},
		},
		{
			name: "image pushed correctly after 1 retry",
			input: input{
				buildkitWaiter: &fakeBuildkitWaiter{
					err: []error{},
				},
				buildkitClientFactory: &fakeBuildkitClientFactory{
					err: []error{},
				},
				fakeSolver: func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error {
					return nil
				},
				fakeRegistryImageChecker: &fakeRegistryImageChecker{
					err: []error{assert.AnError, nil},
				},
			},
			output: output{
				err:      nil,
				attempts: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			solveAttempts = 0
			fmt.Print(solveAttempts)
			r := &Runner{
				waiter:                             tt.input.buildkitWaiter,
				clientFactory:                      tt.input.buildkitClientFactory,
				solveBuild:                         tt.input.fakeSolver,
				registry:                           tt.input.fakeRegistryImageChecker,
				metadata:                           &runnerMetadata{},
				logger:                             io.NewIOController(),
				maxAttemptsBuildkitTransientErrors: 5,
			}
			err := r.Run(context.Background(), &client.SolveOpt{Exports: []client.ExportEntry{
				{
					Type: "image",
					Attrs: map[string]string{
						"push": "true",
						"name": "image:latest",
					},
				},
			}}, "")
			assert.ErrorIs(t, err, tt.output.err)
			assert.Equal(t, tt.output.attempts, r.metadata.attempts)
		})
	}
}

func TestExtractTagsFromOpt(t *testing.T) {
	tests := []struct {
		name     string
		opt      *client.SolveOpt
		expected string
	}{
		{
			name: "single image export with push",
			opt: &client.SolveOpt{
				Exports: []client.ExportEntry{
					{
						Type: "image",
						Attrs: map[string]string{
							"push": "true",
							"name": "image:latest",
						},
					},
				},
			},
			expected: "image:latest",
		},
		{
			name: "multiple exports with one image push",
			opt: &client.SolveOpt{
				Exports: []client.ExportEntry{
					{
						Type: "local",
					},
					{
						Type: "image",
						Attrs: map[string]string{
							"push": "true",
							"name": "image:latest",
						},
					},
				},
			},
			expected: "image:latest",
		},
		{
			name: "image export without push",
			opt: &client.SolveOpt{
				Exports: []client.ExportEntry{
					{
						Type: "image",
						Attrs: map[string]string{
							"push": "false",
							"name": "image:latest",
						},
					},
				},
			},
			expected: "",
		},
		{
			name:     "no exports",
			opt:      &client.SolveOpt{},
			expected: "",
		},
		{
			name: "nil attributes",
			opt: &client.SolveOpt{
				Exports: []client.ExportEntry{
					{
						Type:  "image",
						Attrs: nil,
					},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{}
			tag := r.extractTagsFromOpt(tt.opt)
			assert.Equal(t, tt.expected, tag)
		})
	}
}
func TestNewBuildkitRunner(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		expected int
	}{
		{
			name:     "default max attempts",
			envVar:   "",
			expected: defaultMaxAttempts,
		},
		{
			name:     "custom max attempts",
			envVar:   "5",
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv(MaxRetriesForBuildkitTransientErrorsEnvVar, tt.envVar)
			}

			runner := NewBuildkitRunner(&fakeBuildkitClientFactory{}, &fakeBuildkitWaiter{}, &fakeRegistryImageChecker{}, func(context.Context, *client.Client, *client.SolveOpt, string, *io.Controller) error { return nil }, io.NewIOController())

			assert.Implements(t, (*buildkitClientFactory)(nil), runner.clientFactory)
			assert.Implements(t, (*buildkitWaiterInterface)(nil), runner.waiter)
			assert.Implements(t, (*registryImageChecker)(nil), runner.registry)
			assert.NotNil(t, runner.solveBuild)
			assert.NotNil(t, runner.logger)
			assert.NotNil(t, runner.metadata)
			assert.Equal(t, tt.expected, runner.maxAttemptsBuildkitTransientErrors)
		})
	}
}
func TestCheckIfImageIsPushed(t *testing.T) {
	tests := []struct {
		expected error
		checker  registryImageChecker
		name     string
		tag      string
	}{
		{
			name: "tag is empty",
			tag:  "",
			checker: &fakeRegistryImageChecker{
				err: []error{assert.AnError},
			},
			expected: nil,
		},
		{
			name: "single tag, image pushed successfully",
			tag:  "image:latest",
			checker: &fakeRegistryImageChecker{
				err: nil,
			},
			expected: nil,
		},
		{
			name: "single tag, image push failed",
			tag:  "image:latest",
			checker: &fakeRegistryImageChecker{
				err: []error{assert.AnError},
			},
			expected: assert.AnError,
		},
		{
			name: "multiple tags, all images pushed successfully",
			tag:  "image:latest,image:stable",
			checker: &fakeRegistryImageChecker{
				err: nil,
			},
			expected: nil,
		},
		{
			name: "multiple tags, one image push failed",
			tag:  "image:latest,image:stable",
			checker: &fakeRegistryImageChecker{
				err: []error{nil, assert.AnError},
			},
			expected: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				registry: tt.checker,
			}
			err := r.checkIfImageIsPushed(tt.tag)
			if tt.expected != nil {
				assert.ErrorIs(t, err, tt.expected)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

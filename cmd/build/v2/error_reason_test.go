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

package v2

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/build/buildkit"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
)

// The errors below are built the way the build code builds them, wrapped as
// buildSvcFromDockerfile returns them.
func TestImageBuildErrorReason(t *testing.T) {
	missingFile := &fs.PathError{Op: "open", Path: "/home/user/app/.env", Err: fs.ErrNotExist}
	tests := []struct {
		err      error
		name     string
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name: "secret source missing",
			err: fmt.Errorf("failed to create build solver: %w",
				fmt.Errorf("failed to process build secrets: %w",
					fmt.Errorf("error creating the temp file with expanded values: %w", missingFile))),
			expected: buildErrorSecretError,
		},
		{
			name:     "secret format",
			err:      fmt.Errorf("failed to create build solver: %w", fmt.Errorf("failed to process build secrets: %w", errors.New("secret format error"))),
			expected: buildErrorSecretError,
		},
		{
			name: "build queue timeout",
			err: oktetoErrors.UserError{
				E: errors.New("waiting in the Okteto Build queue timed out after 10m0s"),
			},
			expected: buildErrorQueueTimeout,
		},
		{
			name:     "buildkit connection failure after retries",
			err:      buildkit.ErrBuildConnecionFailed,
			expected: buildErrorConnectionFailure,
		},
		{
			name:     "buildkit service unavailable",
			err:      buildkit.GetSolveErrorMessage(errors.New("rpc error: code = Unavailable desc = connect: connection refused")),
			expected: buildErrorConnectionFailure,
		},
		{
			name:     "legacy buildkit wait timeout",
			err:      errors.New("buildkit service not available for 10m0s"),
			expected: buildErrorConnectionFailure,
		},
		{
			name:     "buildkit pod assignment",
			err:      fmt.Errorf("failed to assign buildkit pod: %w", errors.New("could not get least loaded buildkit pod: 503")),
			expected: buildErrorConnectionFailure,
		},
		{
			name:     "port forward to buildkit",
			err:      fmt.Errorf("port forward creation to BuildKit has failed: %w", errors.New("pod not running")),
			expected: buildErrorConnectionFailure,
		},
		{
			name:     "registry 5xx",
			err:      buildkit.GetSolveErrorMessage(errors.New("failed to push: unexpected status from PUT request to https://registry.example.com/v2/app/manifests/1.0: 503 Service Unavailable")),
			expected: buildErrorRegistryError,
		},
		{
			name:     "image digest not found after push",
			err:      fmt.Errorf("error accessing image at registry %s after 3 attempts: %w", "okteto.dev/api:1.0", errors.New("MANIFEST_UNKNOWN")),
			expected: buildErrorRegistryError,
		},
		{
			name:     "base image not accessible",
			err:      buildkit.GetSolveErrorMessage(errors.New("failed to solve: docker.io/library/nope:latest: pull access denied")),
			expected: buildErrorImageNotFound,
		},
		{
			name:     "dockerfile path does not exist",
			err:      oktetoErrors.ErrManifestPathNotFound,
			expected: buildErrorDockerfileMissing,
		},
		{
			name:     "dockerfile path is a directory",
			err:      oktetoErrors.ErrManifestPathIsDir,
			expected: buildErrorDockerfileMissing,
		},
		{
			name: "dockerfile missing in build context",
			err: fmt.Errorf("failed to create build solver: %w",
				fmt.Errorf("file '%s' not found: %w", "api/Dockerfile", &fs.PathError{Op: "stat", Path: "api/Dockerfile", Err: fs.ErrNotExist})),
			expected: buildErrorDockerfileMissing,
		},
		{
			name:     "dockerfile step failed",
			err:      buildkit.CommandErr{Stage: "RUN npm install", Err: errors.New("exit code 1")},
			expected: buildErrorDockerfileFailed,
		},
		{
			name:     "dockerfile RUN step failed",
			err:      buildkit.GetSolveErrorMessage(errors.New(`failed to solve: process "/bin/sh -c npm ci" did not complete successfully: exit code: 1`)),
			expected: buildErrorDockerfileFailed,
		},
		{
			name:     "dockerfile RUN step echoing a connection message",
			err:      buildkit.GetSolveErrorMessage(errors.New(`failed to solve: process "/bin/sh -c echo 'buildkit service not available for 1s'" did not complete successfully: exit code: 1`)),
			expected: buildErrorDockerfileFailed,
		},
		{
			name:     "dockerfile COPY of a missing file",
			err:      buildkit.GetSolveErrorMessage(errors.New(`failed to solve: failed to compute cache key: failed to calculate checksum of ref 4f3c::abc: "/package.json": not found`)),
			expected: buildErrorDockerfileFailed,
		},
		{
			name:     "dockerfile syntax error",
			err:      buildkit.GetSolveErrorMessage(errors.New("failed to solve: dockerfile parse error on line 3: unknown instruction: RUNN")),
			expected: buildErrorDockerfileFailed,
		},
		{
			name:     "unknown error with credentials in a URL",
			err:      errors.New("failed to clone https://user:pass@github.com/org/repo"),
			expected: buildErrorUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, imageBuildErrorReason(tt.err))
		})
	}
}

func TestBuild_FailureSetsErrorReasonCategory(t *testing.T) {
	dir, err := createDockerfile(t)
	require.NoError(t, err)
	registry := newFakeRegistry()
	rawErr := buildkit.GetSolveErrorMessage(errors.New(`failed to solve: process "/bin/sh -c git clone https://user:pass@github.com/org/repo" did not complete successfully: exit code: 128`))
	builder := test.NewFakeOktetoBuilder(registry, rawErr)
	bc := NewFakeBuilder(builder, registry, fakeConfig{isOkteto: true})
	var finished []*analytics.ImageBuildMetadata
	bc.onBuildFinish = []OnBuildFinish{func(_ context.Context, m *analytics.ImageBuildMetadata) {
		finished = append(finished, m)
	}}
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"api": &build.Info{Context: dir, Dockerfile: filepath.Join(dir, "Dockerfile"), Image: "okteto/api:test"},
		},
	}

	err = bc.Build(context.Background(), &types.BuildOptions{Manifest: manifest, NoCache: true})

	require.ErrorIs(t, err, rawErr)
	require.Len(t, finished, 1)
	require.False(t, finished[0].Success)
	require.Equal(t, buildErrorDockerfileFailed, finished[0].ErrorReason)
}

func TestBuild_SuccessLeavesErrorReasonEmpty(t *testing.T) {
	dir, err := createDockerfile(t)
	require.NoError(t, err)
	registry := newFakeRegistry()
	bc := NewFakeBuilder(test.NewFakeOktetoBuilder(registry), registry, fakeConfig{isOkteto: true})
	var finished []*analytics.ImageBuildMetadata
	bc.onBuildFinish = []OnBuildFinish{func(_ context.Context, m *analytics.ImageBuildMetadata) {
		finished = append(finished, m)
	}}
	manifest := &model.Manifest{
		Name: "test",
		Build: build.ManifestBuild{
			"api": &build.Info{Context: dir, Dockerfile: filepath.Join(dir, "Dockerfile"), Image: "okteto/api:test"},
		},
	}

	err = bc.Build(context.Background(), &types.BuildOptions{Manifest: manifest, NoCache: true})

	require.NoError(t, err)
	require.Len(t, finished, 1)
	require.True(t, finished[0].Success)
	require.Empty(t, finished[0].ErrorReason)
}

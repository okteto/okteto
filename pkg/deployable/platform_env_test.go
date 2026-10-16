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

package deployable

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func splitVars(t *testing.T, vars []string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, v := range vars {
		k, val, ok := strings.Cut(v, "=")
		require.True(t, ok, "variable %q must be key=value", v)
		result[k] = val
	}
	return result
}

func TestGetPlatformEnvironmentEnvAndFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	c := &client.FakeUserClient{
		ExecutionEnv: map[string]string{
			"PLATFORM_CLIENT_ID": "client",
			"PLATFORM_TENANT_ID": "tenant",
		},
		ExecutionFiles: []types.ExecutionFile{
			{EnvVar: "PLATFORM_TOKEN_FILE", Content: "jwt"},
		},
	}

	vars, cleanup := getPlatformEnvironment(context.Background(), c, fs)
	got := splitVars(t, vars)

	require.Len(t, got, 3)
	assert.Equal(t, "client", got["PLATFORM_CLIENT_ID"])
	assert.Equal(t, "tenant", got["PLATFORM_TENANT_ID"])

	path := got["PLATFORM_TOKEN_FILE"]
	require.NotEmpty(t, path)
	assert.Equal(t, "PLATFORM_TOKEN_FILE", filepath.Base(path))

	content, err := afero.ReadFile(fs, path)
	require.NoError(t, err)
	assert.Equal(t, "jwt", string(content))

	info, err := fs.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	cleanup()
	_, err = fs.Stat(filepath.Dir(path))
	assert.True(t, os.IsNotExist(err), "platform files dir must be removed on cleanup")
}

func TestGetPlatformEnvironmentNoFiles(t *testing.T) {
	fs := afero.NewMemMapFs()
	c := &client.FakeUserClient{
		ExecutionEnv: map[string]string{"CLOUDSDK_AUTH_ACCESS_TOKEN": "token"},
	}

	vars, cleanup := getPlatformEnvironment(context.Background(), c, fs)
	defer cleanup()

	assert.Equal(t, []string{"CLOUDSDK_AUTH_ACCESS_TOKEN=token"}, vars)
	_, err := fs.Stat(os.TempDir())
	assert.True(t, os.IsNotExist(err), "no temporary dir must be created without files")
}

func TestGetPlatformEnvironmentErrors(t *testing.T) {
	fs := afero.NewMemMapFs()
	c := &client.FakeUserClient{
		ErrExecutionEnv:   errors.New("env failed"),
		ErrExecutionFiles: errors.New("files failed"),
	}

	vars, cleanup := getPlatformEnvironment(context.Background(), c, fs)
	defer cleanup()

	assert.Empty(t, vars)
}

func TestGetPlatformEnvironmentFilesSurviveEnvError(t *testing.T) {
	fs := afero.NewMemMapFs()
	c := &client.FakeUserClient{
		ErrExecutionEnv: errors.New("env failed"),
		ExecutionFiles:  []types.ExecutionFile{{EnvVar: "PLATFORM_TOKEN_FILE", Content: "jwt"}},
	}

	vars, cleanup := getPlatformEnvironment(context.Background(), c, fs)
	defer cleanup()

	got := splitVars(t, vars)
	require.Len(t, got, 1)
	assert.Contains(t, got, "PLATFORM_TOKEN_FILE")
}

func TestWritePlatformFilesRejectsInvalidEnvVarNames(t *testing.T) {
	fs := afero.NewMemMapFs()

	vars, cleanup := writePlatformFiles(fs, []types.ExecutionFile{
		{EnvVar: "../../etc/passwd", Content: "x"},
		{EnvVar: "WITH SPACE", Content: "x"},
		{EnvVar: "", Content: "x"},
		{EnvVar: "VALID_NAME", Content: "ok"},
	})
	defer cleanup()

	got := splitVars(t, vars)
	require.Len(t, got, 1)
	content, err := afero.ReadFile(fs, got["VALID_NAME"])
	require.NoError(t, err)
	assert.Equal(t, "ok", string(content))
}

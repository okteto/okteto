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

package build

import (
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestCreateDockerfileWithVolumeMounts(t *testing.T) {
	fs := afero.NewMemMapFs()

	oktetoHome, err := filepath.Abs("./tmp/tests")
	require.NoError(t, err)
	err = fs.MkdirAll(oktetoHome, 0700)
	require.NoError(t, err)

	// Set the Okteto home to facilitate where the dockerfile will be created
	t.Setenv(constants.OktetoFolderEnvVar, oktetoHome)

	image := "nginx:latest"
	volumes := []VolumeMounts{
		{
			LocalPath:  "/local/path",
			RemotePath: "/remote/path",
		},
		{
			LocalPath:  "./nginx/nginx.conf",
			RemotePath: "/etc/nginx/nginx.conf",
		},
	}
	ctx := filepath.Join("tmp", "test", "volume-mount")
	expectedContext, err := filepath.Abs(ctx)
	require.NoError(t, err)

	info, err := CreateDockerfileWithVolumeMounts(ctx, image, volumes, fs)
	require.NoError(t, err)

	require.ElementsMatch(t, volumes, info.VolumesToInclude)
	require.Equal(t, expectedContext, info.Context)

	dockerfileContent, err := afero.ReadFile(fs, info.Dockerfile)
	require.NoError(t, err)

	expected := "FROM nginx:latest\nCOPY /local/path /remote/path\nCOPY ./nginx/nginx.conf /etc/nginx/nginx.conf\n"
	require.Equal(t, expected, string(dockerfileContent))
}

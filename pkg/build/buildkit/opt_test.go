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
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func Test_replaceSecretsSourceEnvWithTempFile(t *testing.T) {
	t.Parallel()
	fakeFs := afero.NewMemMapFs()
	localSrcFile, err := afero.TempFile(fakeFs, t.TempDir(), "")
	require.NoError(t, err)

	tests := []struct {
		fs                      afero.Fs
		buildOptions            *types.BuildOptions
		name                    string
		secretTempFolder        string
		expectedErr             bool
		expectedReplacedSecrets bool
	}{
		{
			name:             "valid secret format",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{fmt.Sprintf("id=mysecret,src=%s", localSrcFile.Name())},
			},
			expectedErr:             false,
			expectedReplacedSecrets: true,
		},
		{
			name:             "valid secret format, reorder the fields",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{fmt.Sprintf("src=%s,id=mysecret", localSrcFile.Name())},
			},
			expectedErr:             false,
			expectedReplacedSecrets: true,
		},
		{
			name:             "valid secret format, only id",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{"id=mysecret"},
			},
			expectedErr: false,
		},
		{
			name:             "valid secret format, only source",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{fmt.Sprintf("src=%s", localSrcFile.Name())},
			},
			expectedErr:             false,
			expectedReplacedSecrets: true,
		},
		{
			name:             "invalid secret, local file does not exist",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{"id=mysecret,src=/file/invalid"},
			},
			expectedErr:             true,
			expectedReplacedSecrets: false,
		},
		{
			name:             "invalid secret, no = found",
			fs:               fakeFs,
			secretTempFolder: t.TempDir(),
			buildOptions: &types.BuildOptions{
				Secrets: []string{"mysecret"},
			},
			expectedErr:             true,
			expectedReplacedSecrets: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := &SolveOptBuilder{
				fs: tt.fs,
				secretMgr: &secretManager{
					fs:               tt.fs,
					secretTempFolder: tt.secretTempFolder,
				},
			}
			initialSecrets := make([]string, len(tt.buildOptions.Secrets))
			copy(initialSecrets, tt.buildOptions.Secrets)
			err := b.replaceSecretsSourceEnvWithTempFile(tt.buildOptions)
			require.Truef(t, tt.expectedErr == (err != nil), "not expected error")

			if tt.expectedReplacedSecrets {
				require.NotEqualValues(t, initialSecrets, tt.buildOptions.Secrets)
			} else {
				require.EqualValues(t, initialSecrets, tt.buildOptions.Secrets)
			}
		})
	}
}

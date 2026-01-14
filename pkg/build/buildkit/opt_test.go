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
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/config"
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
				fs:               tt.fs,
				secretTempFolder: tt.secretTempFolder,
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

func Test_getSecretTempFolder(t *testing.T) {
	// Set up a temporary okteto home for testing
	tempDir := t.TempDir()
	t.Setenv("OKTETO_HOME", tempDir)

	fakeFs := afero.NewMemMapFs()

	t.Run("creates UUID subfolder", func(t *testing.T) {
		folder, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)
		require.NotEmpty(t, folder)

		// Verify folder exists
		exists, err := afero.DirExists(fakeFs, folder)
		require.NoError(t, err)
		require.True(t, exists)

		// Verify it's under .secret/
		baseSecretFolder := filepath.Join(config.GetOktetoHome(), ".secret")
		require.Contains(t, folder, baseSecretFolder)

		// Verify the subfolder name is a valid UUID
		folderName := filepath.Base(folder)
		_, err = uuid.Parse(folderName)
		require.NoError(t, err, "folder name should be a valid UUID")
	})

	t.Run("creates unique folders for multiple calls", func(t *testing.T) {
		folder1, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)

		folder2, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)

		// Verify they are different
		require.NotEqual(t, folder1, folder2, "each call should create a unique folder")

		// Verify both folders exist
		exists1, err := afero.DirExists(fakeFs, folder1)
		require.NoError(t, err)
		require.True(t, exists1)

		exists2, err := afero.DirExists(fakeFs, folder2)
		require.NoError(t, err)
		require.True(t, exists2)
	})

	t.Run("has correct permissions", func(t *testing.T) {
		folder, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)

		info, err := fakeFs.Stat(folder)
		require.NoError(t, err)

		// Verify permissions are 0700 (owner only)
		require.Equal(t, PermissionsOwnerOnly, int(info.Mode().Perm()))
	})
}

func Test_SolveOptBuilder_Cleanup(t *testing.T) {
	// Set up a temporary okteto home for testing
	tempDir := t.TempDir()
	t.Setenv("OKTETO_HOME", tempDir)

	fakeFs := afero.NewMemMapFs()

	t.Run("removes secret folder when set", func(t *testing.T) {
		// Create a secret folder
		secretFolder, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)

		// Create a file in the folder
		testFile := filepath.Join(secretFolder, "test-secret.txt")
		err = afero.WriteFile(fakeFs, testFile, []byte("secret content"), 0600)
		require.NoError(t, err)

		// Verify folder and file exist
		exists, err := afero.DirExists(fakeFs, secretFolder)
		require.NoError(t, err)
		require.True(t, exists)

		fileExists, err := afero.Exists(fakeFs, testFile)
		require.NoError(t, err)
		require.True(t, fileExists)

		// Create builder with this secret folder
		builder := &SolveOptBuilder{
			fs:               fakeFs,
			secretTempFolder: secretFolder,
		}

		// Call cleanup
		err = builder.Cleanup()
		require.NoError(t, err)

		// Verify folder was removed
		exists, err = afero.DirExists(fakeFs, secretFolder)
		require.NoError(t, err)
		require.False(t, exists, "secret folder should be removed")
	})

	t.Run("does nothing when secretTempFolder is empty", func(t *testing.T) {
		builder := &SolveOptBuilder{
			fs:               fakeFs,
			secretTempFolder: "",
		}

		// Call cleanup - should not error
		err := builder.Cleanup()
		require.NoError(t, err)
	})

	t.Run("removes only the specific build folder, not siblings", func(t *testing.T) {
		// Create two secret folders
		secretFolder1, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)

		secretFolder2, err := getSecretTempFolder(fakeFs)
		require.NoError(t, err)

		// Create files in both folders
		testFile1 := filepath.Join(secretFolder1, "secret1.txt")
		err = afero.WriteFile(fakeFs, testFile1, []byte("secret 1"), 0600)
		require.NoError(t, err)

		testFile2 := filepath.Join(secretFolder2, "secret2.txt")
		err = afero.WriteFile(fakeFs, testFile2, []byte("secret 2"), 0600)
		require.NoError(t, err)

		// Create builder for first folder only
		builder := &SolveOptBuilder{
			fs:               fakeFs,
			secretTempFolder: secretFolder1,
		}

		// Cleanup first folder
		err = builder.Cleanup()
		require.NoError(t, err)

		// Verify first folder is removed
		exists1, err := afero.DirExists(fakeFs, secretFolder1)
		require.NoError(t, err)
		require.False(t, exists1, "first secret folder should be removed")

		// Verify second folder still exists
		exists2, err := afero.DirExists(fakeFs, secretFolder2)
		require.NoError(t, err)
		require.True(t, exists2, "second secret folder should still exist")

		fileExists2, err := afero.Exists(fakeFs, testFile2)
		require.NoError(t, err)
		require.True(t, fileExists2, "file in second folder should still exist")
	})
}

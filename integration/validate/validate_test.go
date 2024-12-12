//go:build integration
// +build integration

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

package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okteto/okteto/integration"
	"github.com/okteto/okteto/integration/commands"
	"github.com/stretchr/testify/require"
)

// testManifest represents a manifest file to be tested
type testManifest struct {
	path      string
	shouldErr bool
}

func TestValidateManifests(t *testing.T) {
	t.Parallel()

	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	testFiles, err := getTestManifests()
	require.NoError(t, err)

	for _, tf := range testFiles {
		fileName := filepath.Base(tf.path)
		t.Run(fileName, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()

			validateOptions := &commands.ValidateOptions{
				Workdir:      dir,
				ManifestPath: filepath.Join(tf.path),
				OktetoHome:   dir,
			}

			err = commands.RunOktetoValidate(oktetoPath, validateOptions)

			if tf.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateNonExistentFile(t *testing.T) {
	t.Parallel()
	oktetoPath, err := integration.GetOktetoPath()
	require.NoError(t, err)

	dir := t.TempDir()
	nonExistentPath := filepath.Join(dir, "non-existent.yml")

	validateOptions := &commands.ValidateOptions{
		Workdir:      dir,
		ManifestPath: nonExistentPath,
		OktetoHome:   dir,
	}

	err = commands.RunOktetoValidate(oktetoPath, validateOptions)
	require.Error(t, err)
}

// getTestManifests reads the testdata directory and returns a list of test manifests
func getTestManifests() ([]testManifest, error) {
	var manifests []testManifest

	testDataDir := filepath.Join("integration", "validate", "manifests")
	entries, err := os.ReadDir(testDataDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		relPath := filepath.Join(testDataDir, entry.Name())
		absPath, err := filepath.Abs(relPath)
		if err != nil {
			return nil, err
		}
		manifest := testManifest{
			path:      absPath,
			shouldErr: strings.HasPrefix(entry.Name(), "invalid-"),
		}
		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

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

package buildkit

import (
	"fmt"
	"path/filepath"

	"github.com/okteto/okteto/pkg/config"
	"github.com/spf13/afero"
)

// secretBuildManager manages the lifecycle of temporary secret folders for builds
type secretBuildManager interface {
	// GetSecretTempFolder returns the path to the temporary secret folder
	GetSecretTempFolder() string
	// Cleanup removes the temporary secret folder
	Cleanup() error
}

// secretManager implements secretBuildManager
type secretManager struct {
	fs               afero.Fs
	secretTempFolder string
}

// newSecretManager creates a new secret manager with a unique temp folder
func newSecretManager(fs afero.Fs) (*secretManager, error) {
	secretBaseFolder := filepath.Join(config.GetOktetoHome(), ".secret")

	if err := fs.MkdirAll(secretBaseFolder, PermissionsOwnerOnly); err != nil {
		return nil, fmt.Errorf("failed to create %s: %s", secretBaseFolder, err)
	}

	// Create a unique temp subdirectory
	secretTempFolder, err := afero.TempDir(fs, secretBaseFolder, "build-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp folder in %s: %s", secretBaseFolder, err)
	}

	return &secretManager{
		fs:               fs,
		secretTempFolder: secretTempFolder,
	}, nil
}

// GetSecretTempFolder returns the path to the temporary secret folder
func (sm *secretManager) GetSecretTempFolder() string {
	return sm.secretTempFolder
}

// Cleanup removes the temporary secret folder
func (sm *secretManager) Cleanup() error {
	if sm.secretTempFolder == "" {
		return nil
	}
	return sm.fs.RemoveAll(sm.secretTempFolder)
}

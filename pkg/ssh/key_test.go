// Copyright 2022 The Okteto Authors
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

package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/okteto/constants"
)

func TestKeyExists(t *testing.T) {
	dir := t.TempDir()

	defer func() {
		os.Unsetenv(constants.OktetoFolderEnvVar)
	}()

	os.Setenv(constants.OktetoFolderEnvVar, dir)

	if KeyExists() {
		t.Error("keys shouldn't exist in an empty directory")
	}

	f1, err := os.Create(filepath.Join(dir, publicKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f1.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if KeyExists() {
		t.Error("keys shouldn't exist when private key is missing")
	}

	f2, err := os.Create(filepath.Join(dir, privateKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f2.Close(); err != nil {
			t.Fatal(err)
		}
	})

	if !KeyExists() {
		t.Error("keys should exist")
	}
}

func TestGenerateKeys(t *testing.T) {
	dir := t.TempDir()

	defer func() {
		os.Unsetenv(constants.OktetoFolderEnvVar)
	}()

	os.Setenv(constants.OktetoFolderEnvVar, dir)
	public, private := getKeyPaths()
	if err := generateKeys(public, private, 128); err != nil {
		t.Error(err)
	}

	if !KeyExists() {
		t.Error("keys don't exist after creation")
	}

	if _, err := getSSHClientConfig(); err != nil {
		t.Errorf("failed to get ssh client configuration: %s", err)
	}
}

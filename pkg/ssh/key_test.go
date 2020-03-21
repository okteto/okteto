// Copyright 2020 The Okteto Authors
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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyExists(t *testing.T) {

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)

	os.Setenv("OKTETO_HOME", dir)
	if KeyExists() {
		t.Error("keys shouldn't exist in an empty directory")
	}

	if _, err := os.Create(filepath.Join(dir, ".okteto", publicKeyFile)); err != nil {
		t.Fatal(err)
	}

	if KeyExists() {
		t.Error("keys shouldn't exist when private key is missing")
	}

	if _, err := os.Create(filepath.Join(dir, ".okteto", privateKeyFile)); err != nil {
		t.Fatal(err)
	}

	if !KeyExists() {
		t.Error("keys should exist")
	}

}

func TestGenerateKeys(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(dir)
	os.Setenv("OKTETO_HOME", dir)

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

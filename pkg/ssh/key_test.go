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

package ssh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/okteto"
)

func TestKeyExists(t *testing.T) {
	dir := t.TempDir()

	t.Setenv(constants.OktetoFolderEnvVar, dir)

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

func TestConfiguredKeyExists(t *testing.T) {
	dir := t.TempDir()

	t.Setenv(constants.OktetoFolderEnvVar, dir)
	ctxStore := okteto.GetContextStore()
	ctxStore.Contexts["test"] = &okteto.Context{
		PrivateKeyFile: "",
		PublicKeyFile:  "",
	}
	okteto.CurrentStore = ctxStore
	okteto.CurrentStore.CurrentContext = "test"
	t.Cleanup(func() {
		okteto.CurrentStore = nil
	})
	if KeyExists() {
		t.Error("keys shouldn't exist in an empty directory")
	}
	f1, err := os.Create(filepath.Join(dir, "key.pub"))
	okteto.GetContext().PublicKeyFile = f1.Name()
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

	f2, err := os.Create(filepath.Join(dir, "key"))
	okteto.GetContext().PrivateKeyFile = f2.Name()

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

func TestConfiguredKeyOverridesDefault(t *testing.T) {
	dir := t.TempDir()

	t.Setenv(constants.OktetoFolderEnvVar, dir)
	f1, err := os.Create(filepath.Join(dir, "key.pub"))
	f2, err := os.Create(filepath.Join(dir, "key"))
	f3, err := os.Create(filepath.Join(dir, publicKeyFile))
	f4, err := os.Create(filepath.Join(dir, privateKeyFile))

	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f1.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f2.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f3.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f4.Close(); err != nil {
			t.Fatal(err)
		}

		okteto.CurrentStore = nil
	})
	ctxStore := okteto.GetContextStore()
	ctxStore.Contexts["test"] = &okteto.Context{
		PublicKeyFile:  f1.Name(),
		PrivateKeyFile: f2.Name(),
	}
	okteto.CurrentStore = ctxStore
	okteto.CurrentStore.CurrentContext = "test"

	pub, prv := getKeyPaths()

	if pub == f3.Name() || prv == f4.Name() {
		t.Error("key paths should not be the same as the default keys")
	}

	if pub != f1.Name() && prv != f2.Name() {
		t.Error("key paths should be the same as the configured keys")
	}

}

func Test_generate(t *testing.T) {
	dir := t.TempDir()

	t.Setenv(constants.OktetoFolderEnvVar, dir)
	public, private := getKeyPaths()
	if err := generate(public, private); err != nil {
		t.Error(err)
	}

	if !KeyExists() {
		t.Error("keys don't exist after creation")
	}

	if _, err := getSSHClientConfig(); err != nil {
		t.Errorf("failed to get ssh client configuration: %s", err)
	}
}

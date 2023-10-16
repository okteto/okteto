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

package syncthing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
)

func TestGetFiles(t *testing.T) {
	dir := t.TempDir()
	defer func() {
		os.Unsetenv(constants.OktetoFolderEnvVar)
	}()

	t.Setenv(constants.OktetoFolderEnvVar, dir)
	log := GetLogFile("test", "application")
	expected := filepath.Join(dir, "test", "application", "syncthing.log")

	if log != expected {
		t.Errorf("got %s, expected %s", log, expected)
	}

	info := getInfoFile("test", "application")
	expected = filepath.Join(dir, "test", "application", "syncthing.info")
	if info != expected {
		t.Errorf("got %s, expected %s", info, expected)
	}
}

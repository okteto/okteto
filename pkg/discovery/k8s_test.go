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

package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetK8sManifestFileWhenExists(t *testing.T) {

	wd := t.TempDir()
	fullpath := filepath.Join(wd, "k8s.yml")
	f, err := os.Create(fullpath)
	assert.NoError(t, err)
	defer f.Close()

	result, err := GetK8sManifestPath(wd)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(wd, "k8s.yml"), result)

}

func TestGetK8sManifestFolderWhenExists(t *testing.T) {
	wd := t.TempDir()
	fullpath := filepath.Join(wd, "manifests")
	assert.NoError(t, os.MkdirAll(filepath.Dir(fullpath), 0750))
	f, err := os.Create(fullpath)
	assert.NoError(t, err)
	defer f.Close()

	result, err := GetK8sManifestPath(wd)
	assert.NoError(t, err)
	assert.Equal(t, filepath.Join(wd, "manifests"), result)
}

func TestGetK8sPathWhenNotExists(t *testing.T) {
	wd := t.TempDir()
	result, err := GetK8sManifestPath(wd)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, ErrK8sManifestNotFound)
}

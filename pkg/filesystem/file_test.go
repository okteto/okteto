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

package filesystem

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()

	from := filepath.Join(dir, "from")
	to := filepath.Join(dir, "to")

	if err := CopyFile(from, to); err == nil {
		t.Error("failed to return error for missing file")
	}

	content := []byte("hello-world")
	if err := os.WriteFile(from, content, 0600); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(from, to); err != nil {
		t.Fatalf("failed to copy from %s to %s: %s", from, to, err)
	}

	copied, err := os.ReadFile(to)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, copied) {
		t.Fatalf("got %s, expected %s", string(content), string(copied))
	}

	if err := CopyFile(from, to); err != nil {
		t.Fatalf("failed to overwrite from %s to %s: %s", from, to, err)
	}

}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	p := filepath.Join(dir, "exists")
	if FileExists(p) {
		t.Errorf("fail to detect non-existing file")
	}

	if err := os.WriteFile(p, []byte("hello-world"), 0600); err != nil {
		t.Fatal(err)
	}

	if !FileExists(p) {
		t.Errorf("fail to detect existing file")
	}
}

func TestFileExistsAndNotDir_NotFound(t *testing.T) {
	fs := afero.NewMemMapFs()
	result := FileExistsAndNotDir("not_found.txt", fs)
	assert.Equal(t, false, result)
}

func TestFileExistsAndNotDir_IsDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := fs.Mkdir("a_directory", 0755)
	assert.NoError(t, err)

	result := FileExistsAndNotDir("a_directory", fs)
	assert.Equal(t, false, result)
}

func TestFileExistsAndNotDir_IsFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "a_file.txt", []byte("content"), 0644)
	assert.NoError(t, err)

	result := FileExistsAndNotDir("a_file.txt", fs)
	assert.Equal(t, true, result)
}

func TestFileExistsWithFilesystem(t *testing.T) {
	fs := afero.NewMemMapFs()

	exists := FileExistsWithFilesystem("random-file", fs)
	assert.Equal(t, false, exists)

	err := afero.WriteFile(fs, "random-file", []byte("content"), 0644)
	assert.NoError(t, err)
	exists = FileExistsWithFilesystem("random-file", fs)
	assert.Equal(t, true, exists)
}

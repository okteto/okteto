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
	"fmt"
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

// generateLongString generates a string of a given size in kilobytes
func generateLongString(kb int) string {
	size := kb * 1024
	var buffer bytes.Buffer
	buffer.Grow(size)
	for i := 0; i < size; i++ {
		buffer.WriteByte('a')
	}
	return buffer.String()
}

func TestGetLastNLines(t *testing.T) {
	const defaultChunkSize int64 = 1024

	type expected struct {
		err   error
		lines []string
	}

	tests := []struct {
		mockFs      func() afero.Fs
		name        string
		filePath    string
		expected    expected
		linesToRead int
		chunkSize   int64
	}{
		{
			name: "file not found",
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				return fs
			},
			filePath:    "not-found.txt",
			linesToRead: 10,
			chunkSize:   defaultChunkSize,
			expected: expected{
				lines: nil,
				err:   fmt.Errorf("open not-found.txt: file does not exist"),
			},
		},
		{
			name: "file with less  lines than requested",
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				err := afero.WriteFile(fs, "file.txt", []byte("line1\nline2\nline3"), 0644)
				assert.NoError(t, err)
				return fs
			},
			filePath:    "file.txt",
			linesToRead: 10,
			chunkSize:   defaultChunkSize,
			expected: expected{
				lines: []string{"line1", "line2", "line3"},
				err:   nil,
			},
		},
		{
			name: "file with more lines than requested",
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				err := afero.WriteFile(fs, "file.txt", []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\nline11\nline12\nline13\nline14\nline15"), 0644)
				assert.NoError(t, err)
				return fs
			},
			filePath:    "file.txt",
			linesToRead: 10,
			chunkSize:   defaultChunkSize,
			expected: expected{
				lines: []string{"line6", "line7", "line8", "line9", "line10", "line11", "line12", "line13", "line14", "line15"},
				err:   nil,
			},
		},
		{
			name: "file with 0 lines",
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				err := afero.WriteFile(fs, "file.txt", []byte(""), 0644)
				assert.NoError(t, err)
				return fs
			},
			filePath:    "file.txt",
			linesToRead: 10,
			chunkSize:   defaultChunkSize,
			expected:    expected{},
		},
		{
			name: "file with lines above the max chunk size",
			mockFs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				err := afero.WriteFile(fs, "file.txt", []byte(generateLongString(3)), 0644)
				assert.NoError(t, err)
				return fs
			},
			filePath:    "file.txt",
			linesToRead: 1,
			chunkSize:   defaultChunkSize,
			expected: expected{
				lines: []string{generateLongString(1)},
				err:   nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.mockFs()
			got, err := GetLastNLines(fs, tt.filePath, tt.linesToRead, tt.chunkSize)
			if tt.expected.err != nil {
				assert.Equal(t, tt.expected.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected.lines, got)
		})
	}
}

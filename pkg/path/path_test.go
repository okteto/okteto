package path

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetRelativePathFromCWD(t *testing.T) {
	root := t.TempDir()
	var tests = []struct {
		name         string
		path         string
		cwd          string
		expectedPath string
		expectedErr  bool
	}{
		{
			name:         "inside .okteto folder",
			path:         filepath.Join(root, ".okteto", "okteto.yml"),
			cwd:          root,
			expectedPath: ".okteto/okteto.yml",
		},
		{
			name:         "one path ahead - cwd is folder",
			path:         filepath.Join(root, "test", "okteto.yml"),
			cwd:          filepath.Join(root, "test"),
			expectedPath: "okteto.yml",
		},
		{
			name:         "one path ahead - cwd is root",
			path:         filepath.Join(root, "test", "okteto.yml"),
			cwd:          root,
			expectedPath: "test/okteto.yml",
		},
		{
			name:         "one path ahead not abs - cwd is root",
			path:         "test/okteto.yml",
			cwd:          root,
			expectedPath: "test/okteto.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			res, err := GetRelativePathFromCWD(tt.cwd, tt.path)
			if tt.expectedErr && err == nil {
				t.Fatal("expected err")
			}
			if !tt.expectedErr && err != nil {
				t.Fatalf("not expected error, got %v", err)
			}

			assert.Equal(t, tt.expectedPath, res)

		})
	}
}

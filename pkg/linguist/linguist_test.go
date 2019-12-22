package linguist

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessDirectory(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		files []string
	}{
		{
			name:  "gradle",
			want:  gradle,
			files: []string{"build.gradle", "main.java"},
		},
		{
			name:  "maven",
			want:  maven,
			files: []string{"pom.xml", "main.java"},
		},
		{
			name:  "none",
			want:  maven,
			files: []string{"main.java"},
		},
		{
			name:  "golang",
			want:  golang,
			files: []string{"main.go", "server.go"},
		},
		{
			name:  "python",
			want:  python,
			files: []string{"api.py"},
		},
		{
			name:  "javascript",
			want:  javascript,
			files: []string{"Package.json", "index.js"},
		},
		{
			name:  "ruby",
			want:  ruby,
			files: []string{"Gemfile", "Rakefile", "application_controller.rb"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}

			defer os.RemoveAll(tmp)

			for _, f := range tt.files {
				if _, err := os.Create(filepath.Join(tmp, f)); err != nil {
					t.Fatal(err)
				}
			}

			got, err := ProcessDirectory(tmp)

			if err != nil {
				t.Fatal(err)
			}

			if got != tt.want {
				t.Errorf("ProcessDirectory() = %v, want %v", got, tt.want)
			}
		})
	}
}

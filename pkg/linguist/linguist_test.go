package linguist

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func Test_refineJavaChoice(t *testing.T) {
	tests := []struct {
		name      string
		want      string
		magicFile string
	}{
		{
			name:      "gradle",
			want:      gradle,
			magicFile: "build.gradle",
		},
		{
			name:      "maven",
			want:      maven,
			magicFile: "pom.xml",
		},
		{
			name:      "none",
			want:      maven,
			magicFile: "test.java",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}

			defer os.RemoveAll(tmp)

			if _, err := os.Create(filepath.Join(tmp, "main.java")); err != nil {
				t.Error(err)
			}

			if _, err := os.Create(filepath.Join(tmp, tt.magicFile)); err != nil {
				t.Error(err)
			}

			if got := refineJavaChoice(tmp); got != tt.want {
				t.Errorf("refineJavaChoice() = %v, want %v", got, tt.want)
			}
		})
	}
}

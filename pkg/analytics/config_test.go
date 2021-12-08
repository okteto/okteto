package analytics

import (
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func Test_Get(t *testing.T) {
	var tests = []struct {
		name             string
		currentAnalytics bool
		enabled          bool
		fileExits        bool
		want             bool
	}{
		{
			name:             "is-currentAnalytics-enabled",
			currentAnalytics: true,
			enabled:          true,
			want:             true,
		},
		{
			name:             "is-currentAnalytics-disabled",
			currentAnalytics: true,
			enabled:          false,
			want:             false,
		},
		{
			name:      "is-currentAnalytics-nil-file-not-exists",
			fileExits: false,
			want:      false,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-enabled",
			fileExits: true,
			enabled:   true,
			want:      true,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-disabled",
			fileExits: true,
			enabled:   false,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				os.RemoveAll(dir)
			}()

			os.Setenv(model.OktetoFolderEnvVar, dir)

			if !tt.currentAnalytics {
				currentAnalytics = nil
			} else {
				currentAnalytics = &Analytics{Enabled: tt.enabled}
			}

			if tt.fileExits {
				a := &Analytics{Enabled: tt.enabled}
				if err := a.save(); err != nil {
					t.Fatalf("analytics file wasn't created")
				}
			}

			if got := get().Enabled; got != tt.want {
				t.Errorf("After Init, got %v, want %v", got, tt.want)
			}

		})
	}

}

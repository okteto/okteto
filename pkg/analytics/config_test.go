package analytics

import (
	"os"
	"testing"
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

			os.Setenv("OKTETO_FOLDER", dir)

			if !tt.currentAnalytics {
				currentAnalytics = nil
			} else {
				currentAnalytics = &Analytics{Enabled: tt.enabled}
			}

			if tt.fileExits {
				a := &Analytics{Enabled: tt.enabled}
				a.save()
			}

			if got := get().Enabled; got != tt.want {
				t.Errorf("After Init, got %v, want %v", got, tt.want)
			}

		})
	}

}

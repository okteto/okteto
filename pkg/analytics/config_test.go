package analytics

import (
	"fmt"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func Test_Get(t *testing.T) {
	var tests = []struct {
		name             string
		currentAnalytics bool
		enabled          bool
		fileExits        bool
		context          *okteto.OktetoContext
		want             bool
	}{
		{
			name:             "is-currentAnalytics-enabled",
			currentAnalytics: true,
			enabled:          true,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "true",
			},
			want: true,
		},
		{
			name:             "is-currentAnalytics-enabled-is-context-disabled",
			currentAnalytics: true,
			enabled:          true,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "false",
			},
			want: false,
		},
		{
			name:             "is-currentAnalytics-disabled",
			currentAnalytics: true,
			enabled:          false,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "true",
			},
			want: false,
		},
		{
			name:      "is-currentAnalytics-nil-file-not-exists",
			fileExits: false,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "true",
			},
			want: false,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-enabled",
			fileExits: true,
			enabled:   true,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "true",
			},
			want: true,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-disabled",
			fileExits: true,
			enabled:   false,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "true",
			},
			want: false,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-disabled-is-context-disabled",
			fileExits: true,
			enabled:   false,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "false",
			},
			want: false,
		},
		{
			name:      "is-currentAnalytics-nil-file-exists-enabled-is-context-disabled",
			fileExits: true,
			enabled:   true,
			context: &okteto.OktetoContext{
				Name:             "test",
				TelemetryEnabled: "false",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println("******", tt.name)
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

			okteto.CurrentStore = &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": tt.context,
				},
			}

			if got := get().Enabled; got != tt.want {
				t.Errorf("After Init, got %v, want %v", got, tt.want)
			}

		})
	}

}

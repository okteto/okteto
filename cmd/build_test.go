package cmd

import (
	"os"
	"testing"
)

func Test_isManifestV2Enabled(t *testing.T) {
	tests := []struct {
		name       string
		envDefined bool
		envValue   string
		expected   bool
	}{
		{
			name:       "env-not-defined",
			envDefined: false,
			expected:   false,
		},
		{
			name:       "env-value-not-valid",
			envDefined: true,
			envValue:   "test",
			expected:   false,
		},
		{
			name:       "env-value-valid-true",
			envDefined: true,
			envValue:   "true",
			expected:   true,
		},
		{
			name:       "env-value-valid-false",
			envDefined: true,
			envValue:   "false",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("OKTETO_ENABLE_MANIFEST_V2")
			if tt.envDefined {
				err := os.Setenv("OKTETO_ENABLE_MANIFEST_V2", tt.envValue)
				if err != nil {
					t.Log(err)
				}
			}

			if result := isManifestV2Enabled(); result != tt.expected {
				t.Errorf("test failed, expected %v, result %v", tt.expected, result)
			}

		})
	}
}

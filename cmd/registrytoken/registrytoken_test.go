package registrytoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRegistryCredentialHelperCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected bool
	}{
		{
			name:     "registrytoken command with get action",
			input:    []string{"foo", "registrytoken", "get"},
			expected: true,
		},
		{
			name:     "registrytoken command without action",
			input:    []string{"bar", "registrytoken"},
			expected: false,
		},
		{
			name:     "registrytoken command with flag",
			input:    []string{"bar", "registrytoken", "--help"},
			expected: false,
		},
		{
			name:     "non registrytoken command",
			input:    []string{"bar", "namespaces", "list"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsRegistryCredentialHelperCommand(tt.input))
		})
	}
}

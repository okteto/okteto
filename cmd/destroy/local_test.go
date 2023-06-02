package destroy

import (
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_decodeConfigMapVariables(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []types.DeployVariable
	}{
		{
			name:     "empty variable",
			input:    "",
			expected: nil,
		},
		{
			name:     "error decoding variables",
			input:    "test",
			expected: nil,
		},
		{
			name:  "success decoding variables",
			input: "W3sibmFtZSI6InRlc3QiLCJ2YWx1ZSI6InZhbHVlIn1d",
			expected: []types.DeployVariable{
				{
					Name:  "test",
					Value: "value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := decodeConfigMapVariables(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

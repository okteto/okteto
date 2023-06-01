package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DecodeStringToDeployVariable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []DeployVariable
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
			expected: []DeployVariable{
				{
					Name:  "test",
					Value: "value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := DecodeStringToDeployVariable(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

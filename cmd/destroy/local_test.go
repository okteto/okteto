package destroy

import (
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func Test_getVariablesFromCfgmap(t *testing.T) {
	tests := []struct {
		name     string
		cfgmap   *v1.ConfigMap
		expected []types.DeployVariable
	}{
		{
			name:     "nil cfgmap",
			cfgmap:   nil,
			expected: nil,
		},
		{
			name: "no variables at cfgmap",
			cfgmap: &v1.ConfigMap{
				Data: map[string]string{
					"test": "test",
				},
			},
			expected: nil,
		},
		{
			name: "error decoding variables",
			cfgmap: &v1.ConfigMap{
				Data: map[string]string{
					"variables": "test",
				},
			},
			expected: nil,
		},
		{
			name: "success decoding variables",
			cfgmap: &v1.ConfigMap{
				Data: map[string]string{
					"variables": "W3sibmFtZSI6InRlc3QiLCJ2YWx1ZSI6InZhbHVlIn1d",
				},
			},
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
			res := getVariablesFromCfgmap(tt.cfgmap)
			assert.Equal(t, tt.expected, res)
		})
	}
}

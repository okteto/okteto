package build

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestUnmarshalVolumeMounts(t *testing.T) {
	tests := []struct {
		input       string
		expected    *VolumeMounts
		name        string
		expectedErr bool
	}{
		{
			name:  "unmarshal err",
			input: "key: value",
			expected: &VolumeMounts{
				LocalPath:  "local path",
				RemotePath: "remote path",
			},
			expectedErr: true,
		},
		{
			name:  "unmarshal stack volume parts",
			input: `one:second`,
			expected: &VolumeMounts{
				LocalPath:  "one",
				RemotePath: "second",
			},
			expectedErr: false,
		},
		{
			name:  "unmarshal stack volume parts remote",
			input: `one`,
			expected: &VolumeMounts{
				RemotePath: "one",
			},
			expectedErr: false,
		},
		{
			name:  "error unmarshal stack volume parts overflow",
			input: `one:second:third:fourth`,
			expected: &VolumeMounts{
				RemotePath: "one",
			},
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &VolumeMounts{}
			err := yaml.Unmarshal([]byte(tt.input), out)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, out)
			}

		})
	}
}

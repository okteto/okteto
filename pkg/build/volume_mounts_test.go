// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package build

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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

func TestMarshalVolumeMounts(t *testing.T) {
	input := &VolumeMounts{
		LocalPath:  "testLocal",
		RemotePath: "testRemote",
	}
	out, err := yaml.Marshal(input)
	require.NoError(t, err)
	require.Equal(t, "testRemote\n", string(out))
}

func TestVolumeMountsToString(t *testing.T) {
	vm1 := &VolumeMounts{
		LocalPath: "test",
	}
	result1 := vm1.ToString()
	require.Equal(t, "test:", result1)

	vm2 := &VolumeMounts{
		LocalPath:  "",
		RemotePath: "test",
	}

	result2 := vm2.ToString()
	require.Equal(t, "test", result2)
}

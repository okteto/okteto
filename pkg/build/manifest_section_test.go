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
	"bytes"
	"testing"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		input        *ManifestBuild
		name         string
		expectErr    bool
		expectedWarn string
	}{
		{
			name: "nil manifest info",
			input: &ManifestBuild{
				"test": nil,
			},
			expectErr: true,
		},
		{
			name: "one dependent cycle",
			input: &ManifestBuild{
				"test": &Info{
					DependsOn: DependsOn{
						"test",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "greater than one dependent cycle",
			input: &ManifestBuild{
				"testSvc": &Info{
					DependsOn: DependsOn{
						"anotherService",
					},
				},
				"anotherService": &Info{
					DependsOn: DependsOn{
						"testSvc",
					},
				},
			},
			expectErr: true,
		},
		{
			name: "repeated image names",
			input: &ManifestBuild{
				"testSvc": &Info{
					Image: "testImage",
				},
				"anotherService": &Info{
					Image: "testImage",
				},
			},
			expectErr:    false,
			expectedWarn: "The following images are repeated in the build section",
		},
		{
			name: "successful validation",
			input: &ManifestBuild{
				"testSvc": &Info{
					DependsOn: DependsOn{
						"anotherService",
					},
				},
			},
			expectErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := bytes.NewBuffer(nil)
			oktetoLog.SetOutput(b)
			err := tt.input.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Contains(t, b.String(), tt.expectedWarn)
		})
	}
}

func TestGetListDiff(t *testing.T) {
	tests := []struct {
		input struct {
			l1 []string
			l2 []string
		}
		name     string
		expected []string
	}{
		{
			name: "l1 greater than l2",
			input: struct {
				l1 []string
				l2 []string
			}{
				l1: []string{"a", "b"},
				l2: []string{"a", "b", "c"},
			},
			expected: []string{"c"},
		},
		{
			name: "l2 greater than l1",
			input: struct {
				l1 []string
				l2 []string
			}{
				l1: []string{"a", "b", "c", "d"},
				l2: []string{"a", "b", "c"},
			},
			expected: []string{"d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getListDiff(tt.input.l1, tt.input.l2)
			require.ElementsMatch(t, result, tt.expected)
		})
	}
}

func TestGetSvcsToBuildFromListf(t *testing.T) {
	mb := &ManifestBuild{
		"a": &Info{
			DependsOn: DependsOn{
				"d",
			},
		},
	}
	inputList := []string{"a", "b", "c"}
	expectedList := []string{"a", "b", "c", "d"}
	result := mb.GetSvcsToBuildFromList(inputList)
	require.ElementsMatch(t, result, expectedList)
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		input    ManifestBuild
		expected bool
	}{
		{
			input:    ManifestBuild{},
			expected: true,
		},
		{
			input: ManifestBuild{
				"testSvc": &Info{
					DependsOn: DependsOn{
						"anotherService",
					},
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := tt.input.IsEmpty()
			require.Equal(t, tt.expected, result)
		})
	}
}

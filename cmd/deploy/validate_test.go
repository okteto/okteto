// Copyright 2022 The Okteto Authors
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

package deploy

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_setEnvVars(t *testing.T) {
	var tests = []struct {
		name            string
		variables       []string
		expectedErr     bool
		expectedEnvVars []envKeyValue
	}{
		{
			name:        "correct separator",
			variables:   []string{"NAME=test"},
			expectedErr: false,
			expectedEnvVars: []envKeyValue{
				{
					key:   "Name",
					value: "test",
				},
			},
		},
		{
			name:            "bad separator",
			variables:       []string{"NAME:test"},
			expectedErr:     true,
			expectedEnvVars: []envKeyValue{},
		},
		{
			name:            "bad count",
			variables:       []string{"=foo"},
			expectedErr:     true,
			expectedEnvVars: []envKeyValue{},
		},
		{
			name:            "one bad",
			variables:       []string{"first=one", "second=two", "third:three"},
			expectedErr:     true,
			expectedEnvVars: []envKeyValue{},
		},
		{
			name:        "multiples",
			variables:   []string{"first=one", "second=two", "third=three"},
			expectedErr: false,
			expectedEnvVars: []envKeyValue{
				{
					key:   "first",
					value: "one",
				},
				{
					key:   "second",
					value: "two",
				},
				{
					key:   "third",
					value: "three",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := setEnvVars(tt.variables)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, each := range tt.expectedEnvVars {
					assert.Equal(t, each.value, os.Getenv(each.key))
				}
			}
		})
	}
}

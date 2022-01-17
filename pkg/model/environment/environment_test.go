// Copyright 2021 The Okteto Authors
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

package environment

import (
	"os"
	"testing"
)

func Test_ExpandEnv(t *testing.T) {
	os.Setenv("BAR", "bar")
	tests := []struct {
		name   string
		value  string
		result string
	}{
		{
			name:   "no-var",
			value:  "value",
			result: "value",
		},
		{
			name:   "var",
			value:  "value-${BAR}-value",
			result: "value-bar-value",
		},
		{
			name:   "default",
			value:  "value-${FOO:-foo}-value",
			result: "value-foo-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExpandEnv(tt.value)
			if err != nil {
				t.Errorf("error in test '%s': %s", tt.name, err.Error())
			}
			if result != tt.result {
				t.Errorf("error in test '%s': '%s', expected: '%s'", tt.name, result, tt.result)
			}
		})
	}
}

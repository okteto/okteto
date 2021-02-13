// Copyright 2020 The Okteto Authors
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

package status

import (
	"testing"
)

func Test_computeProgress(t *testing.T) {
	var tests = []struct {
		name     string
		local    float64
		remote   float64
		expected float64
	}{
		{
			name:     "both-100",
			local:    100,
			remote:   100,
			expected: 100,
		},
		{
			name:     "local-100",
			local:    100,
			remote:   30,
			expected: 30,
		},
		{
			name:     "remote-100",
			local:    30,
			remote:   100,
			expected: 30,
		},
		{
			name:     "none-100",
			local:    30,
			remote:   50,
			expected: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeProgress(tt.local, tt.remote)
			if result != tt.expected {
				t.Fatalf("Test '%s' failed: expected %.f got %.f", tt.name, tt.expected, result)
			}
		})
	}
}

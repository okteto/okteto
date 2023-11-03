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

package utils

import (
	"testing"
)

func Test_renderProgressBar(t *testing.T) {
	var tests = []struct {
		expected string
		name     string
		progress float64
	}{
		{
			expected: "[__________________________________________________]   0%",
			progress: 0.0,
			name:     "no-progress",
		},
		{
			expected: "[-------------->___________________________________]  30%",
			progress: 30.0,
			name:     "progress-30",
		},
		{
			expected: "[--------------------------------->________________]  68%",
			progress: 68.5,
			name:     "progress-68.5",
		},
		{
			expected: "[--------------------------------------------------] 100%",
			progress: 100.0,
			name:     "progress-100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := RenderProgressBar("", tt.progress, 0.5)
			if tt.expected != actual {
				t.Errorf("\nexpected:\n%s\ngot:\n%s", tt.expected, actual)
			}
		})
	}

}

func Test_renderProgressBarFuzz(_ *testing.T) {
	for i := 0.0; i < 100.0; i = i + 0.01 {
		RenderProgressBar("", i, 0.35)
	}
}

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

package cmd

import "testing"

func Test_getDeploymentName(t *testing.T) {
	var tests = []struct {
		name       string
		deployment string
		expected   string
	}{
		{name: "all lower case", deployment: "lowercase", expected: "lowercase"},
		{name: "with some lower case", deployment: "lowerCase", expected: "lowercase"},
		{name: "upper case", deployment: "UpperCase", expected: "uppercase"},
		{name: "valid symbols", deployment: "getting-started.test", expected: "getting-started-test"},
		{name: "invalid symbols", deployment: "getting_$#started", expected: "getting-started"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getDeploymentName(tt.deployment)
			if actual != tt.expected {
				t.Errorf("got: %s expected: %s", actual, tt.expected)
			}
		})
	}
}

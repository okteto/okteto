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

package okteto

import (
	"fmt"
	"strings"
	"testing"
)

func Test_membersToString(t *testing.T) {
	expected := ""
	got := membersToString([]string{})

	if expected != got {
		t.Errorf("got %s, expected %s", got, expected)
	}

	expected = `"cindylopez"`
	got = membersToString([]string{"cindylopez"})

	if expected != got {
		t.Errorf("got %s, expected %s", got, expected)
	}

	expected = `"cindylopez","otheruser"`
	got = membersToString([]string{"cindylopez", "otheruser"})

	if expected != got {
		t.Errorf("got %s, expected %s", got, expected)
	}
}

func Test_validateNamespaceName(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		expectedError bool
		errorMessage  string
	}{
		{
			name:          "ok-namespace-starts-with-letter",
			namespace:     "argo-yournamespace",
			expectedError: false,
			errorMessage:  "",
		},
		{
			name:          "ok-namespace-starts-with-number",
			namespace:     "1-argo-yournamespace",
			expectedError: false,
			errorMessage:  "",
		},
		{
			name:          "wrong-namespace-starts-with-unsupported-character",
			namespace:     "-argo-yournamespace",
			expectedError: true,
			errorMessage:  "Malformed namespace name",
		},
		{
			name:          "wrong-namespace-unsupported-character",
			namespace:     "argo/test-yournamespace",
			expectedError: true,
			errorMessage:  "Malformed namespace name",
		},
		{
			name:          "wrong-namespace-exceeded-char-limit",
			namespace:     fmt.Sprintf("%s-yournamespace", strings.Repeat("test", 20)),
			expectedError: true,
			errorMessage:  "Exceeded number of character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespace(tt.namespace, "namespace")
			if err != nil && !tt.expectedError {
				t.Errorf("Expected error but no error found")
			}
			if err != nil && !tt.expectedError {
				if !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected %s, but got %s", tt.errorMessage, err.Error())
				}
			}

		})
	}
}

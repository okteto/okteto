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

package okteto

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func Test_getResourceFullName(t *testing.T) {
	tests := []struct {
		name    string
		kindArg string
		nameArg string
		result  string
	}{
		{
			name:    "deployment",
			kindArg: model.Deployment,
			nameArg: "name",
			result:  "deployment/name",
		},
		{
			name:    "statefulset",
			kindArg: model.StatefulSet,
			nameArg: "name",
			result:  "statefulset/name",
		},
		{
			name:    "job",
			kindArg: model.Job,
			nameArg: "name",
			result:  "job/name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getResourceFullName(tt.kindArg, tt.nameArg)
			if result != tt.result {
				t.Errorf("Test %s: expected %s, but got %s", tt.name, tt.result, result)
			}
		})
	}
}

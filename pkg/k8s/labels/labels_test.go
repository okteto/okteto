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

package labels

import (
	"reflect"
	"testing"
)

func TestTransformLabelsToSelector(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected []string
	}{
		{
			"single-label",
			map[string]string{"app": "db"},
			[]string{"app=db"},
		},
		{
			"multiple_labels",
			map[string]string{"app": "db", "stage": "prod"},
			[]string{"app=db,stage=prod", "stage=prod,app=db"},
		},
		{
			"none_labels",
			map[string]string{},
			[]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			passed := true
			result := TransformLabelsToSelector(tt.labels)
			for _, possibleSolution := range tt.expected {
				if reflect.DeepEqual(result, possibleSolution) {
					passed = true
				}
			}
			if !passed {
				t.Errorf("didn't transformed correctly. Actual %+v, Expected one of %+v", result, tt.expected)
			}

		})
	}
}

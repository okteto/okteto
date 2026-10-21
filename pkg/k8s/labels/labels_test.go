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

package labels

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestSetInMetadata(t *testing.T) {
	tests := []struct {
		om       *metav1.ObjectMeta
		expected map[string]string
		name     string
		key      string
		value    string
	}{
		{
			name:     "empty-labels",
			om:       &metav1.ObjectMeta{},
			key:      "key",
			value:    "value",
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "update-label",
			om:       &metav1.ObjectMeta{Labels: map[string]string{"key": "old"}},
			key:      "key",
			value:    "new",
			expected: map[string]string{"key": "new"},
		},
		{
			name:     "new-label",
			om:       &metav1.ObjectMeta{Labels: map[string]string{"key1": "value1"}},
			key:      "key2",
			value:    "value2",
			expected: map[string]string{"key1": "value1", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetInMetadata(tt.om, tt.key, tt.value)
			if !reflect.DeepEqual(tt.om.Labels, tt.expected) {
				t.Errorf("error in %s: Actual %+v, Expected one of %+v", tt.name, tt.om.Labels, tt.expected)
			}
		})
	}
}

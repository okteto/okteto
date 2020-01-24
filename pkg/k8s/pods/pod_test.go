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

package pods

import (
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var ns = &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

func TestGetBySelector(t *testing.T) {
	var tests = []struct {
		name        string
		selector    map[string]string
		pods        []apiv1.Pod
		expectError bool
	}{
		{
			name:        "empty-selector",
			expectError: true,
			pods: []apiv1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "empty-selector",
						Namespace: "test",
						Labels:    map[string]string{"app": "api"},
					},
				},
			},
		},
		{
			name:     "single-selector",
			selector: map[string]string{"app": "api"},
			pods: []apiv1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "single-selector",
						Namespace: "test",
						Labels:    map[string]string{"app": "api"},
					},
				},
			},
		},
		{
			name:     "multiple-selector-multiple-labels",
			selector: map[string]string{"app": "api", "component": "web"},
			pods: []apiv1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-selector-multiple-labels",
						Namespace: "test",
						Labels:    map[string]string{"app": "api", "component": "web"},
					},
				},
			},
		},
		{
			name:        "no-match",
			expectError: true,
			selector:    map[string]string{"app": "api"},
			pods: []apiv1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-selector-multiple-labels",
						Namespace: "test",
						Labels:    map[string]string{"app": "queue"},
					},
				},
			},
		},
		{
			name:     "multiple-matches",
			selector: map[string]string{"app": "api"},
			pods: []apiv1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-matches",
						Namespace: "test",
						Labels:    map[string]string{"app": "api"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multiple-matches-second",
						Namespace: "test",
						Labels:    map[string]string{"app": "api"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(ns)
			for _, p := range tt.pods {
				if err := c.Tracker().Add(&p); err != nil {
					t.Fatal(err)
				}
			}

			r, err := GetBySelector("test", tt.selector, c)
			if err != nil {
				if !tt.expectError {
					t.Fatal(err)
				}

				return
			}

			if r.GetName() != tt.name {
				t.Fatalf("expected %s but got %s", tt.name, r.GetName())
			}
		})
	}
}

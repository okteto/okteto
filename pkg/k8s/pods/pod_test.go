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
	"context"
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

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewSimpleClientset(ns)
			for _, p := range tt.pods {
				if err := c.Tracker().Add(&p); err != nil {
					t.Fatal(err)
				}
			}

			r, err := GetBySelector(ctx, "test", tt.selector, c)
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

func Test_parseUserID(t *testing.T) {
	var tests = []struct {
		name   string
		output string
		result int64
	}{
		{
			name:   "single-line-ok",
			output: "USER:300",
			result: 300,
		},
		{
			name:   "double-line-ok",
			output: "USER:300\nline2",
			result: 300,
		},
		{
			name:   "no-lines-ko",
			output: "",
			result: -1,
		},
		{
			name:   "no-user-ko",
			output: "other",
			result: -1,
		},
		{
			name:   "no-parts-ko",
			output: "USER:100:100",
			result: -1,
		},
		{
			name:   "no-integer-ko",
			output: "USER:no",
			result: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseUserID(tt.output)
			if result != tt.result {
				t.Fatalf("error in test '%s': expected %d but got %d", tt.name, tt.result, result)
			}
		})
	}
}

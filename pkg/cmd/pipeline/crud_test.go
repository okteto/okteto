// Copyright 2024 The Okteto Authors
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

package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_IsDeployedPipeline(t *testing.T) {
	ctx := context.Background()
	namespace := "test"

	var tests = []struct {
		name     string
		status   string
		create   bool
		expected bool
	}{
		{
			name:     "already deployed",
			status:   DeployedStatus,
			create:   true,
			expected: true,
		},
		{
			name:     "progressing",
			status:   ProgressingStatus,
			create:   true,
			expected: true,
		},
		{
			name:     "error",
			status:   ErrorStatus,
			create:   true,
			expected: false,
		},
		{
			name:     "not found",
			create:   false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fakeClient *fake.Clientset
			cmap := &apiv1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      TranslatePipelineName("test"),
					Namespace: namespace,
					Labels:    map[string]string{},
				},
				Data: map[string]string{
					statusField: tt.status,
				},
			}
			if tt.create {
				fakeClient = fake.NewSimpleClientset(cmap)
			} else {
				fakeClient = fake.NewSimpleClientset()
			}
			result := IsDeployed(ctx, "test", namespace, fakeClient)
			assert.Equal(t, tt.expected, result)
		})
	}
}

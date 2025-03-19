// Copyright 2025 The Okteto Authors
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
package endpoints

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestGetter_GetByName(t *testing.T) {
	ctx := context.TODO()
	endpointName := "test-endpoint"
	namespace := "test-namespace"

	mockEndpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      endpointName,
			Namespace: namespace,
		},
	}

	tests := []struct {
		name           string
		endpointExists bool
		expectedError  bool
	}{
		{
			name:           "Endpoint exists",
			endpointExists: true,
			expectedError:  false,
		},
		{
			name:           "Endpoint does not exist",
			endpointExists: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()

			if tt.endpointExists {
				_, _ = fakeClient.CoreV1().Endpoints(namespace).Create(ctx, mockEndpoints, metav1.CreateOptions{})
			}

			getter := NewGetter(fakeClient)
			got, err := getter.GetByName(ctx, endpointName, namespace)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, endpointName, got.Name)
				assert.Equal(t, namespace, got.Namespace)
			}
		})
	}
}

func TestGetter_GetByName_ClientError(t *testing.T) {
	ctx := context.TODO()
	endpointName := "test-endpoint"
	namespace := "test-namespace"

	expectedError := errors.New("client error")

	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("get", "endpoints", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, expectedError
	})

	getter := NewGetter(fakeClient)
	got, err := getter.GetByName(ctx, endpointName, namespace)

	assert.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), expectedError.Error())
}

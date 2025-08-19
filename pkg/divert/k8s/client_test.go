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

package k8s_test

import (
	"context"
	"testing"

	k8sdivert "github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/divert/k8s/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDivertClient_CRUD(t *testing.T) {
	namespace := "test-namespace"

	// Create fake client
	fakeClient := fake.NewFakeDivertV1(fake.PossibleDivertErrors{})
	client := fakeClient.Diverts(namespace)

	// Test Create
	divert := &k8sdivert.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert",
			Namespace: namespace,
		},
		Spec: k8sdivert.DivertSpec{
			SharedNamespace: "shared-ns",
			Service:         "test-service",
			DivertKey:       "test-key",
		},
	}

	created, err := client.Create(context.Background(), divert, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create divert")
	require.Equal(t, "test-divert", created.Name)

	// Test Get
	retrieved, err := client.Get(context.Background(), "test-divert", metav1.GetOptions{})
	require.NoError(t, err, "Failed to get divert")
	require.Equal(t, "test-divert", retrieved.Name)
	require.Equal(t, "test-service", retrieved.Spec.Service)

	// Test Update
	retrieved.Spec.Service = "updated-service"
	updated, err := client.Update(context.Background(), retrieved)
	require.NoError(t, err, "Failed to update divert")
	require.Equal(t, "updated-service", updated.Spec.Service)

	// Test List
	list, err := client.List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err, "Failed to list diverts")
	require.Len(t, list.Items, 1)

	// Test Delete
	err = client.Delete(context.Background(), "test-divert", metav1.DeleteOptions{})
	require.NoError(t, err, "Failed to delete divert")
}

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

package divert

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/divert/k8s/fake"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateOrUpdate_Create(t *testing.T) {
	ctx := context.Background()
	client := fake.NewFakeDivertV1(fake.PossibleDivertErrors{})
	manager := NewManager(client)

	d := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert",
			Namespace: "default",
		},
		Spec: k8s.DivertSpec{Service: "my-service"},
	}

	err := manager.CreateOrUpdate(ctx, d)
	require.NoError(t, err, "expected no error on create")

	created, err := client.Diverts("default").Get(ctx, "test-divert", metav1.GetOptions{})
	require.NoError(t, err, "expected to get created divert")

	require.NotEmpty(t, created.Annotations[constants.LastUpdatedAnnotation], "expected LastUpdatedAnnotation to be set")
	require.Equal(t, "my-service", created.Spec.Service, "expected spec Service to be 'my-service'")
}

func TestCreateOrUpdate_Update(t *testing.T) {
	ctx := context.Background()
	pre := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert",
			Namespace: "default",
		},
		Spec: k8s.DivertSpec{Service: "old-service"},
	}
	client := fake.NewFakeDivertV1(fake.PossibleDivertErrors{}, pre)
	manager := NewManager(client)

	update := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert",
			Namespace: "default",
		},
		Spec: k8s.DivertSpec{Service: "new-service"},
	}

	err := manager.CreateOrUpdate(ctx, update)
	require.NoError(t, err, "expected no error on update")

	updated, err := client.Diverts("default").Get(ctx, "test-divert", metav1.GetOptions{})
	require.NoError(t, err, "expected to get updated divert")

	require.NotEmpty(t, updated.Annotations[constants.LastUpdatedAnnotation], "expected LastUpdatedAnnotation to be set")
	require.Equal(t, "new-service", updated.Spec.Service, "expected spec Service to be 'new-service'")
}

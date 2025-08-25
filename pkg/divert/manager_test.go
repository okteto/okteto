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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
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

func TestList(t *testing.T) {
	ctx := context.Background()

	divert1 := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert-1",
			Namespace: "default",
		},
		Spec: k8s.DivertSpec{Service: "service-1"},
	}

	divert2 := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert-2",
			Namespace: "default",
		},
		Spec: k8s.DivertSpec{Service: "service-2"},
	}

	client := fake.NewFakeDivertV1(fake.PossibleDivertErrors{}, divert1, divert2)
	manager := NewManager(client)
	expected := []*k8s.Divert{divert1, divert2}

	diverts, err := manager.List(ctx, "default")
	require.NoError(t, err, "expected no error on list")
	require.ElementsMatch(t, expected, diverts)
}

func TestList_Error(t *testing.T) {
	ctx := context.Background()

	errorClient := fake.NewFakeDivertV1(fake.PossibleDivertErrors{
		ListErr: assert.AnError,
	})
	errorManager := NewManager(errorClient)

	_, err := errorManager.List(ctx, "default")
	require.Error(t, err, "expected error when List returns error")
}

func TestDelete_Success(t *testing.T) {
	ctx := context.Background()

	divert := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert",
			Namespace: "default",
		},
		Spec: k8s.DivertSpec{Service: "my-service"},
	}

	client := fake.NewFakeDivertV1(fake.PossibleDivertErrors{}, divert)
	manager := NewManager(client)

	err := manager.Delete(ctx, "test-divert", "default")
	require.NoError(t, err, "expected no error on delete")

	_, err = client.Diverts("default").Get(ctx, "test-divert", metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err), "expected divert to be deleted")
}

func TestDelete_NotFound(t *testing.T) {
	ctx := context.Background()

	client := fake.NewFakeDivertV1(fake.PossibleDivertErrors{})
	manager := NewManager(client)

	err := manager.Delete(ctx, "non-existent", "default")
	require.NoError(t, err, "expected no error when divert doesn't exist")
}

func TestDelete_Error(t *testing.T) {
	ctx := context.Background()

	errorClient := fake.NewFakeDivertV1(fake.PossibleDivertErrors{
		DeleteErr: assert.AnError,
	})
	manager := NewManager(errorClient)

	err := manager.Delete(ctx, "test-divert", "default")
	require.Error(t, err, "expected error when Delete returns error")
	require.Equal(t, assert.AnError, err, "expected specific error to be returned")
}

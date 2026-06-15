// Copyright 2026 The Okteto Authors
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

package namespaces

import (
	"context"
	"errors"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func setupUIDResolverContext(t *testing.T) func() {
	t.Helper()
	prev := okteto.CurrentStore
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {Cfg: clientcmdapi.NewConfig()},
		},
	}
	return func() { okteto.CurrentStore = prev }
}

func TestUIDResolver_FetchesFromK8s(t *testing.T) {
	teardown := setupUIDResolverContext(t)
	defer teardown()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-ns",
			UID:  types.UID("abc-123"),
		},
	}
	provider := test.NewFakeK8sProvider(ns)
	r := NewUIDResolver(provider)

	uid, err := r.GetNamespaceUID(context.Background(), "my-ns")
	require.NoError(t, err)
	require.Equal(t, "abc-123", uid)
}

func TestUIDResolver_CacheHit(t *testing.T) {
	teardown := setupUIDResolverContext(t)
	defer teardown()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-ns",
			UID:  types.UID("abc-123"),
		},
	}
	provider := test.NewFakeK8sProvider(ns)
	r := NewUIDResolver(provider)

	// First call populates cache
	_, err := r.GetNamespaceUID(context.Background(), "my-ns")
	require.NoError(t, err)

	// Break the provider so a second real K8s call would fail
	provider.ErrProvide = errors.New("should not be called")

	// Second call must return cached value without hitting provider
	uid, err := r.GetNamespaceUID(context.Background(), "my-ns")
	require.NoError(t, err)
	require.Equal(t, "abc-123", uid)
}

func TestUIDResolver_NilCfg(t *testing.T) {
	prev := okteto.CurrentStore
	defer func() { okteto.CurrentStore = prev }()

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {Cfg: nil},
		},
	}

	r := NewUIDResolver(test.NewFakeK8sProvider())
	_, err := r.GetNamespaceUID(context.Background(), "my-ns")
	require.ErrorContains(t, err, "k8s config not available")
}

func TestUIDResolver_ProvideError(t *testing.T) {
	teardown := setupUIDResolverContext(t)
	defer teardown()

	provider := test.NewFakeK8sProvider()
	provider.ErrProvide = errors.New("k8s unavailable")

	r := NewUIDResolver(provider)
	_, err := r.GetNamespaceUID(context.Background(), "my-ns")
	require.ErrorContains(t, err, "providing k8s client")
}

func TestUIDResolver_NamespaceNotFound(t *testing.T) {
	teardown := setupUIDResolverContext(t)
	defer teardown()

	// Provider with no namespace objects — Get will return NotFound
	provider := test.NewFakeK8sProvider()
	r := NewUIDResolver(provider)

	_, err := r.GetNamespaceUID(context.Background(), "nonexistent-ns")
	require.ErrorContains(t, err, "getting namespace")
}

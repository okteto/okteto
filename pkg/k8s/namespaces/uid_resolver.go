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
	"fmt"
	"sync"

	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/singleflight"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UIDResolver fetches Kubernetes namespace UIDs and caches results by namespace name.
type UIDResolver struct {
	provider okteto.K8sClientProvider
	cache    sync.Map // namespace name → UID string
	sg       singleflight.Group
}

// NewUIDResolver creates a UIDResolver backed by the given K8sClientProvider.
func NewUIDResolver(provider okteto.K8sClientProvider) *UIDResolver {
	return &UIDResolver{provider: provider}
}

// GetNamespaceUID returns the namespace UID, cached by name. Concurrent callers
// for the same namespace collapse into one lookup via singleflight (governed by
// the first caller's context). Errors are not cached, so later calls retry.
func (r *UIDResolver) GetNamespaceUID(ctx context.Context, namespace string) (string, error) {
	if uid, ok := r.cache.Load(namespace); ok {
		return uid.(string), nil
	}

	v, err, _ := r.sg.Do(namespace, func() (any, error) {
		if uid, ok := r.cache.Load(namespace); ok {
			return uid.(string), nil
		}

		cfg := okteto.GetContext().Cfg
		if cfg == nil {
			return "", fmt.Errorf("k8s config not available")
		}

		k8sClient, _, err := r.provider.Provide(cfg)
		if err != nil {
			return "", fmt.Errorf("providing k8s client: %w", err)
		}

		ns, err := k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("getting namespace %q: %w", namespace, err)
		}

		uid := string(ns.UID)
		r.cache.Store(namespace, uid)
		return uid, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

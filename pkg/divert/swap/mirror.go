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

package swap

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mirrorSharedServices makes the shared namespace's services resolvable from the developer's
// namespace.
//
// This is what makes a mid-chain divert usable. Once a request reaches the developer's copy
// of `api`, that copy carries on calling `catalog` — a name that only exists in the shared
// namespace. Without a mirror the call fails to resolve and the divert looks broken for a
// reason that has nothing to do with routing.
//
// Mirrors are ExternalName services, and they are deliberately *not* removed by teardown:
// two diverts into the same developer namespace share one set of mirrors, so tearing one
// down would break the other. They are inert records pointing at real shared services, and
// they carry the same auto-create annotation the nginx driver uses so that the developer's
// own deploy can recognise and replace them.
func (c *Client) mirrorSharedServices(ctx context.Context, opts UpOptions) error {
	shared, err := c.k8s.CoreV1().Services(opts.SharedNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing services in namespace %s: %w", opts.SharedNamespace, err)
	}

	existing, err := c.k8s.CoreV1().Services(opts.TargetNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error listing services in namespace %s: %w", opts.TargetNamespace, err)
	}

	developerHas := make(map[string]bool, len(existing.Items))
	for i := range existing.Items {
		developerHas[existing.Items[i].Name] = true
	}

	mirrored := 0
	for i := range shared.Items {
		svc := &shared.Items[i]
		if !shouldMirror(svc, opts.Service, developerHas) {
			continue
		}

		mirror := externalNameMirror(svc, opts.TargetNamespace)
		if _, err := c.k8s.CoreV1().Services(opts.TargetNamespace).Create(ctx, mirror, metav1.CreateOptions{}); err != nil {
			// The developer deploying the same service at the same moment is a race worth
			// losing: their own copy is the one that should win.
			if k8sErrors.IsAlreadyExists(err) {
				continue
			}
			return fmt.Errorf("error mirroring service %s into namespace %s: %w", svc.Name, opts.TargetNamespace, err)
		}

		mirrored++
	}

	c.logger.Infof("mirrored %d shared service(s) into namespace %s", mirrored, opts.TargetNamespace)
	return nil
}

func shouldMirror(svc *apiv1.Service, divertedService string, developerHas map[string]bool) bool {
	// Whatever the developer deployed themselves always wins.
	if developerHas[svc.Name] {
		return false
	}

	// Never mirror the service being diverted. The router forwards to that name in the
	// developer's namespace, and a mirror there would send it straight back to the shared
	// namespace and into the router again.
	if svc.Name == divertedService {
		return false
	}

	// Skip the baseline and anything else this package created.
	if _, ok := svc.Labels[managedServiceLabel]; ok {
		return false
	}

	return true
}

func externalNameMirror(svc *apiv1.Service, targetNamespace string) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: targetNamespace,
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation:        "true",
				model.OktetoDivertedNamespaceAnnotation: svc.Namespace,
			},
		},
		Spec: apiv1.ServiceSpec{
			Type:         apiv1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.%s", svc.Name, svc.Namespace, clusterDomain),
		},
	}
}

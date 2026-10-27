// Copyright 2023 The Okteto Authors
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

package gatewayapi

import (
	"context"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// divertIngress applies the divert header annotation to an ingress deployed in the developer namespace
func (d *Driver) divertIngress(ctx context.Context, in *networkingv1.Ingress) error {
	if in.Annotations[model.OktetoDivertHeaderAnnotation] == in.Namespace {
		return nil
	}

	updatedIn := in.DeepCopy()
	if updatedIn.Annotations == nil {
		updatedIn.Annotations = map[string]string{}
	}
	updatedIn.Annotations[model.OktetoDivertHeaderAnnotation] = updatedIn.Namespace

	oktetoLog.Infof("updating ingress %s/%s", updatedIn.Namespace, updatedIn.Name)
	if _, err := d.client.NetworkingV1().Ingresses(d.namespace).Update(ctx, updatedIn, metav1.UpdateOptions{}); err != nil {
		if !k8sErrors.IsConflict(err) {
			return err
		}
		// the ingress was updated, refresh the cache
		updatedIn, err = d.client.NetworkingV1().Ingresses(d.namespace).Get(ctx, in.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}
	d.cache.developerIngresses[in.Name] = updatedIn
	return nil
}

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
	"encoding/json"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// divertIngress applies the divert header annotation to an ingress deployed in the developer namespace.
// It uses a merge patch so the API server applies the annotation over the latest version of the ingress
func (d *Driver) divertIngress(ctx context.Context, in *networkingv1.Ingress) error {
	if in.Annotations[model.OktetoDivertHeaderAnnotation] == in.Namespace {
		return nil
	}

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				model.OktetoDivertHeaderAnnotation: in.Namespace,
			},
		},
	})
	if err != nil {
		return err
	}

	oktetoLog.Infof("patching ingress %s/%s", in.Namespace, in.Name)
	updatedIn, err := d.client.NetworkingV1().Ingresses(d.namespace).Patch(ctx, in.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	d.cache.developerIngresses[in.Name] = updatedIn
	return nil
}

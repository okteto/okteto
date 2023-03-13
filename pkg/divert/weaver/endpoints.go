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

package weaver

import (
	"context"
	"reflect"

	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (d *Driver) divertEndpoints(ctx context.Context, name string) error {
	from := d.cache.divertServices[name]
	e, ok := d.cache.developerEndpoints[name]
	if !ok {
		newE := translateEndpoints(d.name, d.namespace, from)
		oktetoLog.Infof("creating endpoint %s/%s", newE.Namespace, newE.Name)
		if _, err := d.client.CoreV1().Endpoints(d.namespace).Create(ctx, newE, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
		}
		d.cache.developerEndpoints[name] = newE
		return nil
	}
	if e.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}
	updatedE := translateEndpoints(d.name, d.namespace, from)
	if isEqualEndpoints(e, updatedE) {
		return nil
	}
	oktetoLog.Infof("updating endpoints %s/%s", updatedE.Namespace, updatedE.Name)
	if _, err := d.client.CoreV1().Endpoints(d.namespace).Update(ctx, updatedE, metav1.UpdateOptions{}); err != nil {
		if !k8sErrors.IsConflict(err) {
			return err
		}
	}
	d.cache.developerEndpoints[name] = updatedE
	return nil
}

func translateEndpoints(name, namespace string, s *apiv1.Service) *apiv1.Endpoints {
	result := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   namespace,
			Labels:      s.Labels,
			Annotations: s.Annotations,
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: s.Spec.ClusterIP,
						TargetRef: &apiv1.ObjectReference{
							Kind:            "Service",
							Namespace:       s.Namespace,
							Name:            s.Name,
							UID:             s.UID,
							APIVersion:      "v1",
							ResourceVersion: s.ResourceVersion,
						},
					},
				},
				Ports: []apiv1.EndpointPort{},
			},
		},
	}
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(name))
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	delete(result.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	for _, p := range s.Spec.Ports {
		result.Subsets[0].Ports = append(
			result.Subsets[0].Ports,
			apiv1.EndpointPort{
				Name:        p.Name,
				Port:        p.Port,
				Protocol:    p.Protocol,
				AppProtocol: p.AppProtocol,
			},
		)
	}

	return result
}

func isEqualEndpoints(e1 *apiv1.Endpoints, e2 *apiv1.Endpoints) bool {
	return reflect.DeepEqual(e1.Subsets, e2.Subsets) && reflect.DeepEqual(e1.Labels, e2.Labels) && reflect.DeepEqual(e1.Annotations, e2.Annotations)
}

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

func (d *Driver) divertService(ctx context.Context, name string) error {
	from, ok := d.cache.divertServices[name]
	if !ok {
		oktetoLog.Infof("service %s not found: %s", name)
		return nil
	}
	s, ok := d.cache.developerServices[name]
	if !ok {
		newS := translateService(d.name, d.namespace, from)
		oktetoLog.Infof("creating service %s/%s", newS.Namespace, newS.Name)
		if _, err := d.client.CoreV1().Services(d.namespace).Create(ctx, newS, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
			// the service was created, refresh the cache
			newS, err = d.client.CoreV1().Services(d.namespace).Get(ctx, newS.Name, metav1.GetOptions{})
			if err != nil {
				return nil
			}
		}
		d.cache.developerServices[name] = newS
		return d.divertEndpoints(ctx, name)
	}

	if s.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}

	updatedS := translateService(d.name, d.namespace, from)

	if !isEqualService(s, updatedS) {
		oktetoLog.Infof("updating service %s/%s", updatedS.Namespace, updatedS.Name)
		if _, err := d.client.CoreV1().Services(d.namespace).Update(ctx, updatedS, metav1.UpdateOptions{}); err != nil {
			if !k8sErrors.IsConflict(err) {
				return err
			}
			// the service was updated, refresh the cache
			updatedS, err = d.client.CoreV1().Services(d.namespace).Get(ctx, updatedS.Name, metav1.GetOptions{})
			if err != nil {
				return nil
			}
		}
		d.cache.developerServices[name] = updatedS
	}
	return d.divertEndpoints(ctx, name)
}

func translateService(name, namespace string, s *apiv1.Service) *apiv1.Service {
	result := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   namespace,
			Labels:      s.Labels,
			Annotations: s.Annotations,
		},
		Spec: s.Spec,
	}
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(name))
	// create a headless service pointing to an endpoints object that resolves to service cluster ip in the diverted namespace
	result.Spec.ClusterIP = apiv1.ClusterIPNone
	result.Spec.ClusterIPs = nil
	result.Spec.Selector = nil
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	return result
}

func isEqualService(s1 *apiv1.Service, s2 *apiv1.Service) bool {
	return reflect.DeepEqual(s1.Spec.Ports, s2.Spec.Ports) && reflect.DeepEqual(s1.Labels, s2.Labels) && reflect.DeepEqual(s1.Annotations, s2.Annotations) && reflect.DeepEqual(s1.Spec.Selector, s2.Spec.Selector)
}

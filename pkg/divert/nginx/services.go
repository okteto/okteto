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

package nginx

import (
	"context"
	"fmt"

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

	if _, ok := d.cache.developerServices[name]; !ok {
		newS := translateService(d.name, d.namespace, from)
		oktetoLog.Infof("creating service %s/%s", newS.Namespace, newS.Name)
		if _, err := d.client.CoreV1().Services(d.namespace).Create(ctx, newS, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
		}
		d.cache.developerServices[name] = newS
		return nil
	}

	return nil

}

func translateService(name, namespace string, s *apiv1.Service) *apiv1.Service {
	result := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   namespace,
			Labels:      s.Labels,
			Annotations: s.Annotations,
		},
		Spec: apiv1.ServiceSpec{
			Type:         apiv1.ServiceTypeExternalName,
			ExternalName: fmt.Sprintf("%s.%s.svc.cluster.local", s.Name, s.Namespace),
		},
	}
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(name))
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	result.Annotations[model.OktetoDivertedNamespaceAnnotation] = s.Namespace
	return result
}

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
	"encoding/json"
	"reflect"

	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PortMapping represents the divert original port mappings
type PortMapping struct {
	ProxyPort          int32 `json:"proxy_port,omitempty" yaml:"proxy_port,omitempty"`
	OriginalPort       int32 `json:"original_port,omitempty" yaml:"original_port,omitempty"`
	OriginalTargetPort int32 `json:"original_target_port,omitempty" yaml:"original_target_port,omitempty"`
}

func (d *Driver) divertService(ctx context.Context, name string) error {
	from, ok := d.cache.divertServices[name]
	if !ok {
		oktetoLog.Infof("service %s not found: %s", name)
		return nil
	}
	s, ok := d.cache.developerServices[name]
	if !ok {
		newS, err := translateService(d.Manifest, from)
		if err != nil {
			return err
		}
		oktetoLog.Infof("creating service %s/%s", newS.Namespace, newS.Name)
		if _, err := d.Client.CoreV1().Services(d.Manifest.Namespace).Create(ctx, newS, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
		}
		d.cache.developerServices[name] = newS
		return d.divertEndpoints(ctx, name)
	}

	if s.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}

	updatedS, err := translateService(d.Manifest, from)
	if err != nil {
		return err
	}
	if !isEqualService(s, updatedS) {
		oktetoLog.Infof("updating service %s/%s", updatedS.Namespace, updatedS.Name)
		if _, err := d.Client.CoreV1().Services(d.Manifest.Namespace).Update(ctx, updatedS, metav1.UpdateOptions{}); err != nil {
			if !k8sErrors.IsConflict(err) {
				return err
			}
		}
		d.cache.developerServices[name] = updatedS
	}
	return d.divertEndpoints(ctx, name)
}

func translateService(m *model.Manifest, s *apiv1.Service) (*apiv1.Service, error) {
	result := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   m.Namespace,
			Labels:      s.Labels,
			Annotations: s.Annotations,
		},
		Spec: s.Spec,
	}
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(m.Name))
	// create a headless service pointing to an endpoints object that resolves to service cluster ip in the diverted namespace
	result.Spec.ClusterIP = apiv1.ClusterIPNone
	result.Spec.ClusterIPs = nil
	result.Spec.Selector = nil
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"

	if v := result.Annotations[model.OktetoDivertServiceAnnotation]; v != "" {
		divertMapping := PortMapping{}
		if err := json.Unmarshal([]byte(v), &divertMapping); err != nil {
			return nil, err
		}
		for i := range result.Spec.Ports {
			if result.Spec.Ports[i].TargetPort.IntVal == divertMapping.ProxyPort {
				result.Spec.Ports[i].TargetPort = intstr.IntOrString{IntVal: divertMapping.OriginalTargetPort}
			}
		}
		delete(result.Annotations, model.OktetoDivertServiceAnnotation)
	}

	return result, nil
}

func isEqualService(s1 *apiv1.Service, s2 *apiv1.Service) bool {
	return reflect.DeepEqual(s1.Spec.Ports, s2.Spec.Ports) && reflect.DeepEqual(s1.Labels, s2.Labels) && reflect.DeepEqual(s1.Annotations, s2.Annotations)
}

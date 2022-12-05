// Copyright 2022 The Okteto Authors
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

package stack

import (
	"encoding/json"

	"github.com/okteto/okteto/pkg/k8s/diverts"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func applyDivertToDeployment(d *appsv1.Deployment, old *appsv1.Deployment) {
	if old.Spec.Template.Labels == nil {
		return
	}
	if old.Spec.Template.Labels[model.OktetoDivertInjectSidecarLabel] == "" {
		return
	}
	if d.Spec.Template.Labels == nil {
		d.Spec.Template.Labels = map[string]string{}
	}
	d.Spec.Template.Labels[model.OktetoDivertInjectSidecarLabel] = old.Spec.Template.Labels[model.OktetoDivertInjectSidecarLabel]
}

func applyDivertToService(s *apiv1.Service, old *apiv1.Service) {
	if old.Annotations[model.OktetoDivertServiceAnnotation] == "" {
		return
	}
	if s.Annotations == nil {
		s.Annotations = map[string]string{}
	}
	s.Annotations[model.OktetoDivertServiceAnnotation] = old.Annotations[model.OktetoDivertServiceAnnotation]
	divertMapping := diverts.PortMapping{}
	if err := json.Unmarshal([]byte(old.Annotations[model.OktetoDivertServiceAnnotation]), &divertMapping); err != nil {
		oktetoLog.Warning("skipping apply divert to service '%s': %s", s.Name, err.Error())
		return
	}
	for i := range s.Spec.Ports {
		if s.Spec.Ports[i].Port == divertMapping.OriginalPort {
			s.Spec.Ports[i].TargetPort = intstr.IntOrString{IntVal: divertMapping.ProxyPort}
		}
	}
}

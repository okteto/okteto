// Copyright 2020 The Okteto Authors
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

package services

import (
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	oktetoAutoIngressAnnotation = "dev.okteto.com/auto-ingress"
)

func translate(dev *model.Dev) *apiv1.Service {
	annotations := model.Annotations{}
	if len(dev.Services) == 0 {
		annotations[oktetoAutoIngressAnnotation] = "true"
	}
	for k, v := range dev.Annotations {
		annotations[k] = v
	}
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Labels: map[string]string{
				labels.DevLabel: "true",
			},
			Annotations: annotations,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{"app": dev.Name},
			Type:     apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name:       dev.Name,
					Port:       8080,
					TargetPort: intstr.IntOrString{StrVal: "8080"},
				},
			},
		},
	}
}

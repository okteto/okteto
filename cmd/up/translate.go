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

package up

import (
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func translateService(svcName string, s *model.Stack) *apiv1.Service {
	svc := s.Services[svcName]
	annotations := translateAnnotations(svc)

	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Namespace:   s.Namespace,
			Annotations: annotations,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				"app": utils.DetachModePodName,
			},
			Type:  apiv1.ServiceTypeClusterIP,
			Ports: translateServicePorts(svcName, svc),
		},
	}
}

func translateAnnotations(svc *model.Service) map[string]string {
	result := map[string]string{}
	for k, v := range svc.Annotations {
		result[k] = v
	}
	return result
}

func translateServicePorts(svcName string, svc *model.Service) []apiv1.ServicePort {
	result := []apiv1.ServicePort{}
	for _, p := range svc.Ports {
		result = append(result, apiv1.ServicePort{
			Name:       fmt.Sprintf("%s-%d", svcName, p.ContainerPort),
			Port:       p.ContainerPort,
			TargetPort: intstr.IntOrString{IntVal: p.ContainerPort},
			Protocol:   apiv1.ProtocolTCP,
		})
	}
	return result
}

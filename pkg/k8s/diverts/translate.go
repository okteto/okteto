// Copyright 2021 The Okteto Authors
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

package diverts

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/constants"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type divertServiceModification struct {
	ProxyPort          int `json:"proxy_port"`
	OriginalPort       int `json:"original_port"`
	OriginalTargetPort int `json:"original_target_port"`
}

func serviceModFromAnnotationValue(val string) (divertServiceModification, error) {
	type modAnnotationValue struct {
		ProxyPort          json.Number `json:"proxy_port"`
		OriginalPort       json.Number `json:"original_port"`
		OriginalTargetPort json.Number `json:"original_target_port"`
	}

	var modVal modAnnotationValue
	if err := json.Unmarshal([]byte(val), &modVal); err != nil {
		return divertServiceModification{}, err
	}

	proxyPort, err := modVal.ProxyPort.Int64()
	if err != nil {
		return divertServiceModification{}, err
	}

	originalPort, err := modVal.OriginalPort.Int64()
	if err != nil {
		return divertServiceModification{}, err
	}

	originalTargetPort, err := modVal.OriginalTargetPort.Int64()
	if err != nil {
		return divertServiceModification{}, err
	}

	return divertServiceModification{
		ProxyPort:          int(proxyPort),
		OriginalPort:       int(originalPort),
		OriginalTargetPort: int(originalTargetPort),
	}, nil
}

func translateService(username string, app apps.App, s *apiv1.Service) (*apiv1.Service, error) {
	result := s.DeepCopy()
	result.UID = ""
	result.Name = model.DivertName(s.Name, username)
	result.Labels = map[string]string{constants.OktetoDivertLabel: username}
	if s.Labels != nil && s.Labels[constants.DeployedByLabel] != "" {
		result.Labels[constants.DeployedByLabel] = s.Labels[constants.DeployedByLabel]
	}
	if s.Annotations != nil {
		modification := s.Annotations[constants.OktetoDivertServiceModificationAnnotation]
		if modification != "" {
			mod, err := serviceModFromAnnotationValue(modification)
			if err != nil {
				return nil, fmt.Errorf("bad divert service modification: %s", modification)
			}
			for i := range result.Spec.Ports {
				if result.Spec.Ports[i].Port == int32(mod.OriginalPort) {
					result.Spec.Ports[i].TargetPort = intstr.FromInt(int(mod.OriginalTargetPort))
				}
			}
		}
	}
	delete(result.Annotations, constants.OktetoAutoIngressAnnotation)
	delete(result.Annotations, constants.OktetoDivertServiceModificationAnnotation)
	result.Spec.Selector = map[string]string{
		constants.OktetoDivertLabel:   username,
		constants.InteractiveDevLabel: app.ObjectMeta().Name,
	}
	result.ResourceVersion = ""
	result.Spec.ClusterIP = ""
	result.Spec.ClusterIPs = []string{}
	return result, nil
}

func translateIngress(username string, i *networkingv1.Ingress) *networkingv1.Ingress {
	result := i.DeepCopy()
	result.UID = ""
	result.Name = model.DivertName(i.Name, username)
	result.Labels = map[string]string{constants.OktetoDivertLabel: username}
	if i.Labels != nil && i.Labels[constants.DeployedByLabel] != "" {
		result.Labels[constants.DeployedByLabel] = i.Labels[constants.DeployedByLabel]
	}
	if host := result.Annotations[constants.OktetoIngressAutoGenerateHost]; host != "" {
		if host != "true" {
			result.Annotations[constants.OktetoIngressAutoGenerateHost] = fmt.Sprintf("%s-%s", username, host)
		}
	} else {
		result.Annotations[constants.OktetoIngressAutoGenerateHost] = "true"
	}
	result.ResourceVersion = ""
	return result
}

func translateDivertCRD(username string, dev *model.Dev, s *apiv1.Service, i *networkingv1.Ingress) *Divert {
	result := &Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: dev.Namespace,
		},
		Spec: DivertSpec{
			Ingress: IngressDivertSpec{
				Name:      i.Name,
				Namespace: dev.Namespace,
				Value:     username,
			},
			FromService: ServiceDivertSpec{
				Name:      dev.Divert.Service,
				Namespace: dev.Namespace,
				Port:      dev.Divert.Port,
			},
			ToService: ServiceDivertSpec{
				Name:      s.Name,
				Namespace: dev.Namespace,
				Port:      dev.Divert.Port,
			},
			Deployment: DeploymentDivertSpec{
				Name:      dev.Name,
				Namespace: dev.Namespace,
			},
		},
	}
	if s.Labels != nil && s.Labels[constants.DeployedByLabel] != "" {
		result.Labels = map[string]string{constants.DeployedByLabel: s.Labels[constants.DeployedByLabel]}
	}
	return result
}

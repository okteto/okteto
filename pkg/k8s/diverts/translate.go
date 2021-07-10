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

	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
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

// DivertName returns the name of the diverted version of a given resource
func DivertName(username, name string) string {
	return fmt.Sprintf("%s-%s", username, name)
}

func translateDeployment(username string, d *appsv1.Deployment) *appsv1.Deployment {
	result := d.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, d.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if d.Labels != nil && d.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = d.Labels[model.DeployedByLabel]
	}
	result.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			model.OktetoDivertLabel: username,
		},
	}
	result.Spec.Template.Labels = map[string]string{
		model.OktetoDivertLabel: username,
	}
	annotations.Set(result.GetObjectMeta(), model.OktetoAutoCreateAnnotation, model.OktetoUpCmd)
	result.ResourceVersion = ""
	return result
}

func translateStatefulset(username string, d *appsv1.StatefulSet) *appsv1.StatefulSet {
	result := d.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, d.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if d.Labels != nil && d.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = d.Labels[model.DeployedByLabel]
	}
	result.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			model.OktetoDivertLabel: username,
		},
	}
	result.Spec.Template.Labels = map[string]string{
		model.OktetoDivertLabel: username,
	}
	annotations.Set(result.GetObjectMeta(), model.OktetoAutoCreateAnnotation, model.OktetoUpCmd)
	result.ResourceVersion = ""
	return result
}

func translateService(username string, r *model.K8sObject, s *apiv1.Service) (*apiv1.Service, error) {
	result := s.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, s.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if s.Labels != nil && s.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = s.Labels[model.DeployedByLabel]
	}
	if s.Annotations != nil {
		modification := s.Annotations[model.OktetoDivertServiceModificationAnnotation]
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
	delete(result.Annotations, model.OktetoAutoIngressAnnotation)
	delete(result.Annotations, model.OktetoDivertServiceModificationAnnotation)
	result.Spec.Selector = map[string]string{
		model.OktetoDivertLabel:   username,
		model.InteractiveDevLabel: r.Name,
	}
	result.ResourceVersion = ""
	result.Spec.ClusterIP = ""
	return result, nil
}

func translateIngress(username string, i *networkingv1.Ingress) *networkingv1.Ingress {
	result := i.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, i.Name)
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if i.Labels != nil && i.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = i.Labels[model.DeployedByLabel]
	}
	if host := annotations.Get(result.GetObjectMeta(), model.OktetoIngressAutoGenerateHost); host != "" {
		if host != "true" {
			annotations.Set(result.GetObjectMeta(), model.OktetoIngressAutoGenerateHost, fmt.Sprintf("%s-%s", username, host))
		}
	} else {
		annotations.Set(result.GetObjectMeta(), model.OktetoIngressAutoGenerateHost, "true")
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
	if s.Labels != nil && s.Labels[model.DeployedByLabel] != "" {
		result.Labels = map[string]string{model.DeployedByLabel: s.Labels[model.DeployedByLabel]}
	}
	return result
}

func translateDev(username string, dev *model.Dev, k8sObject *model.K8sObject) {
	dev.Name = k8sObject.Name
	dev.Labels = nil
}

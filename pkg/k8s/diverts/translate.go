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

package diverts

import (
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translateIngress(name, namespace string, in *networkingv1.Ingress, resourceVersion string) *networkingv1.Ingress {
	result := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            in.Name,
			Labels:          in.Labels,
			Annotations:     in.Annotations,
			ResourceVersion: resourceVersion,
		},
		Spec: in.Spec,
	}
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, name)
	for i := range result.Spec.Rules {
		result.Spec.Rules[i].Host = strings.ReplaceAll(result.Spec.Rules[i].Host, in.Namespace, namespace)
	}
	for i := range result.Spec.TLS {
		for j := range result.Spec.TLS[i].Hosts {
			result.Spec.TLS[i].Hosts[j] = strings.ReplaceAll(result.Spec.TLS[i].Hosts[j], in.Namespace, namespace)
		}
	}
	return result
}

func translateService(name string, s *apiv1.Service, resourceVersion string) *apiv1.Service {
	result := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            s.Name,
			Labels:          s.Labels,
			Annotations:     s.Annotations,
			ResourceVersion: resourceVersion,
		},
		Spec: s.Spec,
	}
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, name)
	// create a headless service pointing to an endpoints object that resolves to pods in the diverted namespace
	result.Spec.ClusterIP = apiv1.ClusterIPNone
	result.Spec.ClusterIPs = nil
	result.Spec.Selector = nil
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	return result
}

func translateEndpoints(manifestName string, e *apiv1.Endpoints, resourceVersion string) *apiv1.Endpoints {
	result := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:            e.Name,
			Labels:          e.Labels,
			Annotations:     e.Annotations,
			ResourceVersion: resourceVersion,
		},
		Subsets: e.Subsets,
	}
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, manifestName)
	labels.SetInMetadata(&result.ObjectMeta, model.OktetoDivertedFromLabel, string(e.UID))
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	return result
}

func translateDivertCRD(m *model.Manifest, in *networkingv1.Ingress) *Divert {
	result := &Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", m.Name, in.Name),
			Namespace:   m.Namespace,
			Labels:      map[string]string{model.DeployedByLabel: m.Name},
			Annotations: map[string]string{model.OktetoAutoCreateAnnotation: "true"},
		},
		Spec: DivertSpec{
			Ingress: IngressDivertSpec{
				Name:      in.Name,
				Namespace: m.Namespace,
				Value:     m.Namespace,
			},
			FromService: ServiceDivertSpec{
				Name:      m.Deploy.Divert.Service,
				Namespace: m.Deploy.Divert.Namespace,
				Port:      m.Deploy.Divert.Port,
			},
			ToService: ServiceDivertSpec{
				Name:      m.Deploy.Divert.Service,
				Namespace: m.Namespace,
				Port:      m.Deploy.Divert.Port,
			},
			Deployment: DeploymentDivertSpec{
				Name:      m.Deploy.Divert.Deployment,
				Namespace: m.Deploy.Divert.Namespace,
			},
		},
	}
	return result
}

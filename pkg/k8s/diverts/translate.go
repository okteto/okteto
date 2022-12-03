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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/textblock"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	nginxConfigurationSnippetAnnotation = "nginx.ingress.kubernetes.io/configuration-snippet"
	divertIngressInjectionAnnotation    = "divert.okteto.com/injection"
	divertTextBlockHeader               = "# ---- START DIVERT ----"
	divertTextBlockFooter               = "# ---- END DIVERT ----"
)

var (
	divertTextBlockParser = textblock.NewTextBlock(divertTextBlockHeader, divertTextBlockFooter)
)

// PortMapping represents the port mapping of a divert
type PortMapping struct {
	ProxyPort          int32 `json:"proxy_port,omitempty" yaml:"proxy_port,omitempty"`
	OriginalPort       int32 `json:"original_port,omitempty" yaml:"original_port,omitempty"`
	OriginalTargetPort int32 `json:"original_target_port,omitempty" yaml:"original_target_port,omitempty"`
}

func translateIngress(m *model.Manifest, in *networkingv1.Ingress) *networkingv1.Ingress {
	result := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        in.Name,
			Namespace:   m.Namespace,
			Labels:      in.Labels,
			Annotations: in.Annotations,
		},
		Spec: in.Spec,
	}
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	result.Annotations[divertIngressInjectionAnnotation] = m.Namespace
	result.Annotations[nginxConfigurationSnippetAnnotation] = divertTextBlockParser.WriteBlock(fmt.Sprintf("proxy_set_header x-okteto-dvrt %s;", m.Namespace))

	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(m.Name))
	for i := range result.Spec.Rules {
		result.Spec.Rules[i].Host = strings.ReplaceAll(result.Spec.Rules[i].Host, in.Namespace, m.Namespace)
	}
	for i := range result.Spec.TLS {
		for j := range result.Spec.TLS[i].Hosts {
			result.Spec.TLS[i].Hosts[j] = strings.ReplaceAll(result.Spec.TLS[i].Hosts[j], in.Namespace, m.Namespace)
		}
	}
	return result
}

func isEqualIngress(in1 *networkingv1.Ingress, in2 *networkingv1.Ingress) bool {
	if in1.Annotations == nil {
		in1.Annotations = map[string]string{}
	}
	if in2.Annotations == nil {
		in2.Annotations = map[string]string{}
	}
	return reflect.DeepEqual(in1.Spec, in2.Spec) && (in1.Annotations[divertIngressInjectionAnnotation] == in2.Annotations[divertIngressInjectionAnnotation])
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
	return reflect.DeepEqual(s1.Spec, s2.Spec)
}

func translateEndpoints(m *model.Manifest, s *apiv1.Service) *apiv1.Endpoints {
	result := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.Name,
			Namespace:   m.Namespace,
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
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(m.Name))
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
	return reflect.DeepEqual(e1.Subsets, e2.Subsets)
}

func translateDivertCRD(m *model.Manifest) *Divert {
	result := &Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", m.Name, m.Deploy.Divert.Service),
			Namespace: m.Namespace,
			Labels: map[string]string{
				model.DeployedByLabel:    format.ResourceK8sMetaString(m.Name),
				"dev.okteto.com/version": "0.1.9",
			},
			Annotations: map[string]string{model.OktetoAutoCreateAnnotation: "true"},
		},
		Spec: DivertSpec{
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

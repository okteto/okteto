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

package weaver

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (d *Driver) divertIngress(ctx context.Context, name string) error {
	from := d.cache.divertIngresses[name]
	in, ok := d.cache.developerIngresses[name]
	if !ok {
		in = translateIngress(d.Manifest, from)
		oktetoLog.Infof("creating ingress %s/%s", in.Namespace, in.Name)
		if _, err := d.Client.NetworkingV1().Ingresses(d.Manifest.Namespace).Create(ctx, in, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
			d.cache.developerIngresses[name] = in
		}
	} else {
		updatedIn := in.DeepCopy()
		if in.Annotations[model.OktetoAutoCreateAnnotation] == "true" {
			// ingress was created by divert
			updatedIn = translateIngress(d.Manifest, d.cache.divertIngresses[name])
		} else if in.Annotations[model.OktetoDivertIngressInjectionAnnotation] != d.Manifest.Namespace {
			// ingress wasnt created by divert, check header injection
			if updatedIn.Annotations == nil {
				updatedIn.Annotations = map[string]string{}
			}
			updatedIn.Annotations[model.OktetoDivertIngressInjectionAnnotation] = d.Manifest.Namespace
			updatedIn.Annotations[model.OktetoNginxConfigurationSnippetAnnotation] = divertTextBlockParser.WriteBlock(fmt.Sprintf("proxy_set_header x-okteto-dvrt %s;", d.Manifest.Namespace))
		}
		if !isEqualIngress(in, updatedIn) {
			oktetoLog.Infof("updating ingress %s/%s", updatedIn.Namespace, updatedIn.Name)
			if _, err := d.Client.NetworkingV1().Ingresses(d.Manifest.Namespace).Update(ctx, updatedIn, metav1.UpdateOptions{}); err != nil {
				if !k8sErrors.IsConflict(err) {
					return err
				}
			}
			d.cache.developerIngresses[name] = updatedIn
			in = updatedIn
		}
	}

	for _, rule := range in.Spec.Rules {
		for _, path := range rule.IngressRuleValue.HTTP.Paths {
			if err := d.divertService(ctx, path.Backend.Service.Name); err != nil {
				return fmt.Errorf("error diverting ingress '%s/%s' service '%s': %v", in.Namespace, in.Name, path.Backend.Service.Name, err)
			}
		}
	}
	return nil
}

func translateIngress(m *model.Manifest, from *networkingv1.Ingress) *networkingv1.Ingress {
	result := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        from.Name,
			Namespace:   m.Namespace,
			Labels:      from.Labels,
			Annotations: from.Annotations,
		},
		Spec: from.Spec,
	}
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	result.Annotations[model.OktetoDivertIngressInjectionAnnotation] = m.Namespace
	result.Annotations[model.OktetoNginxConfigurationSnippetAnnotation] = divertTextBlockParser.WriteBlock(fmt.Sprintf("proxy_set_header x-okteto-dvrt %s;", m.Namespace))

	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(m.Name))
	for i := range result.Spec.Rules {
		result.Spec.Rules[i].Host = strings.ReplaceAll(result.Spec.Rules[i].Host, from.Namespace, m.Namespace)
	}
	for i := range result.Spec.TLS {
		for j := range result.Spec.TLS[i].Hosts {
			result.Spec.TLS[i].Hosts[j] = strings.ReplaceAll(result.Spec.TLS[i].Hosts[j], from.Namespace, m.Namespace)
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
	return reflect.DeepEqual(in1.Spec, in2.Spec) && (in1.Annotations[model.OktetoDivertIngressInjectionAnnotation] == in2.Annotations[model.OktetoDivertIngressInjectionAnnotation])
}

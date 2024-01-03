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
		in = translateIngress(d.name, d.namespace, from)
		oktetoLog.Infof("creating ingress %s/%s", in.Namespace, in.Name)
		if _, err := d.client.NetworkingV1().Ingresses(d.namespace).Create(ctx, in, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
			in, err = d.client.NetworkingV1().Ingresses(d.namespace).Get(ctx, in.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			// the ingress was created, refresh the cache
			d.cache.developerIngresses[name] = in
		}
	} else {
		updatedIn := in.DeepCopy()
		if in.Annotations[model.OktetoAutoCreateAnnotation] == "true" {
			// ingress was created by divert
			updatedIn = translateIngress(d.name, d.namespace, d.cache.divertIngresses[name])
		}
		if !isEqualIngress(in, updatedIn) {
			oktetoLog.Infof("updating ingress %s/%s", updatedIn.Namespace, updatedIn.Name)
			if _, err := d.client.NetworkingV1().Ingresses(d.namespace).Update(ctx, updatedIn, metav1.UpdateOptions{}); err != nil {
				if !k8sErrors.IsConflict(err) {
					return err
				}
				// the ingress was updated, refresh the cache
				updatedIn, err = d.client.NetworkingV1().Ingresses(d.namespace).Get(ctx, in.Name, metav1.GetOptions{})
				if err != nil {
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
				return fmt.Errorf("error diverting ingress '%s/%s' service '%s': %w", in.Namespace, in.Name, path.Backend.Service.Name, err)
			}
		}
	}
	return nil
}

func translateIngress(name, namespace string, from *networkingv1.Ingress) *networkingv1.Ingress {
	result := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        from.Name,
			Namespace:   namespace,
			Labels:      from.Labels,
			Annotations: from.Annotations,
		},
		Spec: from.Spec,
	}
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"

	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, format.ResourceK8sMetaString(name))
	for i := range result.Spec.Rules {
		result.Spec.Rules[i].Host = strings.ReplaceAll(result.Spec.Rules[i].Host, from.Namespace, namespace)
	}
	for i := range result.Spec.TLS {
		for j := range result.Spec.TLS[i].Hosts {
			result.Spec.TLS[i].Hosts[j] = strings.ReplaceAll(result.Spec.TLS[i].Hosts[j], from.Namespace, namespace)
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
	return reflect.DeepEqual(in1.Spec, in2.Spec) && reflect.DeepEqual(in1.Labels, in2.Labels) && reflect.DeepEqual(in1.Annotations, in2.Annotations)
}

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
	"context"
	"fmt"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DivertCache keep information about k8s service, ingress and endpoints to reduce k8s calls for diverting a namespace
type DivertCache struct {
	// DivertIngresses ingress cache for diverted namespace
	DivertIngresses map[string]*networkingv1.Ingress
	// DivertServices service cache for diverted namespace
	DivertServices map[string]*apiv1.Service
	// DivertEndpoints endpoints cache for diverted namespace
	DivertEndpoints map[string]*apiv1.Endpoints
	// DivertIngresses ingress cache for developer namespace
	DeveloperIngresses map[string]*networkingv1.Ingress
	// DivertServices service cache for developer namespace
	DeveloperServices map[string]*apiv1.Service
	// DivertEndpoints endpoints cache for developer namespace
	DeveloperEndpoints map[string]*apiv1.Endpoints
}

func InitDivertCache(ctx context.Context, m *model.Manifest, c kubernetes.Interface) (*DivertCache, error) {
	result := &DivertCache{
		DivertIngresses:    map[string]*networkingv1.Ingress{},
		DivertServices:     map[string]*apiv1.Service{},
		DivertEndpoints:    map[string]*apiv1.Endpoints{},
		DeveloperIngresses: map[string]*networkingv1.Ingress{},
		DeveloperServices:  map[string]*apiv1.Service{},
		DeveloperEndpoints: map[string]*apiv1.Endpoints{},
	}
	// Init ingress cache for diverted namespace
	iList, err := c.NetworkingV1().Ingresses(m.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range iList.Items {
		result.DivertIngresses[iList.Items[i].Name] = &iList.Items[i]
	}

	// Service cache for diverted namespace
	sList, err := c.CoreV1().Services(m.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range sList.Items {
		result.DivertServices[sList.Items[i].Name] = &sList.Items[i]
	}

	// Endpoints cache for diverted namespace
	eList, err := c.CoreV1().Endpoints(m.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range eList.Items {
		result.DivertEndpoints[eList.Items[i].Name] = &eList.Items[i]
	}

	// Ingress cache for developer namespace
	iList, err = c.NetworkingV1().Ingresses(m.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range iList.Items {
		result.DeveloperIngresses[iList.Items[i].Name] = &iList.Items[i]
	}

	// Service cache for developer namespace
	sList, err = c.CoreV1().Services(m.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range sList.Items {
		result.DeveloperServices[sList.Items[i].Name] = &sList.Items[i]
	}

	// Endpoints cache for developer namespace
	eList, err = c.CoreV1().Endpoints(m.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range eList.Items {
		result.DeveloperEndpoints[eList.Items[i].Name] = &eList.Items[i]
	}

	return result, nil
}

// DivertIngress diverts the traffic of a given ingress
func DivertIngress(ctx context.Context, m *model.Manifest, name string, cache *DivertCache, c kubernetes.Interface) error {
	from := cache.DivertIngresses[name]
	in, ok := cache.DeveloperIngresses[name]
	if !ok {
		in = translateIngress(m, from)
		oktetoLog.Infof("creating ingress %s/%s", in.Namespace, in.Name)
		if _, err := c.NetworkingV1().Ingresses(m.Namespace).Create(ctx, in, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
			cache.DeveloperIngresses[name] = in
		}
	} else {
		updatedIn := in.DeepCopy()
		if in.Annotations[model.OktetoAutoCreateAnnotation] == "true" {
			// ingress was created by divert
			updatedIn = translateIngress(m, cache.DivertIngresses[name])
		} else if in.Annotations[model.OktetoDivertIngressInjectionAnnotation] != m.Namespace {
			// ingress wasnt created by divert, check header injection
			if updatedIn.Annotations == nil {
				updatedIn.Annotations = map[string]string{}
			}
			updatedIn.Annotations[model.OktetoDivertIngressInjectionAnnotation] = m.Namespace
			updatedIn.Annotations[model.OktetoNginxConfigurationSnippetAnnotation] = divertTextBlockParser.WriteBlock(fmt.Sprintf("proxy_set_header x-okteto-dvrt %s;", m.Namespace))
		}
		if !isEqualIngress(in, updatedIn) {
			oktetoLog.Infof("updating ingress %s/%s", updatedIn.Namespace, updatedIn.Name)
			if _, err := c.NetworkingV1().Ingresses(m.Namespace).Update(ctx, updatedIn, metav1.UpdateOptions{}); err != nil {
				return err
			}
			cache.DeveloperIngresses[name] = updatedIn
			in = updatedIn
		}
	}

	for _, rule := range in.Spec.Rules {
		for _, path := range rule.IngressRuleValue.HTTP.Paths {
			if err := divertService(ctx, m, path.Backend.Service.Name, cache, c); err != nil {
				return fmt.Errorf("error diverting ingress '%s/%s' service '%s': %v", in.Namespace, in.Name, path.Backend.Service.Name, err)
			}
		}
	}
	return nil
}

func divertService(ctx context.Context, m *model.Manifest, name string, cache *DivertCache, c kubernetes.Interface) error {
	from, ok := cache.DivertServices[name]
	if !ok {
		oktetoLog.Infof("service %s not found: %s", name)
		return nil
	}
	s, ok := cache.DeveloperServices[name]
	if !ok {
		newS, err := translateService(m, from)
		if err != nil {
			return err
		}
		oktetoLog.Infof("creating service %s/%s", newS.Namespace, newS.Name)
		if _, err := c.CoreV1().Services(m.Namespace).Create(ctx, newS, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
		}
		cache.DeveloperServices[name] = newS
		return divertEndpoints(ctx, m, name, cache, c)
	}

	if s.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}

	updatedS, err := translateService(m, from)
	if err != nil {
		return err
	}
	if !isEqualService(s, updatedS) {
		oktetoLog.Infof("updating service %s/%s", updatedS.Namespace, updatedS.Name)
		if _, err := c.CoreV1().Services(m.Namespace).Update(ctx, updatedS, metav1.UpdateOptions{}); err != nil {
			return err
		}
		cache.DeveloperServices[name] = updatedS
	}
	return divertEndpoints(ctx, m, name, cache, c)
}

func divertEndpoints(ctx context.Context, m *model.Manifest, name string, cache *DivertCache, c kubernetes.Interface) error {
	from := cache.DivertServices[name]
	e, ok := cache.DeveloperEndpoints[name]
	if !ok {
		newE := translateEndpoints(m, from)
		oktetoLog.Infof("creating endpoint %s/%s", newE.Namespace, newE.Name)
		if _, err := c.CoreV1().Endpoints(m.Namespace).Create(ctx, newE, metav1.CreateOptions{}); err != nil {
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
		}
		cache.DeveloperEndpoints[name] = newE
		return nil
	}
	if e.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}
	updatedE := translateEndpoints(m, from)
	if isEqualEndpoints(e, updatedE) {
		return nil
	}
	oktetoLog.Infof("updating endpoints %s/%s", updatedE.Namespace, updatedE.Name)
	if _, err := c.CoreV1().Endpoints(m.Namespace).Update(ctx, updatedE, metav1.UpdateOptions{}); err != nil {
		return err
	}
	cache.DeveloperEndpoints[name] = updatedE
	return nil
}

func CreateDivertCRD(ctx context.Context, m *model.Manifest) error {
	dClient, err := getDivertClient()
	if err != nil {
		return fmt.Errorf("error creating divert CRD client: %s", err.Error())
	}

	divertCRD := translateDivertCRD(m)

	old, err := dClient.Diverts(m.Namespace).Get(ctx, divertCRD.Name, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error getting divert CRD '%s': %w", divertCRD.Name, err)
	}

	if old.Name == "" {
		oktetoLog.Infof("creating  divert CRD '%s'", divertCRD.Name)
		_, err = dClient.Diverts(m.Namespace).Create(ctx, divertCRD)
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating divert CRD '%s': %w", divertCRD.Name, err)
		}
		oktetoLog.Infof("created divert CRD '%s'", divertCRD.Name)
	} else {
		oktetoLog.Infof("updating divert CRD '%s'", divertCRD.Name)
		old.TypeMeta = divertCRD.TypeMeta
		old.Annotations = divertCRD.Annotations
		old.Labels = divertCRD.Labels
		old.Spec = divertCRD.Spec
		old.Status = DivertStatus{}
		_, err = dClient.Diverts(m.Namespace).Update(ctx, old)
		if err != nil {
			return fmt.Errorf("error updating divert CRD '%s': %w", divertCRD.Name, err)
		}
		oktetoLog.Infof("updated divert CRD '%s'.", divertCRD.Name)
	}

	return nil
}

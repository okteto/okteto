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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//DivertIngress diverts the traffic of a given ingress
func DivertIngress(ctx context.Context, m *model.Manifest, fromIn *networkingv1.Ingress, c kubernetes.Interface) error {
	in, err := c.NetworkingV1().Ingresses(m.Namespace).Get(ctx, fromIn.Name, metav1.GetOptions{})
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return err
		}
		in = translateIngress(m, fromIn, "")
		if _, err := c.NetworkingV1().Ingresses(m.Namespace).Create(ctx, in, metav1.CreateOptions{}); err != nil {
			return err
		}
	} else if in.Annotations[model.OktetoAutoCreateAnnotation] == "true" {
		in = translateIngress(m, fromIn, in.ResourceVersion)
		in.ResourceVersion = ""
		if _, err := c.NetworkingV1().Ingresses(m.Namespace).Update(ctx, in, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	for _, rule := range in.Spec.Rules {
		for _, path := range rule.IngressRuleValue.HTTP.Paths {
			if err := divertService(ctx, m, path.Backend.Service.Name, c); err != nil {
				return fmt.Errorf("error diverting ingress '%s' service '%s': %v", in.Name, path.Backend.Service.Name, err)
			}
		}
	}
	return createDivertCRD(ctx, m, fromIn)
}

func divertService(ctx context.Context, m *model.Manifest, name string, c kubernetes.Interface) error {
	from, err := c.CoreV1().Services(m.Deploy.Divert.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			oktetoLog.Infof("service %s not found: %s", name)
			return nil
		}
		return err
	}
	s, err := c.CoreV1().Services(m.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return err
		}
		s = translateService(m, from, "")
		if _, err := c.CoreV1().Services(m.Namespace).Create(ctx, s, metav1.CreateOptions{}); err != nil {
			return err
		}
		return divertEndpoints(ctx, m, name, c)
	}

	if s.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}

	s = translateService(m, from, s.ResourceVersion)
	if _, err := c.CoreV1().Services(m.Namespace).Update(ctx, s, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return divertEndpoints(ctx, m, name, c)
}

func divertEndpoints(ctx context.Context, m *model.Manifest, name string, c kubernetes.Interface) error {
	from, err := c.CoreV1().Endpoints(m.Deploy.Divert.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			oktetoLog.Infof("endpoint %s not found: %s", name)
			return nil
		}
		return err
	}
	e, err := c.CoreV1().Endpoints(m.Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return err
		}
		e = translateEndpoints(m, from, "")
		_, err := c.CoreV1().Endpoints(m.Namespace).Create(ctx, e, metav1.CreateOptions{})
		return err
	}
	if e.Annotations[model.OktetoAutoCreateAnnotation] != "true" {
		return nil
	}
	e = translateEndpoints(m, from, e.ResourceVersion)
	_, err = c.CoreV1().Endpoints(m.Namespace).Update(ctx, e, metav1.UpdateOptions{})
	return err
}

func createDivertCRD(ctx context.Context, m *model.Manifest, in *networkingv1.Ingress) error {
	dClient, err := getDivertClient()
	if err != nil {
		return fmt.Errorf("error creating divert CRD client: %s", err.Error())
	}

	divertCRD := translateDivertCRD(m, in)

	old, err := dClient.Diverts(m.Namespace).Get(ctx, divertCRD.Name, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error getting divert CRD '%s'': %s", divertCRD.Name, err)
	}

	if old.Name == "" {
		oktetoLog.Infof("creating  divert CRD '%s'", divertCRD.Name)
		_, err = dClient.Diverts(m.Namespace).Create(ctx, divertCRD)
		if err != nil {
			return fmt.Errorf("error creating divert CRD '%s': %s", divertCRD.Name, err)
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
			return fmt.Errorf("error updating divert CRD '%s': %s", divertCRD.Name, err)
		}
		oktetoLog.Infof("updated divert CRD '%s'.", divertCRD.Name)
	}

	return nil
}

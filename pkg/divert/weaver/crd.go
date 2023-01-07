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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (d *Driver) createDivertCRD(ctx context.Context) error {
	divertCRD := translateDivertCRD(d.Manifest)

	old, err := d.DivertClient.Diverts(d.Manifest.Namespace).Get(ctx, divertCRD.Name, metav1.GetOptions{})
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return fmt.Errorf("error getting divert CRD '%s': %w", divertCRD.Name, err)
	}

	if old.Name == "" {
		oktetoLog.Infof("creating  divert CRD '%s'", divertCRD.Name)
		_, err = d.DivertClient.Diverts(d.Manifest.Namespace).Create(ctx, divertCRD)
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
		old.Status = diverts.DivertStatus{}
		_, err = d.DivertClient.Diverts(d.Manifest.Namespace).Update(ctx, old)
		if err != nil {
			if !k8sErrors.IsConflict(err) {
				return fmt.Errorf("error updating divert CRD '%s': %w", divertCRD.Name, err)
			}
		}
		oktetoLog.Infof("updated divert CRD '%s'.", divertCRD.Name)
	}

	return nil
}

func translateDivertCRD(m *model.Manifest) *diverts.Divert {
	result := &diverts.Divert{
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
		Spec: diverts.DivertSpec{
			Ingress: diverts.IngressDivertSpec{
				Value: m.Namespace,
			},
			FromService: diverts.ServiceDivertSpec{
				Name:      m.Deploy.Divert.Service,
				Namespace: m.Deploy.Divert.Namespace,
				Port:      m.Deploy.Divert.Port,
			},
			ToService: diverts.ServiceDivertSpec{
				Name:      m.Deploy.Divert.Service,
				Namespace: m.Namespace,
				Port:      m.Deploy.Divert.Port,
			},
			Deployment: diverts.DeploymentDivertSpec{
				Name:      m.Deploy.Divert.Deployment,
				Namespace: m.Deploy.Divert.Namespace,
			},
		},
	}
	return result
}

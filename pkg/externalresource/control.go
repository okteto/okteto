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

package externalresource

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/externalresource/k8s"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// K8sControl represents the controller that performs the actions with k8s
type K8sControl struct {
	ClientProvider func(*rest.Config) (k8s.ExternalResourceV1Interface, error)
	Cfg            *rest.Config
}

func NewExternalK8sControl(cfg *rest.Config) *K8sControl {
	return &K8sControl{
		ClientProvider: k8s.GetExternalClient,
		Cfg:            cfg,
	}
}

func (c *K8sControl) Deploy(ctx context.Context, name, ns string, er *ExternalResource) error {
	k8sclient, err := c.ClientProvider(c.Cfg)
	if err != nil {
		return fmt.Errorf("error creating external CRD client: %w", err)
	}

	externalResourceCRD := translate(name, ns, er, time.Now())

	old, err := k8sclient.ExternalResources(ns).Get(ctx, externalResourceCRD.Name, metav1.GetOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("error getting external resource CRD '%s': %w", externalResourceCRD.Name, err)
	}

	if old.Name == "" {
		oktetoLog.Infof("creating external resource CRD '%s'", externalResourceCRD.Name)
		_, err = k8sclient.ExternalResources(ns).Create(ctx, externalResourceCRD, metav1.CreateOptions{})
		if err != nil && !k8sErrors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating external resource CRD '%s': %w", externalResourceCRD.Name, err)
		}
		oktetoLog.Infof("created external resource CRD '%s'", externalResourceCRD.Name)
	} else {
		oktetoLog.Infof("updating external resource CRD '%s'", externalResourceCRD.Name)
		old.TypeMeta = externalResourceCRD.TypeMeta
		old.Annotations = externalResourceCRD.Annotations
		old.Labels = externalResourceCRD.Labels
		old.Spec = externalResourceCRD.Spec
		_, err = k8sclient.ExternalResources(ns).Update(ctx, old)
		if err != nil {
			if !k8sErrors.IsConflict(err) {
				return fmt.Errorf("error updating external resource CRD '%s': %w", externalResourceCRD.Name, err)
			}
		}
		oktetoLog.Infof("updated external resource CRD '%s'.", externalResourceCRD.Name)
	}

	return nil
}

func (c *K8sControl) List(ctx context.Context, ns string, labelSelector string) ([]ExternalResource, error) {
	k8sclient, err := c.ClientProvider(c.Cfg)
	if err != nil {
		return nil, fmt.Errorf("error providing external resource client: %w", err)
	}

	externals, err := k8sclient.ExternalResources(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing external resources: %w", err)
	}

	result := []ExternalResource{}
	for _, er := range externals.Items {
		result = append(result, translateK8sToExternal(er))
	}
	return result, nil
}

func translate(name, namespace string, externalResource *ExternalResource, now time.Time) *k8s.External {
	var externalEndpointsSpec []k8s.Endpoint
	for _, endpoint := range externalResource.Endpoints {
		externalEndpointsSpec = append(externalEndpointsSpec, k8s.Endpoint(*endpoint))
	}

	var notes *k8s.Notes
	if externalResource.Notes != nil {
		notes = &k8s.Notes{
			Path:     externalResource.Notes.Path,
			Markdown: externalResource.Notes.Markdown,
		}
	}

	return &k8s.External{
		TypeMeta: metav1.TypeMeta{
			Kind:       k8s.ExternalResourceKind,
			APIVersion: fmt.Sprintf("%s/%s", k8s.GroupName, k8s.GroupVersion),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      format.ResourceK8sMetaString(name),
			Namespace: namespace,
			Annotations: map[string]string{
				constants.LastUpdatedAnnotation: now.UTC().Format(constants.TimeFormat),
			},
			Labels: map[string]string{
				constants.OktetoNamespaceLabel: namespace,
			},
		},
		Spec: k8s.ExternalResourceSpec{
			Icon:      externalResource.Icon,
			Name:      format.ResourceK8sMetaString(name),
			Notes:     notes,
			Endpoints: externalEndpointsSpec,
		},
	}
}

func translateK8sToExternal(er k8s.External) ExternalResource {
	var notes *Notes
	if er.Spec.Notes != nil {
		notes = &Notes{
			Path:     er.Spec.Notes.Path,
			Markdown: er.Spec.Notes.Markdown,
		}
	}

	var endpoints []*ExternalEndpoint
	for _, ep := range er.Spec.Endpoints {
		endpoints = append(endpoints, translateK8sToEndpoint(ep))
	}
	return ExternalResource{
		Notes:     notes,
		Endpoints: endpoints,
	}
}

func translateK8sToEndpoint(k8sEndpoint k8s.Endpoint) *ExternalEndpoint {
	return &ExternalEndpoint{
		Name: k8sEndpoint.Name,
		Url:  k8sEndpoint.Url,
	}
}

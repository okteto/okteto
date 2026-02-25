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

package stack

import (
	"context"

	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

// ingressDeployer deploys endpoints using Kubernetes Ingress
type ingressDeployer struct {
	client    *ingresses.Client
	stackName string
	namespace string
}

func (d *ingressDeployer) DeployServiceEndpoint(ctx context.Context, name, serviceName string, port model.Port, stack *model.Stack) error {
	return deployK8sEndpoint(ctx, name, serviceName, port, stack, d.client)
}

func (d *ingressDeployer) DeployComposeEndpoint(ctx context.Context, name string, endpoint model.Endpoint, stack *model.Stack) error {
	translateOptions := &ingresses.TranslateOptions{
		Name:      format.ResourceK8sMetaString(d.stackName),
		Namespace: d.namespace,
	}
	ingress := ingresses.Translate(name, endpoint, translateOptions)
	// check for labels collision in the case of a compose - before creation or update (deploy)
	if skipIngressDeployForStackNameLabel(ctx, d.client, ingress) {
		return nil
	}
	return d.client.Deploy(ctx, ingress)
}

// NewIngressDeployer creates a new Ingress endpoint deployer
func NewIngressDeployer(client *ingresses.Client, stackName, namespace string) EndpointDeployer {
	return &ingressDeployer{
		client:    client,
		stackName: stackName,
		namespace: namespace,
	}
}

func skipIngressDeployForStackNameLabel(ctx context.Context, iClient *ingresses.Client, ingress *ingresses.Ingress) bool {
	// err is not checked here, we just want to check if the ingress already exists for this labels
	old, err := iClient.Get(ctx, ingress.GetName(), ingress.GetNamespace())
	if err != nil {
		oktetoLog.Infof("error getting ingress '%s': %s", ingress.GetName(), err)
		return false
	}
	if old != nil {
		if old.GetLabels()[model.StackNameLabel] == "" {
			oktetoLog.Warning("skipping deploy of %s due to name collision: the ingress '%s' was running before deploying your compose", old.GetName(), old.GetName())
			return true
		}
		if old.GetLabels()[model.StackNameLabel] != ingress.GetLabels()[model.StackNameLabel] {
			oktetoLog.Warning("skipping creation of endpoint '%s' due to name collision with endpoint in stack '%s'", ingress.GetName(), old.GetLabels()[model.StackNameLabel])
			return true
		}
	}
	return false
}

func deployK8sEndpoint(ctx context.Context, ingressName, svcName string, port model.Port, s *model.Stack, c *ingresses.Client) error {
	// create a new endpoint for this port ingress deployment
	endpoint := model.Endpoint{
		Labels:      translateLabels(svcName, s),
		Annotations: translateAnnotations(s.Services[svcName]),
		Rules: []model.EndpointRule{
			{
				Path:    "/",
				Service: svcName,
				Port:    port.ContainerPort,
			},
		},
	}
	// add specific stack labels
	if _, ok := endpoint.Labels[model.StackNameLabel]; !ok {
		endpoint.Labels[model.StackNameLabel] = format.ResourceK8sMetaString(s.Name)
	}
	if _, ok := endpoint.Labels[model.StackEndpointNameLabel]; !ok {
		endpoint.Labels[model.StackEndpointNameLabel] = ingressName
	}

	translateOptions := &ingresses.TranslateOptions{
		Name:      format.ResourceK8sMetaString(s.Name),
		Namespace: s.Namespace,
	}
	ingress := ingresses.Translate(ingressName, endpoint, translateOptions)

	// check for labels collision in the case of a compose - before creation or update (deploy)
	if skipIngressDeployForStackNameLabel(ctx, c, ingress) {
		return nil
	}
	return c.Deploy(ctx, ingress)
}

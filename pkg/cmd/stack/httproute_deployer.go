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
	"github.com/okteto/okteto/pkg/k8s/httproutes"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
)

// httpRouteDeployer deploys endpoints using Gateway API HTTPRoute
type httpRouteDeployer struct {
	client          *httproutes.Client
	stackName       string
	namespace       string
	clusterMetadata types.ClusterMetadata
}

func (d *httpRouteDeployer) DeployServiceEndpoint(ctx context.Context, name, serviceName string, port model.Port, stack *model.Stack) error {
	return deployK8sEndpointHTTPRoute(ctx, name, serviceName, port, stack, d.client, d.clusterMetadata)
}

func (d *httpRouteDeployer) DeployComposeEndpoint(ctx context.Context, name string, endpoint model.Endpoint, stack *model.Stack) error {
	translateOptions := &httproutes.TranslateOptions{
		Name:             format.ResourceK8sMetaString(d.stackName),
		Namespace:        d.namespace,
		GatewayName:      d.clusterMetadata.GatewayName,
		GatewayNamespace: d.clusterMetadata.GatewayNamespace,
	}
	httpRoute := httproutes.Translate(name, endpoint, translateOptions)
	return d.client.Deploy(ctx, httpRoute)
}

// NewHTTPRouteDeployer creates a new HTTPRoute endpoint deployer
func NewHTTPRouteDeployer(client *httproutes.Client, stackName, namespace string, clusterMetadata types.ClusterMetadata) EndpointDeployer {
	return &httpRouteDeployer{
		client:          client,
		stackName:       stackName,
		namespace:       namespace,
		clusterMetadata: clusterMetadata,
	}
}

func deployK8sEndpointHTTPRoute(ctx context.Context, httpRouteName, svcName string, port model.Port, s *model.Stack, c *httproutes.Client, metadata types.ClusterMetadata) error {
	// create a new endpoint for this port httproute deployment
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
		endpoint.Labels[model.StackEndpointNameLabel] = httpRouteName
	}

	translateOptions := &httproutes.TranslateOptions{
		Name:             format.ResourceK8sMetaString(s.Name),
		Namespace:        s.Namespace,
		GatewayName:      metadata.GatewayName,
		GatewayNamespace: metadata.GatewayNamespace,
	}
	httpRoute := httproutes.Translate(httpRouteName, endpoint, translateOptions)

	return c.Deploy(ctx, httpRoute)
}

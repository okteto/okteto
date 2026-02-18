// Copyright 2023-2025 The Okteto Authors
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

package httproutes

import (
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// TranslateOptions represents the options for translating endpoints to HTTPRoutes
type TranslateOptions struct {
	Name             string
	Namespace        string
	GatewayName      string
	GatewayNamespace string
}

// Translate translates the endpoints spec at compose or okteto manifest and returns an HTTPRoute
func Translate(endpointName string, endpoint model.Endpoint, opts *TranslateOptions) *gatewayv1.HTTPRoute {
	// endpointName could not be sanitized
	if endpointName == "" {
		// opts.Name is already sanitized- this should be clean version of name
		endpointName = opts.Name
	}

	pathMatchType := gatewayv1.PathMatchPathPrefix
	rules := make([]gatewayv1.HTTPRouteRule, 0)
	for _, rule := range endpoint.Rules {
		port := gatewayv1.PortNumber(rule.Port)
		httpRouteRule := gatewayv1.HTTPRouteRule{
			Matches: []gatewayv1.HTTPRouteMatch{
				{
					Path: &gatewayv1.HTTPPathMatch{
						Type:  &pathMatchType,
						Value: &rule.Path,
					},
				},
			},
			BackendRefs: []gatewayv1.HTTPBackendRef{
				{
					BackendRef: gatewayv1.BackendRef{
						BackendObjectReference: gatewayv1.BackendObjectReference{
							Name: gatewayv1.ObjectName(rule.Service),
							Port: &port,
						},
					},
				},
			},
		}
		rules = append(rules, httpRouteRule)
	}

	gatewayNamespace := gatewayv1.Namespace(opts.GatewayNamespace)
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:        format.ResourceK8sMetaString(endpointName),
			Namespace:   opts.Namespace,
			Labels:      setLabels(endpoint, opts),
			Annotations: setAnnotations(endpoint),
		},
		Spec: gatewayv1.HTTPRouteSpec{
			Rules: rules,
		},
	}

	// Set ParentRefs through CommonRouteSpec
	httpRoute.Spec.CommonRouteSpec.ParentRefs = []gatewayv1.ParentReference{
		{
			Name:      gatewayv1.ObjectName(opts.GatewayName),
			Namespace: &gatewayNamespace,
		},
	}

	return httpRoute
}

func setLabels(endpoint model.Endpoint, opts *TranslateOptions) map[string]string {
	// init with default label
	labels := model.Labels{
		model.DeployedByLabel: format.ResourceK8sMetaString(opts.Name),
	}

	// append labels from the endpoint spec
	for k := range endpoint.Labels {
		labels[k] = endpoint.Labels[k]
	}
	return labels
}

func setAnnotations(endpoint model.Endpoint) map[string]string {
	// init with default annotations from endpoint spec
	annotations := model.Annotations{}
	for k := range endpoint.Annotations {
		annotations[k] = endpoint.Annotations[k]
	}
	return annotations
}

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

package istio

import (
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/virtualservices"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	istioV1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

func (d *Driver) translateDivertVirtualService(vs *istioV1beta1.VirtualService, routes []string) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	httpRoutes := []*istioNetworkingV1beta1.HTTPRoute{}
	for i := range result.Spec.Http {
		if strings.HasPrefix(result.Spec.Http[i].Name, virtualservices.GetHTTPRoutePrefixOktetoName(d.namespace)) {
			continue
		}
		httpRoutes = append(httpRoutes, result.Spec.Http[i])
	}
	result.Spec.Http = httpRoutes
	httpRoutes = []*istioNetworkingV1beta1.HTTPRoute{}
	for _, httpRoute := range result.Spec.Http {
		if !matchHTTPRoute(httpRoute, routes) {
			continue
		}
		httpRoute := httpRoute.DeepCopy()
		httpRoute.Name = virtualservices.GetHTTPRouteOktetoName(d.namespace, httpRoute)
		for j := range httpRoute.Match {
			if httpRoute.Match[j].Headers == nil {
				httpRoute.Match[j].Headers = map[string]*istioNetworkingV1beta1.StringMatch{}
			}
			switch d.divert.Header.Match {
			case model.OktetoDivertIstioExactMatch:
				httpRoute.Match[j].Headers[d.divert.Header.Name] = &istioNetworkingV1beta1.StringMatch{
					MatchType: &istioNetworkingV1beta1.StringMatch_Exact{Exact: d.divert.Header.Value},
				}
			case model.OktetoDivertIstioRegexMatch:
				httpRoute.Match[j].Headers[d.divert.Header.Name] = &istioNetworkingV1beta1.StringMatch{
					MatchType: &istioNetworkingV1beta1.StringMatch_Regex{Regex: d.divert.Header.Value},
				}
			case model.OktetoDivertIstioPrefixMatch:
				httpRoute.Match[j].Headers[d.divert.Header.Name] = &istioNetworkingV1beta1.StringMatch{
					MatchType: &istioNetworkingV1beta1.StringMatch_Prefix{Prefix: d.divert.Header.Value},
				}
			}
		}
		for j := range httpRoute.Route {
			parts := strings.Split(httpRoute.Route[j].Destination.Host, ".")
			httpRoute.Route[j].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", parts[0], d.namespace)
		}
		httpRoutes = append(httpRoutes, httpRoute)
	}
	httpRoutes = append(httpRoutes, result.Spec.Http...)
	result.Spec.Http = httpRoutes
	return result
}

func matchHTTPRoute(r *istioNetworkingV1beta1.HTTPRoute, routes []string) bool {
	if len(routes) == 0 {
		return true
	}
	for _, routeName := range routes {
		if r.Name == routeName {
			return true
		}
	}
	return false
}

func (d *Driver) restoreDivertVirtualService(vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	httpRoutes := []*istioNetworkingV1beta1.HTTPRoute{}
	for i := range result.Spec.Http {
		if strings.HasPrefix(result.Spec.Http[i].Name, virtualservices.GetHTTPRoutePrefixOktetoName(d.namespace)) {
			continue
		}
		httpRoutes = append(httpRoutes, result.Spec.Http[i])
	}
	result.Spec.Http = httpRoutes
	return result
}

func (d *Driver) translateDivertHost(vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, d.name)
	labels.SetInMetadata(&result.ObjectMeta, model.OktetoAutoCreateAnnotation, "true")
	result.Namespace = d.namespace
	result.ResourceVersion = ""
	result.UID = types.UID("")
	result.Spec.Tls = nil
	result.Spec.Hosts = []string{
		fmt.Sprintf("%s-%s.%s", result.Name, d.namespace, okteto.GetSubdomain()),
		fmt.Sprintf("%s.%s.svc.cluster.local", result.Name, d.namespace),
	}

	result.Spec = d.injectDivertHeader(result.Spec)

	for i := range result.Spec.Http {

		for j := range result.Spec.Http[i].Route {
			if !strings.Contains(result.Spec.Http[i].Route[j].Destination.Host, ".") {
				result.Spec.Http[i].Route[j].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", result.Spec.Http[i].Route[j].Destination.Host, vs.Namespace)
			}
		}
	}
	return result
}

func (d *Driver) injectDivertHeader(vsSpec istioNetworkingV1beta1.VirtualService) istioNetworkingV1beta1.VirtualService {
	for i := range vsSpec.Http {
		if vsSpec.Http[i].Headers == nil {
			vsSpec.Http[i].Headers = &istioNetworkingV1beta1.Headers{}
		}
		if vsSpec.Http[i].Headers.Request == nil {
			vsSpec.Http[i].Headers.Request = &istioNetworkingV1beta1.Headers_HeaderOperations{}
		}
		if vsSpec.Http[i].Headers.Request.Set == nil {
			vsSpec.Http[i].Headers.Request.Set = map[string]string{}
		}
		vsSpec.Http[i].Headers.Request.Set[model.OktetoDivertDefaultHeaderName] = d.namespace
	}
	return vsSpec
}

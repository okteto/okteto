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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createIntoDeveloperVirtualService(m *model.Manifest, vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := &istioV1beta1.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:        vs.Name,
			Namespace:   m.Namespace,
			Labels:      vs.Labels,
			Annotations: vs.Annotations,
		},
		Spec: *vs.Spec.DeepCopy(),
	}
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = "true"
	labels.SetInMetadata(&result.ObjectMeta, model.DeployedByLabel, m.Name)

	return translateDeveloperVirtualService(m, result)
}

func translateDeveloperVirtualService(m *model.Manifest, vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	result.Spec.Hosts = []string{
		fmt.Sprintf("%s-%s.%s", result.Name, m.Namespace, okteto.GetSubdomain()),
		fmt.Sprintf("%s.%s.svc.cluster.local", result.Name, m.Namespace),
	}
	result.Spec.Tls = nil

	for i := range result.Spec.Http {
		if result.Spec.Http[i].Headers == nil {
			result.Spec.Http[i].Headers = &istioNetworkingV1beta1.Headers{}
		}
		if result.Spec.Http[i].Headers.Request == nil {
			result.Spec.Http[i].Headers.Request = &istioNetworkingV1beta1.Headers_HeaderOperations{}
		}
		if result.Spec.Http[i].Headers.Request.Set == nil {
			result.Spec.Http[i].Headers.Request.Set = map[string]string{}
		}
		result.Spec.Http[i].Headers.Request.Set[model.OktetoDivertHeader] = m.Namespace

		for j := range result.Spec.Http[i].Route {
			if !strings.Contains(result.Spec.Http[i].Route[j].Destination.Host, ".") {
				result.Spec.Http[i].Route[j].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", result.Spec.Http[i].Route[j].Destination.Host, m.Deploy.Divert.Namespace)
			}
		}
	}
	return result
}

func translateDivertVirtualService(m *model.Manifest, vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	httpRoutes := []*istioNetworkingV1beta1.HTTPRoute{}
	for i := range result.Spec.Http {
		if strings.HasPrefix(result.Spec.Http[i].Name, virtualservices.GetHTTPRoutePrefixOktetoName(m.Namespace)) {
			continue
		}
		httpRoutes = append(httpRoutes, result.Spec.Http[i])
	}
	result.Spec.Http = httpRoutes
	httpRoutes = []*istioNetworkingV1beta1.HTTPRoute{}
	for _, httpRoute := range result.Spec.Http {
		httpRoute := httpRoute.DeepCopy()
		httpRoute.Name = virtualservices.GetHTTPRouteOktetoName(m.Namespace, httpRoute)
		for j := range httpRoute.Match {
			if httpRoute.Match[j].Headers == nil {
				httpRoute.Match[j].Headers = map[string]*istioNetworkingV1beta1.StringMatch{}
			}
			httpRoute.Match[j].Headers[model.OktetoDivertHeader] = &istioNetworkingV1beta1.StringMatch{
				MatchType: &istioNetworkingV1beta1.StringMatch_Exact{Exact: m.Namespace},
			}
		}
		matchService := false
		for j := range httpRoute.Route {
			parts := strings.Split(httpRoute.Route[j].Destination.Host, ".")
			if parts[0] == m.Deploy.Divert.Service {
				httpRoute.Route[j].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", parts[0], m.Namespace)
				matchService = true
			}
		}
		if matchService {
			httpRoutes = append(httpRoutes, httpRoute)
		}
	}
	httpRoutes = append(httpRoutes, result.Spec.Http...)
	result.Spec.Http = httpRoutes
	return result
}

func restoreDivertVirtualService(m *model.Manifest, vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	httpRoutes := []*istioNetworkingV1beta1.HTTPRoute{}
	for i := range result.Spec.Http {
		if strings.HasPrefix(result.Spec.Http[i].Name, virtualservices.GetHTTPRoutePrefixOktetoName(m.Namespace)) {
			continue
		}
		httpRoutes = append(httpRoutes, result.Spec.Http[i])
	}
	result.Spec.Http = httpRoutes
	return result
}

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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	istioV1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

func (d *Driver) getDivertAnnotationName() string {
	divertHash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", d.namespace, d.name)))

	return fmt.Sprintf(constants.OktetoDivertAnnotationTemplate, hex.EncodeToString(divertHash[:20]))
}

func (d *Driver) getDeprecatedDivertAnnotationName() string {
	return fmt.Sprintf(constants.OktetoDeprecatedDivertAnnotationTemplate, d.namespace, d.name)
}

func (d *Driver) translateDivertVirtualService(vs *istioV1beta1.VirtualService, routes []string) (*istioV1beta1.VirtualService, error) {
	result := vs.DeepCopy()
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	annotation := DivertTransformation{
		Namespace: d.namespace,
		Routes:    routes,
	}
	bytes, err := json.Marshal(annotation)
	if err != nil {
		return nil, err
	}
	result.Annotations[d.getDivertAnnotationName()] = string(bytes)
	return result, nil
}

func (d *Driver) restoreDivertVirtualService(vs *istioV1beta1.VirtualService) *istioV1beta1.VirtualService {
	result := vs.DeepCopy()
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	delete(result.Annotations, d.getDivertAnnotationName())
	delete(result.Annotations, d.getDeprecatedDivertAnnotationName())
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

	d.injectDivertHeader(&result.Spec)

	for i := range result.Spec.Http {

		for j := range result.Spec.Http[i].Route {
			if !strings.Contains(result.Spec.Http[i].Route[j].Destination.Host, ".") {
				result.Spec.Http[i].Route[j].Destination.Host = fmt.Sprintf("%s.%s.svc.cluster.local", result.Spec.Http[i].Route[j].Destination.Host, vs.Namespace)
			}
		}
	}
	return result
}

func (d *Driver) injectDivertHeader(vsSpec *istioNetworkingV1beta1.VirtualService) {
	for i := range vsSpec.Http {
		if vsSpec.Http[i].Headers == nil {
			vsSpec.Http[i].Headers = &istioNetworkingV1beta1.Headers{}
		}
		if vsSpec.Http[i].Headers.Request == nil {
			vsSpec.Http[i].Headers.Request = &istioNetworkingV1beta1.Headers_HeaderOperations{}
		}
		if vsSpec.Http[i].Headers.Request.Add == nil {
			vsSpec.Http[i].Headers.Request.Add = map[string]string{}
		}
		vsSpec.Http[i].Headers.Request.Add[constants.OktetoDivertBaggageHeader] = fmt.Sprintf("%s=%s", constants.OktetoDivertHeaderName, d.namespace)
	}
}

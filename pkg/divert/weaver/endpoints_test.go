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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func Test_translateEndpoints(t *testing.T) {
	s := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			UID:         types.UID("my-uid"),
			Name:        "name",
			Namespace:   "staging",
			Labels:      map[string]string{"l1": "v1"},
			Annotations: map[string]string{"a1": "v1"},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: "my-ip",
			Ports: []apiv1.ServicePort{
				{
					Name:        "port1",
					Port:        8080,
					TargetPort:  intstr.IntOrString{IntVal: 9090},
					Protocol:    apiv1.ProtocolTCP,
					AppProtocol: pointer.String("tcp"),
				},
				{
					Name:        "port2",
					Port:        8081,
					TargetPort:  intstr.IntOrString{IntVal: 9091},
					Protocol:    apiv1.ProtocolTCP,
					AppProtocol: pointer.String("tcp"),
				},
			},
		},
	}
	expected := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v1",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v1",
			},
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "my-ip",
						TargetRef: &apiv1.ObjectReference{
							Kind:            "Service",
							Namespace:       s.Namespace,
							Name:            s.Name,
							UID:             s.UID,
							APIVersion:      "v1",
							ResourceVersion: s.ResourceVersion,
						},
					},
				},
				Ports: []apiv1.EndpointPort{
					{
						Name:        "port1",
						Port:        8080,
						Protocol:    apiv1.ProtocolTCP,
						AppProtocol: pointer.String("tcp"),
					},
					{
						Name:        "port2",
						Port:        8081,
						Protocol:    apiv1.ProtocolTCP,
						AppProtocol: pointer.String("tcp"),
					},
				},
			},
		},
	}
	result := translateEndpoints("test", "cindy", s)
	assert.Equal(t, result, expected)
}

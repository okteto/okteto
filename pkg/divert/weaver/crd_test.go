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
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_translateDivertCRD(t *testing.T) {
	m := &model.Manifest{
		Name:      "test",
		Namespace: "cindy",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace:  "staging",
				Service:    "service",
				Deployment: "deployment",
				Port:       8080,
			},
		},
	}
	expected := &diverts.Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel:    "test",
				"dev.okteto.com/version": "0.1.9",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
			},
		},
		Spec: diverts.DivertSpec{
			Ingress: diverts.IngressDivertSpec{
				Value: "cindy",
			},
			FromService: diverts.ServiceDivertSpec{
				Name:      "service",
				Namespace: "staging",
				Port:      8080,
			},
			ToService: diverts.ServiceDivertSpec{
				Name:      "service",
				Namespace: "cindy",
				Port:      8080,
			},
			Deployment: diverts.DeploymentDivertSpec{
				Name:      "deployment",
				Namespace: "staging",
			},
		},
	}
	result := translateDivertCRD(m)
	assert.True(t, reflect.DeepEqual(result, expected))
}

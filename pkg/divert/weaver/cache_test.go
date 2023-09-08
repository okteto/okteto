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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_initCache(t *testing.T) {
	ctx := context.Background()
	i1 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i1",
			Namespace: "cindy",
		},
	}
	i2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i2",
			Namespace: "other",
		},
	}
	i3 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i3",
			Namespace: "staging",
		},
	}
	s1 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "cindy",
		},
	}
	s2 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s2",
			Namespace: "other",
		},
	}
	s3 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3",
			Namespace: "staging",
		},
	}
	e1 := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e1",
			Namespace: "cindy",
		},
	}
	e2 := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2",
			Namespace: "other",
		},
	}
	c := fake.NewSimpleClientset(i1, i2, i3, s1, s2, s3, e1, e2)
	m := &model.Manifest{
		Name:      "test",
		Namespace: "cindy",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace: "staging",
			},
		},
	}

	d := &Driver{client: c, name: m.Name, namespace: m.Namespace, divert: *m.Deploy.Divert}
	err := d.initCache(ctx)
	assert.NoError(t, err)

	assert.Equal(t, map[string]*networkingv1.Ingress{"i1": i1}, d.cache.developerIngresses)
	assert.Equal(t, map[string]*networkingv1.Ingress{"i3": i3}, d.cache.divertIngresses)
	assert.Equal(t, map[string]*apiv1.Service{"s1": s1}, d.cache.developerServices)
	assert.Equal(t, map[string]*apiv1.Service{"s3": s3}, d.cache.divertServices)
	assert.Equal(t, map[string]*apiv1.Endpoints{"e1": e1}, d.cache.developerEndpoints)
}

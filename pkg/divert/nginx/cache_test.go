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

package nginx

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	div := &k8s.Divert{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-divert-1",
			Namespace: "cindy",
		},
	}
	c := fake.NewClientset(i1, i2, i3, s1, s2, s3)
	m := &model.Manifest{
		Name: "test",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace: "staging",
			},
		},
	}

	divertManager := &fakeDivertManager{}
	divertManager.On("List", mock.Anything, "cindy").Return([]*k8s.Divert{div}, nil)

	d := &Driver{client: c, name: m.Name, namespace: "cindy", divert: *m.Deploy.Divert, divertManager: divertManager}
	err := d.initCache(ctx)
	assert.NoError(t, err)

	assert.Equal(t, map[string]*networkingv1.Ingress{"i1": i1}, d.cache.developerIngresses)
	assert.Equal(t, map[string]*networkingv1.Ingress{"i3": i3}, d.cache.divertIngresses)
	assert.Equal(t, map[string]*apiv1.Service{"s1": s1}, d.cache.developerServices)
	assert.Equal(t, map[string]*apiv1.Service{"s3": s3}, d.cache.divertServices)
	assert.Equal(t, map[string]*k8s.Divert{"test-divert-1": div}, d.cache.divertResources)

	divertManager.AssertExpectations(t)
}

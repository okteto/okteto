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

package gatewayapi

import (
	"context"
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newIngress returns an ingress routing the host "<name>-<namespace>.okteto.dev" to the given service
func newIngress(name, namespace, service string, annotations map[string]string) *networkingv1.Ingress {
	host := name + "-" + namespace + ".okteto.dev"
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      map[string]string{model.DeployedByLabel: "test"},
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: service,
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{host},
				},
			},
		},
	}
}

func newIngressDriver(c *fake.Clientset, in *networkingv1.Ingress) *Driver {
	return &Driver{
		client:    c,
		name:      "test",
		namespace: "cindy",
		divert:    model.DivertDeploy{Namespace: "staging"},
		cache: &cache{
			developerIngresses: map[string]*networkingv1.Ingress{in.Name: in},
		},
	}
}

func Test_divertIngress_AddsHeaderAnnotation(t *testing.T) {
	ctx := context.Background()
	in := newIngress("i1", "cindy", "s1", map[string]string{"a1": "v1"})
	c := fake.NewClientset(in)
	d := newIngressDriver(c, in)

	err := d.divertIngress(ctx, in)
	require.NoError(t, err)

	expected := newIngress("i1", "cindy", "s1", map[string]string{
		"a1":                               "v1",
		model.OktetoDivertHeaderAnnotation: "cindy",
	})
	result, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, expected.Annotations, result.Annotations)
	require.Equal(t, expected.Labels, result.Labels)
	require.Equal(t, expected.Spec, result.Spec)
	require.Equal(t, expected.Annotations, d.cache.developerIngresses["i1"].Annotations)
}

func Test_divertIngress_NilAnnotations(t *testing.T) {
	ctx := context.Background()
	in := newIngress("i1", "cindy", "s1", nil)
	c := fake.NewClientset(in)
	d := newIngressDriver(c, in)

	err := d.divertIngress(ctx, in)
	require.NoError(t, err)

	result, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{model.OktetoDivertHeaderAnnotation: "cindy"}, result.Annotations)
}

func Test_divertIngress_AlreadyAnnotated(t *testing.T) {
	ctx := context.Background()
	in := newIngress("i1", "cindy", "s1", map[string]string{model.OktetoDivertHeaderAnnotation: "cindy"})
	c := fake.NewClientset(in)
	d := newIngressDriver(c, in)

	err := d.divertIngress(ctx, in)
	require.NoError(t, err)
	require.Empty(t, c.Actions())
}

func Test_divertIngress_UpdateError(t *testing.T) {
	ctx := context.Background()
	in := newIngress("i1", "cindy", "s1", nil)
	c := fake.NewClientset(in)
	c.PrependReactor("update", "ingresses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("update error")
	})
	d := newIngressDriver(c, in)

	err := d.divertIngress(ctx, in)
	require.EqualError(t, err, "update error")
}

func conflictOnUpdate(k8stesting.Action) (bool, runtime.Object, error) {
	return true, nil, k8sErrors.NewConflict(schema.GroupResource{Resource: "ingresses"}, "i1", errors.New("conflict"))
}

func Test_divertIngress_UpdateConflict(t *testing.T) {
	ctx := context.Background()
	in := newIngress("i1", "cindy", "s1", nil)
	current := newIngress("i1", "cindy", "s1", map[string]string{"a1": "v2"})
	c := fake.NewClientset(current)
	c.PrependReactor("update", "ingresses", conflictOnUpdate)
	d := newIngressDriver(c, in)

	err := d.divertIngress(ctx, in)
	require.NoError(t, err)
	require.Equal(t, current.Annotations, d.cache.developerIngresses["i1"].Annotations)
}

func Test_divertIngress_GetAfterConflictError(t *testing.T) {
	ctx := context.Background()
	in := newIngress("i1", "cindy", "s1", nil)
	c := fake.NewClientset(in)
	c.PrependReactor("update", "ingresses", conflictOnUpdate)
	c.PrependReactor("get", "ingresses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("get error")
	})
	d := newIngressDriver(c, in)

	err := d.divertIngress(ctx, in)
	require.EqualError(t, err, "get error")
}

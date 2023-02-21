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

package virtualservices

import (
	"context"
	"fmt"

	istioNetowrkingV1beta1 "istio.io/api/networking/v1beta1"
	istioV1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclientset "istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Create creates an istio virtual services
func Create(ctx context.Context, vs *istioV1beta1.VirtualService, c istioclientset.Interface) error {
	_, err := c.NetworkingV1beta1().VirtualServices(vs.Namespace).Create(ctx, vs, metav1.CreateOptions{})
	return err
}

// Update updates an istio virtual services
func Update(ctx context.Context, vs *istioV1beta1.VirtualService, c istioclientset.Interface) error {
	_, err := c.NetworkingV1beta1().VirtualServices(vs.Namespace).Update(ctx, vs, metav1.UpdateOptions{})
	return err
}

// Get get an istio virtual services by name
func Get(ctx context.Context, name, namespace string, c istioclientset.Interface) (*istioV1beta1.VirtualService, error) {
	return c.NetworkingV1beta1().VirtualServices(namespace).Get(ctx, name, metav1.GetOptions{})
}

// List list istio virtual services
func List(ctx context.Context, namespace string, c istioclientset.Interface) ([]*istioV1beta1.VirtualService, error) {
	vsList, err := c.NetworkingV1beta1().VirtualServices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return vsList.Items, nil
}

func GetHTTPRoutePrefixOktetoName(ns string) string {
	return fmt.Sprintf("okteto-divert-%s", ns)
}

func GetHTTPRouteOktetoName(ns string, r *istioNetowrkingV1beta1.HTTPRoute) string {
	if r.Name == "" {
		return fmt.Sprintf("okteto-divert-%s", ns)
	}
	return fmt.Sprintf("okteto-divert-%s-%s", ns, r.Name)
}

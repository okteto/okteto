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

package weaver

import (
	"context"

	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cache struct {
	divertIngresses    map[string]*networkingv1.Ingress
	divertServices     map[string]*apiv1.Service
	divertEndpoints    map[string]*apiv1.Endpoints
	developerIngresses map[string]*networkingv1.Ingress
	developerServices  map[string]*apiv1.Service
	developerEndpoints map[string]*apiv1.Endpoints
}

func (d *Driver) initCache(ctx context.Context) error {
	d.cache = &cache{
		divertIngresses:    map[string]*networkingv1.Ingress{},
		divertServices:     map[string]*apiv1.Service{},
		divertEndpoints:    map[string]*apiv1.Endpoints{},
		developerIngresses: map[string]*networkingv1.Ingress{},
		developerServices:  map[string]*apiv1.Service{},
		developerEndpoints: map[string]*apiv1.Endpoints{},
	}
	// Init ingress cache for diverted namespace
	iList, err := d.Client.NetworkingV1().Ingresses(d.Manifest.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range iList.Items {
		d.cache.divertIngresses[iList.Items[i].Name] = &iList.Items[i]
	}

	// Service cache for diverted namespace
	sList, err := d.Client.CoreV1().Services(d.Manifest.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range sList.Items {
		d.cache.divertServices[sList.Items[i].Name] = &sList.Items[i]
	}

	// Endpoints cache for diverted namespace
	eList, err := d.Client.CoreV1().Endpoints(d.Manifest.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range eList.Items {
		d.cache.divertEndpoints[eList.Items[i].Name] = &eList.Items[i]
	}

	// Ingress cache for developer namespace
	iList, err = d.Client.NetworkingV1().Ingresses(d.Manifest.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range iList.Items {
		d.cache.developerIngresses[iList.Items[i].Name] = &iList.Items[i]
	}

	// Service cache for developer namespace
	sList, err = d.Client.CoreV1().Services(d.Manifest.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range sList.Items {
		d.cache.developerServices[sList.Items[i].Name] = &sList.Items[i]
	}

	// Endpoints cache for developer namespace
	eList, err = d.Client.CoreV1().Endpoints(d.Manifest.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for i := range eList.Items {
		d.cache.developerEndpoints[eList.Items[i].Name] = &eList.Items[i]
	}

	return nil
}

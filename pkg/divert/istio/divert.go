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

package istio

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/k8s/virtualservices"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	istioclientset "istio.io/client-go/pkg/clientset/versioned"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

const (
	UPDATE_CONFLICT_RETRIES = 20
)

// Driver weaver struct for the divert driver
type Driver struct {
	name        string
	namespace   string
	divert      model.DivertDeploy
	client      kubernetes.Interface
	istioClient istioclientset.Interface
}

func New(m *model.Manifest, c kubernetes.Interface, ic istioclientset.Interface) *Driver {
	return &Driver{
		name:        m.Name,
		namespace:   m.Namespace,
		divert:      *m.Deploy.Divert,
		client:      c,
		istioClient: ic,
	}
}

func (d *Driver) Deploy(ctx context.Context) error {
	oktetoLog.Spinner(fmt.Sprintf("Diverting service %s/%s...", d.divert.Namespace, d.divert.Service))
	if err := d.retryTranslateDivertService(ctx); err != nil {
		return err
	}

	for i := range d.divert.Hosts {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			oktetoLog.Spinner(fmt.Sprintf("Diverting host %s/%s...", d.divert.Hosts[i].Namespace, d.divert.Hosts[i].VirtualService))
			if err := d.retryTranslateDivertHost(ctx, d.divert.Hosts[i]); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Driver) Destroy(ctx context.Context) error {
	oktetoLog.Spinner(fmt.Sprintf("Destroying divert service %s/%s ...", d.divert.Namespace, d.divert.Service))
	var err error
	for retries := 0; retries < UPDATE_CONFLICT_RETRIES; retries++ {
		vs, err := virtualservices.Get(ctx, d.divert.VirtualService, d.divert.Namespace, d.istioClient)
		if err != nil {
			return err
		}
		restoredVS := d.restoreDivertService(vs)

		err = virtualservices.Update(ctx, restoredVS, d.istioClient)
		if err == nil {
			return nil
		}
		if !k8sErrors.IsConflict(err) {
			return err
		}
	}
	return err
}

func (d *Driver) UpdatePod(pod apiv1.PodSpec) apiv1.PodSpec {
	return pod
}

func (d *Driver) UpdateVirtualService(vs istioNetworkingV1beta1.VirtualService) istioNetworkingV1beta1.VirtualService {
	return d.injectDivertHeader(vs)
}

func (d *Driver) retryTranslateDivertService(ctx context.Context) error {
	var err error
	for retries := 0; retries < UPDATE_CONFLICT_RETRIES; retries++ {
		vs, err := virtualservices.Get(ctx, d.divert.VirtualService, d.divert.Namespace, d.istioClient)
		if err != nil {
			return err
		}
		translatedVS := d.translateDivertService(vs)
		err = virtualservices.Update(ctx, translatedVS, d.istioClient)
		if err == nil {
			return nil
		}
		if !k8sErrors.IsConflict(err) {
			return err
		}
	}
	return err
}

func (d *Driver) retryTranslateDivertHost(ctx context.Context, divertHost model.DivertHost) error {
	var err error
	for retries := 0; retries < UPDATE_CONFLICT_RETRIES; retries++ {
		vs, err := virtualservices.Get(ctx, divertHost.VirtualService, divertHost.Namespace, d.istioClient)
		if err != nil {
			return err
		}
		translatedVS := d.translateDivertHost(vs)

		devVS, err := virtualservices.Get(ctx, divertHost.VirtualService, d.namespace, d.istioClient)
		if k8sErrors.IsNotFound(err) {
			err = virtualservices.Create(ctx, translatedVS, d.istioClient)
			if err == nil {
				return nil
			}
			if k8sErrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}

		if devVS.Labels[model.OktetoAutoCreateAnnotation] != "true" {
			oktetoLog.Infof("Ignoring host '%s/%s', virtual service '%s/%s'", divertHost.Namespace, divertHost.VirtualService, d.namespace, divertHost.VirtualService)
			return nil
		}

		translatedVS.ResourceVersion = devVS.ResourceVersion
		err = virtualservices.Update(ctx, translatedVS, d.istioClient)
		if err == nil {
			return nil
		}
		if !k8sErrors.IsConflict(err) {
			return err
		}
	}
	return err
}

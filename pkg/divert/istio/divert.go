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
	client      kubernetes.Interface
	istioClient istioclientset.Interface
	name        string
	namespace   string
	divert      model.DivertDeploy
}

// DivertTransformation represents the annotation for the okteto mutation webhook to divert a virtual service
type DivertTransformation struct {
	Namespace string   `json:"namespace"`
	Routes    []string `json:"routes,omitempty"`
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
	for i := range d.divert.VirtualServices {
		oktetoLog.Spinner(fmt.Sprintf("Diverting virtual service %s/%s...", d.divert.VirtualServices[i].Namespace, d.divert.VirtualServices[i].Name))
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
		if err := d.retryTranslateDivertVirtualService(ctx, d.divert.VirtualServices[i]); err != nil {
			return err
		}
		oktetoLog.StopSpinner()
		oktetoLog.Success("Virtual service '%s/%s' successfully diverted", d.divert.VirtualServices[i].Namespace, d.divert.VirtualServices[i].Name)
	}

	for i := range d.divert.Hosts {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			oktetoLog.Spinner(fmt.Sprintf("Diverting host %s/%s...", d.divert.Hosts[i].Namespace, d.divert.Hosts[i].VirtualService))
			oktetoLog.StartSpinner()
			defer oktetoLog.StopSpinner()
			if err := d.retryTranslateDivertHost(ctx, d.divert.Hosts[i]); err != nil {
				return err
			}
			oktetoLog.StopSpinner()
			oktetoLog.Success("Host '%s/%s' successfully diverted", d.divert.Hosts[i].Namespace, d.divert.Hosts[i].VirtualService)
		}
	}

	return nil
}

func (d *Driver) Destroy(ctx context.Context) error {
	var err error
	for i := range d.divert.VirtualServices {
		oktetoLog.Spinner(fmt.Sprintf("Restoring virtual service %s/%s...", d.divert.VirtualServices[i].Namespace, d.divert.VirtualServices[i].Name))
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
		for retries := 0; retries < UPDATE_CONFLICT_RETRIES; retries++ {
			vs, err := virtualservices.Get(ctx, d.divert.VirtualServices[i].Name, d.divert.VirtualServices[i].Namespace, d.istioClient)
			if err != nil {
				return err
			}
			restoredVS := d.restoreDivertVirtualService(vs)

			err = virtualservices.Update(ctx, restoredVS, d.istioClient)
			if err == nil {
				oktetoLog.StopSpinner()
				oktetoLog.Success("Virtual service '%s/%s' successfully restored", d.divert.VirtualServices[i].Namespace, d.divert.VirtualServices[i].Name)
				break
			}
			if !k8sErrors.IsConflict(err) {
				return err
			}
		}
		if err != nil {
			return err
		}
	}
	return nil

}

func (d *Driver) UpdatePod(pod apiv1.PodSpec) apiv1.PodSpec {
	return pod
}

func (d *Driver) UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService) {
	d.injectDivertHeader(vs)
}

func (d *Driver) retryTranslateDivertVirtualService(ctx context.Context, divertVS model.DivertVirtualService) error {
	var err error
	for retries := 0; retries < UPDATE_CONFLICT_RETRIES; retries++ {
		vs, err := virtualservices.Get(ctx, divertVS.Name, divertVS.Namespace, d.istioClient)
		if err != nil {
			return err
		}
		translatedVS, err := d.translateDivertVirtualService(vs, divertVS.Routes)
		if err != nil {
			return err
		}
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

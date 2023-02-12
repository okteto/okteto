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
	"reflect"

	"github.com/okteto/okteto/pkg/k8s/virtualservices"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	istioV1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	istioclientset "istio.io/client-go/pkg/clientset/versioned"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

const (
	UPDATE_CONFLICT_RETRIES = 10
)

// Driver weaver struct for the divert driver
type Driver struct {
	Client      kubernetes.Interface
	IstioClient istioclientset.Interface
	Manifest    *model.Manifest
	cache       *divertIstioCache
}

type divertIstioCache struct {
	divertVirtualServices    map[string]*istioV1beta1.VirtualService
	developerVirtualServices map[string]*istioV1beta1.VirtualService
}

func (d *Driver) Deploy(ctx context.Context) error {
	for name := range d.cache.divertVirtualServices {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			oktetoLog.Spinner(fmt.Sprintf("Diverting virtual service %s/%s ...", d.Manifest.Deploy.Divert.Namespace, name))
			if err := d.retryTranslateDivertVirtualService(ctx, name); err != nil {
				return err
			}

			if !isInDivertVirtualServices(name, d.Manifest.Deploy.Divert.VirtualServices) {
				continue
			}
			if err := d.retryTranslateDeveloperVirtualService(ctx, name); err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Driver) Destroy(ctx context.Context) error {
	for name := range d.cache.divertVirtualServices {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("destroyDivert context cancelled")
			return ctx.Err()
		default:
			oktetoLog.Spinner(fmt.Sprintf("Destroying divert un virtual service %s/%s ...", d.Manifest.Deploy.Divert.Namespace, name))
			if err := d.retryRestoreDivertVirtualService(ctx, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Driver) GetDivertNamespace() string {
	if d.Manifest.Deploy.Divert.Namespace == d.Manifest.Namespace {
		return ""
	}
	return d.Manifest.Deploy.Divert.Namespace
}

func (d *Driver) InitCache(ctx context.Context) error {
	d.cache = &divertIstioCache{
		divertVirtualServices:    map[string]*istioV1beta1.VirtualService{},
		developerVirtualServices: map[string]*istioV1beta1.VirtualService{},
	}
	vsList, err := virtualservices.List(ctx, d.Manifest.Deploy.Divert.Namespace, d.IstioClient)
	if err != nil {
		return err
	}
	for i := range vsList {
		d.cache.divertVirtualServices[vsList[i].Name] = vsList[i]
	}

	vsList, err = virtualservices.List(ctx, d.Manifest.Namespace, d.IstioClient)
	if err != nil {
		return err
	}
	for i := range vsList {
		d.cache.developerVirtualServices[vsList[i].Name] = vsList[i]
	}
	return nil
}

func (d *Driver) retryTranslateDeveloperVirtualService(ctx context.Context, name string) error {
	retries := 0
	var err error
	for retries < UPDATE_CONFLICT_RETRIES {
		if _, ok := d.cache.developerVirtualServices[name]; !ok {
			vs := createIntoDeveloperVirtualService(d.Manifest, d.cache.divertVirtualServices[name])
			err = virtualservices.Create(ctx, vs, d.IstioClient)
			if err == nil {
				d.cache.developerVirtualServices[name] = vs
				return nil
			}
			if !k8sErrors.IsAlreadyExists(err) {
				return err
			}
			return nil
		}

		vs := translateDeveloperVirtualService(d.Manifest, d.cache.developerVirtualServices[name])
		if vs.Annotations[model.OktetoAutoCreateAnnotation] != "" {
			vs = createIntoDeveloperVirtualService(d.Manifest, d.cache.divertVirtualServices[name])
			vs.ResourceVersion = d.cache.developerVirtualServices[name].ResourceVersion
		}
		if isEqualVirtualService(vs, d.cache.developerVirtualServices[name]) {
			return nil
		}

		err = virtualservices.Update(ctx, vs, d.IstioClient)
		if err == nil {
			d.cache.developerVirtualServices[name] = vs
			return nil
		}
		if !k8sErrors.IsConflict(err) {
			return err
		}
		retries++
	}
	return err
}

func (d *Driver) retryTranslateDivertVirtualService(ctx context.Context, name string) error {
	retries := 0
	var err error
	for retries < UPDATE_CONFLICT_RETRIES {
		vs := translateDivertVirtualService(d.Manifest, d.cache.divertVirtualServices[name])
		if isEqualVirtualService(vs, d.cache.divertVirtualServices[name]) {
			return nil
		}

		err = virtualservices.Update(ctx, vs, d.IstioClient)
		if err == nil {
			d.cache.divertVirtualServices[name] = vs
			return nil
		}
		if !k8sErrors.IsConflict(err) {
			return err
		}
		retries++
	}
	return err
}

func (d *Driver) retryRestoreDivertVirtualService(ctx context.Context, name string) error {
	retries := 0
	var err error
	for retries < UPDATE_CONFLICT_RETRIES {
		vs := restoreDivertVirtualService(d.Manifest, d.cache.divertVirtualServices[name])
		if isEqualVirtualService(vs, d.cache.divertVirtualServices[name]) {
			return nil
		}

		err = virtualservices.Update(ctx, vs, d.IstioClient)
		if err == nil {
			d.cache.divertVirtualServices[name] = vs
			return nil
		}
		if !k8sErrors.IsConflict(err) {
			return err
		}
		retries++
	}
	return err
}

func isInDivertVirtualServices(name string, virtualServices []string) bool {
	for _, vs := range virtualServices {
		switch vs {
		case "*", name:
			return true
		}
	}
	return false
}

func isEqualVirtualService(vs1 *istioV1beta1.VirtualService, vs2 *istioV1beta1.VirtualService) bool {
	return reflect.DeepEqual(vs1.Spec, vs2.Spec)
}

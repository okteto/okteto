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
	"fmt"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// Driver weaver struct for the divert driver
type Driver struct {
	client    kubernetes.Interface
	cache     *cache
	name      string
	namespace string
	divert    model.DivertDeploy
}

func New(m *model.Manifest, c kubernetes.Interface) *Driver {
	return &Driver{
		name:      m.Name,
		namespace: m.Namespace,
		divert:    *m.Deploy.Divert,
		client:    c,
	}
}

func (d *Driver) Deploy(ctx context.Context) error {
	oktetoLog.Spinner(fmt.Sprintf("Diverting namespace %s...", d.divert.Namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()
	if err := d.initCache(ctx); err != nil {
		return err
	}
	for name, in := range d.cache.divertIngresses {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			oktetoLog.Spinner(fmt.Sprintf("Diverting ingress %s/%s...", in.Namespace, in.Name))
			oktetoLog.StartSpinner()
			defer oktetoLog.StopSpinner()
			if err := d.divertIngress(ctx, name); err != nil {
				return err
			}
			oktetoLog.StopSpinner()
			oktetoLog.Success("Ingress '%s/%s' successfully diverted", in.Namespace, in.Name)
		}
	}
	return nil
}

// Destroy implements from the interface diver.Driver
// nolint:unparam
func (d *Driver) Destroy(_ context.Context) error {
	oktetoLog.Success("Divert from '%s' successfully destroyed", d.divert.Namespace)
	return nil
}

func (d *Driver) UpdatePod(pod apiv1.PodSpec) apiv1.PodSpec {
	if pod.DNSConfig == nil {
		pod.DNSConfig = &apiv1.PodDNSConfig{}
	}
	if pod.DNSConfig.Searches == nil {
		pod.DNSConfig.Searches = []string{}
	}
	searches := []string{fmt.Sprintf("%s.svc.cluster.local", d.divert.Namespace)}
	searches = append(searches, pod.DNSConfig.Searches...)
	pod.DNSConfig.Searches = searches
	return pod
}

func (d *Driver) UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService) {}

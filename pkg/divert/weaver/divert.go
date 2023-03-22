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
	"k8s.io/client-go/kubernetes"
)

// Driver weaver struct for the divert driver
type Driver struct {
	name      string
	namespace string
	divert    model.DivertDeploy
	client    kubernetes.Interface
	cache     *cache
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
			if err := d.divertIngress(ctx, name); err != nil {
				return err
			}
		}
	}
	return nil
}

func (*Driver) Destroy(_ context.Context) error {
	return nil
}

func (d *Driver) GetDivertNamespace() string {
	if d.divert.Namespace == d.namespace {
		return ""
	}
	return d.divert.Namespace
}

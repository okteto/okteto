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

	"github.com/okteto/okteto/pkg/k8s/diverts"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/textblock"
	"k8s.io/client-go/kubernetes"
)

const (
	divertTextBlockHeader = "# ---- START DIVERT ----"
	divertTextBlockFooter = "# ---- END DIVERT ----"
)

var (
	divertTextBlockParser = textblock.NewTextBlock(divertTextBlockHeader, divertTextBlockFooter)
)

// Driver weaver struct for the divert driver
type Driver struct {
	manifest     *model.Manifest
	client       kubernetes.Interface
	divertClient *diverts.DivertV1Client
	cache        *cache
}

func New(m *model.Manifest, c kubernetes.Interface, dc *diverts.DivertV1Client) *Driver {
	return &Driver{
		manifest:     m,
		client:       c,
		divertClient: dc,
	}
}

func (d *Driver) Deploy(ctx context.Context) error {
	if err := d.divertIngresses(ctx); err != nil {
		return err
	}
	return d.createDivertCRD(ctx)
}

func (d *Driver) divertIngresses(ctx context.Context) error {
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
	if d.manifest.Deploy.Divert.Namespace == d.manifest.Namespace {
		return ""
	}
	return d.manifest.Deploy.Divert.Namespace
}

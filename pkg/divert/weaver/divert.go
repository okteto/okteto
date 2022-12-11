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
	"fmt"

	divertInterface "github.com/okteto/okteto/pkg/divert"
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

// PortMapping represents the port mapping of a divert
type PortMapping struct {
	ProxyPort          int32 `json:"proxy_port,omitempty" yaml:"proxy_port,omitempty"`
	OriginalPort       int32 `json:"original_port,omitempty" yaml:"original_port,omitempty"`
	OriginalTargetPort int32 `json:"original_target_port,omitempty" yaml:"original_target_port,omitempty"`
}

type driver struct {
	c     kubernetes.Interface
	m     *model.Manifest
	cache *cache
}

func New(c kubernetes.Interface, m *model.Manifest) divertInterface.Driver {
	return &driver{
		c: c,
		m: m,
	}
}

func (d *driver) Deploy(ctx context.Context) error {
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
	return d.createDivertCRD(ctx)
}

func (d *driver) Destroy(ctx context.Context) error {
	return nil
}

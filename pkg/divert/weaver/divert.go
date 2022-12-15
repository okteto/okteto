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
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/k8s/diverts"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/textblock"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	divertTextBlockHeader = "# ---- START DIVERT ----"
	divertTextBlockFooter = "# ---- END DIVERT ----"
)

var (
	divertTextBlockParser = textblock.NewTextBlock(divertTextBlockHeader, divertTextBlockFooter)
)

type portMapping struct {
	ProxyPort          int32 `json:"proxy_port,omitempty" yaml:"proxy_port,omitempty"`
	OriginalPort       int32 `json:"original_port,omitempty" yaml:"original_port,omitempty"`
	OriginalTargetPort int32 `json:"original_target_port,omitempty" yaml:"original_target_port,omitempty"`
}

// Driver weaver struct for the divert driver
type Driver struct {
	Client       kubernetes.Interface
	DivertClient *diverts.DivertV1Client
	Manifest     *model.Manifest
	cache        *cache
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
	return d.createDivertCRD(ctx)
}

func (d *Driver) Destroy(ctx context.Context) error {
	return nil
}

func (d *Driver) ApplyToDeployment(d1 *appsv1.Deployment, d2 *appsv1.Deployment) {
	if d2.Spec.Template.Labels == nil {
		return
	}
	if d2.Spec.Template.Labels[model.OktetoDivertInjectSidecarLabel] == "" {
		return
	}
	if d1.Spec.Template.Labels == nil {
		d1.Spec.Template.Labels = map[string]string{}
	}
	d1.Spec.Template.Labels[model.OktetoDivertInjectSidecarLabel] = d2.Spec.Template.Labels[model.OktetoDivertInjectSidecarLabel]
}

func (d *Driver) ApplyToService(s1 *apiv1.Service, s2 *apiv1.Service) {
	if s2.Annotations[model.OktetoDivertServiceAnnotation] == "" {
		return
	}
	if s2.Annotations[model.OktetoAutoCreateAnnotation] == "true" {
		return
	}
	if s1.Annotations == nil {
		s1.Annotations = map[string]string{}
	}
	s1.Annotations[model.OktetoDivertServiceAnnotation] = s2.Annotations[model.OktetoDivertServiceAnnotation]
	divertMapping := portMapping{}
	if err := json.Unmarshal([]byte(s2.Annotations[model.OktetoDivertServiceAnnotation]), &divertMapping); err != nil {
		oktetoLog.Warning("skipping apply divert to service '%s': %s", s1.Name, err.Error())
		return
	}
	for i := range s1.Spec.Ports {
		if s1.Spec.Ports[i].Port == divertMapping.OriginalPort {
			s1.Spec.Ports[i].TargetPort = intstr.IntOrString{IntVal: divertMapping.ProxyPort}
		}
	}
}

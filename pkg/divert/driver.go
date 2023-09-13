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

package divert

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert/istio"
	"github.com/okteto/okteto/pkg/divert/weaver"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/virtualservices"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Driver interface {
	Deploy(ctx context.Context) error
	Destroy(ctx context.Context) error
	UpdatePod(spec apiv1.PodSpec) apiv1.PodSpec
	UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService)
}

func New(m *model.Manifest, c kubernetes.Interface) (Driver, error) {
	if !okteto.IsOkteto() {
		return nil, oktetoErrors.ErrDivertNotSupported
	}

	if m.Deploy.Divert.Driver == constants.OktetoDivertWeaverDriver {
		return weaver.New(m, c), nil
	}

	ic, err := virtualservices.GetIstioClient()
	if err != nil {
		return nil, fmt.Errorf("error creating istio client: %w", err)
	}

	return istio.New(m, c, ic), nil
}

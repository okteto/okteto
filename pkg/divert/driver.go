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
	"sync"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert/istio"
	"github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/divert/nginx"
	"github.com/okteto/okteto/pkg/divert/noop"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/virtualservices"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	isDivertCRDInstalled bool
	tOnce                sync.Once
)

type Driver interface {
	Deploy(ctx context.Context) error
	Destroy(ctx context.Context) error
	UpdatePod(spec apiv1.PodSpec) apiv1.PodSpec
	UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService)
}

func New(divert *model.DivertDeploy, name, namespace string, c kubernetes.Interface, ioCtrl *io.Controller) (Driver, error) {
	if !okteto.IsOkteto() {
		return nil, oktetoErrors.ErrDivertNotSupported
	}

	if divert.Driver == constants.OktetoDivertNginxDriver {
		// Check if the divert CRD is installed only once
		tOnce.Do(checkIfDivertCRDIsInstalled(ioCtrl))

		var err error
		divertClient := k8s.GetNoopDivertClient(ioCtrl)

		// If the divert CRD is not installed, we use the noop client to not fail when deploying the divert section
		if isDivertCRDInstalled {
			divertClient, err = k8s.GetDivertClient()
			if err != nil {
				return nil, fmt.Errorf("error creating divert client: %w", err)
			}
		}

		return nginx.New(divert, name, namespace, c, divertClient), nil
	}

	ic, err := virtualservices.GetIstioClient()
	if err != nil {
		return nil, fmt.Errorf("error creating istio client: %w", err)
	}

	return istio.New(divert, name, namespace, c, ic), nil
}

// NewNoop returns a new noop driver for divert. Useful for cases in which divert is not defined
// in the manifest but the flow needs a driver
func NewNoop() Driver {
	return &noop.Driver{}
}

func checkIfDivertCRDIsInstalled(ioCtrl *io.Controller) func() {
	return func() {
		apixClient, err := okteto.GetApiExtensionsClient()
		if err != nil {
			ioCtrl.Logger().Infof("error getting api extensions client: %v. Assuming CRDs are not installed", err)
			return
		}
		checker := k8s.CRDInstallationChecker{
			Client: apixClient,
			Logger: ioCtrl.Logger(),
		}

		isInstalled, err := checker.IsInstalled(context.Background())
		if err != nil {
			ioCtrl.Logger().Infof("error checking if Divert CRD is installed: %v. Assuming CRDs are not installed", err)
			return
		}

		isDivertCRDInstalled = isInstalled
	}
}

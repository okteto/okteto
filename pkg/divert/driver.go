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

	"github.com/okteto/okteto/pkg/divert/weaver"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Driver interface {
	Deploy(ctx context.Context) error
	Destroy(ctx context.Context) error
	ApplyToDeployment(d1 *appsv1.Deployment, d2 *appsv1.Deployment)
	ApplyToService(s1 *apiv1.Service, s2 *apiv1.Service)
}

func New(m *model.Manifest, dc *diverts.DivertV1Client, c kubernetes.Interface) Driver {
	return &weaver.Driver{
		Client:       c,
		DivertClient: dc,
		Manifest:     m,
	}
}

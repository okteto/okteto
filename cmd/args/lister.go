// Copyright 2024 The Okteto Authors
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

package args

import (
	"context"
	"fmt"
	"sort"

	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

type DevModeOnLister struct {
	k8sClientProvider okteto.K8sClientProvider
}

func NewDevModeOnLister(k8sClientProvider okteto.K8sClientProvider) *DevModeOnLister {
	return &DevModeOnLister{
		k8sClientProvider: k8sClientProvider,
	}
}

func (d *DevModeOnLister) List(ctx context.Context, devs model.ManifestDevs, ns string) ([]string, error) {
	k8sClient, _, err := d.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s client: %w", err)
	}

	devNameList := apps.ListDevModeOn(ctx, devs, ns, k8sClient)
	if len(devNameList) == 0 {
		return nil, errNoDevContainerInDevMode
	}
	sort.Strings(devNameList)
	return devNameList, nil
}

type ManifestDevLister struct {
}

func NewManifestDevLister() *ManifestDevLister {
	return &ManifestDevLister{}
}

func (m *ManifestDevLister) List(_ context.Context, devs model.ManifestDevs, _ string) ([]string, error) {
	devList := devs.GetDevs()
	if len(devList) == 0 {
		return nil, errNoDevContainerInManifest
	}
	sort.Strings(devList)
	return devList, nil
}

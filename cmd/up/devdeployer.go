// Copyright 2025 The Okteto Authors
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

package up

import (
	"context"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/client-go/kubernetes"
)

const (
	servicesUpWaitEnvVar    = "OKTETO_SERVICES_UP_WAIT"
	forceRedeployAnnotation = "okteto.dev/force-redeploy"
)

type devDeployer struct {
	translations   map[string]*apps.Translation
	k8sClient      kubernetes.Interface
	servicesUpWait bool

	mainTranslation *apps.Translation
	devTranslations []*apps.Translation
}

// newDevDeployer creates a new devDeployer
func newDevDeployer(translations map[string]*apps.Translation, k8sClient kubernetes.Interface) *devDeployer {
	var mainTranslation *apps.Translation
	devTranslations := make([]*apps.Translation, 0)
	servicesUpWait := env.LoadBoolean(servicesUpWaitEnvVar)

	for _, translation := range translations {
		if translation.MainDev == translation.Dev {
			mainTranslation = translation
		} else {
			devTranslations = append(devTranslations, translation)
		}
	}

	return &devDeployer{
		translations:    translations,
		k8sClient:       k8sClient,
		mainTranslation: mainTranslation,
		devTranslations: devTranslations,
		servicesUpWait:  servicesUpWait,
	}
}

// deployMainDev deploys the main dev
func (dd *devDeployer) deployMainDev(ctx context.Context) error {
	return dd.deploy(ctx, dd.mainTranslation)
}

// deployDevServices deploys the dev services
func (dd *devDeployer) deployDevServices(ctx context.Context) error {
	for _, tr := range dd.devTranslations {
		annotations := tr.DevApp.TemplateObjectMeta().Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[forceRedeployAnnotation] = uuid.New().String()

		if err := dd.deploy(ctx, tr); err != nil {
			return err
		}
	}
	return nil
}

// deploy replaces the devApp and the app
func (dd *devDeployer) deploy(ctx context.Context, tr *apps.Translation) error {
	delete(tr.DevApp.ObjectMeta().Annotations, model.DeploymentRevisionAnnotation)
	if err := tr.DevApp.Deploy(ctx, dd.k8sClient); err != nil {
		return err
	}
	if err := tr.App.Deploy(ctx, dd.k8sClient); err != nil {
		return err
	}
	return nil
}

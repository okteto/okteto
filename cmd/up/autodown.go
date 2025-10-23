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

package up

import (
	"context"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"k8s.io/client-go/kubernetes"
)

const (
	autoDownEnvVar = "OKTETO_AUTO_DOWN_ENABLED"
)

// autoDownRunner is the struct that runs the AutoDown logic if enabled
// based on the env var OKTETO_AUTO_DOWN_ENABLED, defaulting to false
// if enabled, it will run the AutoDown logic
type autoDownRunner struct {
	autoDown bool

	ioCtrl           *io.Controller
	k8sLogger        *io.K8sLogger
	analyticsTracker analyticsTrackerInterface
	downCmd          downCmdRunner
}

type downCmdRunner interface {
	Run(app apps.App, dev *model.Dev, namespace string, trMap map[string]*apps.Translation, wait bool) error
}

// newAutoDown creates a new AutoDown instance
func newAutoDown(ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at analyticsTrackerInterface, upMeta *analytics.UpMetricsMetadata) *autoDownRunner {
	enabled := env.LoadBooleanOrDefault(autoDownEnvVar, false)
	upMeta.IsAutoDownEnabled(enabled)

	downCmd := down.New(afero.NewOsFs(), okteto.NewK8sClientProviderWithLogger(k8sLogger), at)
	return &autoDownRunner{
		autoDown:         enabled,
		ioCtrl:           ioCtrl,
		k8sLogger:        k8sLogger,
		analyticsTracker: at,
		downCmd:          downCmd,
	}
}

// run is the main function that runs the AutoDown logic if enabled
func (a *autoDownRunner) run(ctx context.Context, dev *model.Dev, namespace string, k8sClient kubernetes.Interface) error {
	if !a.autoDown {
		a.ioCtrl.Logger().Infof("AutoDown is disabled, skipping AutoDown logic")
		return nil
	}
	a.ioCtrl.Logger().Infof("AutoDown is enabled, running AutoDown logic")
	sp := a.ioCtrl.Out().Spinner("Running okteto down...")
	sp.Start()
	defer sp.Stop()
	app, _, err := utils.GetApp(ctx, dev, namespace, k8sClient, false)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return err
		}
		app = apps.NewDeploymentApp(deployments.Sandbox(dev, namespace))
	}
	if dev.Autocreate {
		app = apps.NewDeploymentApp(deployments.Sandbox(dev, namespace))
	}

	trMap, err := apps.GetTranslations(ctx, namespace, dev, app, false, k8sClient)
	if err != nil {
		return err
	}

	err = a.downCmd.Run(app, dev, namespace, trMap, true)
	if err != nil {
		return err
	}
	a.ioCtrl.Out().Success("okteto down completed")
	return nil
}

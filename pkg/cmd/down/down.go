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

package down

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/afero"
	"k8s.io/client-go/kubernetes"
)

type analyticsTrackerInterface interface {
	TrackDown(bool)
	TrackDownVolumes(bool)
}

type Operation struct {
	Fs                afero.Fs
	K8sClientProvider okteto.K8sClientProvider
	AnalyticsTracker  analyticsTrackerInterface
}

func New(fs afero.Fs, k8sClientProvider okteto.K8sClientProvider, at analyticsTrackerInterface) *Operation {
	return &Operation{
		Fs:                fs,
		K8sClientProvider: k8sClientProvider,
		AnalyticsTracker:  at,
	}
}

func (d *Operation) Down(ctx context.Context, dev *model.Dev, rm bool) error {
	oktetoLog.Spinner(fmt.Sprintf("Deactivating '%s' development container...", dev.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	k8sClient, _, err := d.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		app, _, err := utils.GetApp(ctx, dev, k8sClient, false)
		if err != nil {
			if !oktetoErrors.IsNotFound(err) {
				exit <- err
				return
			}
			app = apps.NewDeploymentApp(deployments.Sandbox(dev))
		}
		if dev.Autocreate {
			app = apps.NewDeploymentApp(deployments.Sandbox(dev))
		}

		trMap, err := apps.GetTranslations(ctx, dev, app, false, k8sClient)
		if err != nil {
			exit <- err
			return
		}

		if err := d.Run(app, dev, trMap, true); err != nil {
			exit <- err
			return
		}

		oktetoLog.Success(fmt.Sprintf("Development container '%s' deactivated", dev.Name))

		if !rm {
			exit <- nil
			return
		}

		oktetoLog.Spinner(fmt.Sprintf("Removing '%s' persistent volume...", dev.Name))
		if err := removeVolume(ctx, dev, k8sClient); err != nil {
			d.AnalyticsTracker.TrackDownVolumes(false)
			exit <- err
			return
		}
		oktetoLog.Success(fmt.Sprintf("Persistent volume '%s' removed", dev.Name))

		if os.Getenv(model.OktetoSkipCleanupEnvVar) == "" {
			if err := syncthing.RemoveFolder(dev, d.Fs); err != nil {
				oktetoLog.Infof("failed to delete existing syncthing folder")
			}
		}

		d.AnalyticsTracker.TrackDownVolumes(true)
		exit <- nil
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		oktetoLog.StopSpinner()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func removeVolume(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	return volumes.Destroy(ctx, dev.GetVolumeName(), dev.Namespace, c, dev.Timeout.Default)
}

func (d *Operation) AllDown(ctx context.Context, manifest *model.Manifest, rm bool) error {
	oktetoLog.Spinner("Deactivating your development containers...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	k8sClient, _, err := d.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	if len(manifest.Dev) == 0 {
		return fmt.Errorf("okteto manifest has no 'dev' section. Configure it with 'okteto init'")
	}

	for _, dev := range manifest.Dev {
		app, _, err := utils.GetApp(ctx, dev, k8sClient, false)
		if err != nil {
			return err
		}

		if apps.IsDevModeOn(app) {
			oktetoLog.StopSpinner()
			if err := d.Down(ctx, dev, rm); err != nil {
				d.AnalyticsTracker.TrackDown(false)
				return fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
			}
		}
	}

	d.AnalyticsTracker.TrackDown(true)

	return nil
}

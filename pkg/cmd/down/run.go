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

package down

import (
	"context"

	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
)

// Run runs the "okteto down" sequence
func (d *Operation) Run(app apps.App, dev *model.Dev, trMap map[string]*apps.Translation, wait bool) error {
	ctx := context.Background()
	if len(trMap) == 0 {
		oktetoLog.Info("no translations available in the deployment")
	}

	k8sClient, _, err := d.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}

	for _, tr := range trMap {
		if app.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] == model.OktetoUpCmd {
			if err := app.Destroy(ctx, k8sClient); err != nil {
				return err
			}

			if err := services.DestroyDev(ctx, dev, k8sClient); err != nil {
				return err
			}
			if tr.Dev != dev {
				if err := tr.DevModeOff(); err != nil {
					oktetoLog.Infof("failed to turn devmode off: %s", err)
				}
				if err := tr.App.Deploy(ctx, k8sClient); err != nil {
					return err
				}
			}

		} else {
			if err := tr.DevModeOff(); err != nil {
				oktetoLog.Infof("failed to turn devmode off: %s", err)
			}
			if err := tr.App.Deploy(ctx, k8sClient); err != nil {
				return err
			}
		}

		tr.DevApp = tr.App.DevClone()
		if err := tr.DevApp.Destroy(ctx, k8sClient); err != nil {
			return err
		}
	}

	if err := secrets.Destroy(ctx, dev, k8sClient); err != nil {
		return err
	}

	d.stopSyncthing(dev)

	if err := ssh.RemoveEntry(dev.Name); err != nil {
		oktetoLog.Infof("failed to remove ssh entry: %s", err)
	}

	if !wait {
		return nil
	}

	devPodTerminationRetries := 30
	waitForDevPodsTermination(ctx, k8sClient, dev, devPodTerminationRetries)
	return nil
}

func (d *Operation) stopSyncthing(dev *model.Dev) {
	sy, err := syncthing.New(dev, d.Fs)
	if err != nil {
		oktetoLog.Infof("failed to create syncthing instance")
		return
	}

	if err := sy.HardTerminate(); err != nil {
		oktetoLog.Infof("failed to hard terminate existing syncthing")
	}
}

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
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"k8s.io/client-go/kubernetes"
)

// Run runs the "okteto down" sequence
func Run(dev *model.Dev, app apps.App, trMap map[string]*apps.Translation, wait bool, c kubernetes.Interface) error {
	ctx := context.Background()
	if len(trMap) == 0 {
		oktetoLog.Info("no translations available in the deployment")
	}

	for _, tr := range trMap {
		if app.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] == model.OktetoUpCmd {
			if err := app.Destroy(ctx, c); err != nil {
				return err
			}

			if err := services.DestroyDev(ctx, dev, c); err != nil {
				return err
			}
			if tr.Dev != dev {
				if err := tr.DevModeOff(); err != nil {
					oktetoLog.Infof("failed to turn devmode off: %s", err)
				}
				if err := tr.App.Deploy(ctx, c); err != nil {
					return err
				}
			}

		} else {
			if err := tr.DevModeOff(); err != nil {
				oktetoLog.Infof("failed to turn devmode off: %s", err)
			}
			if err := tr.App.Deploy(ctx, c); err != nil {
				return err
			}
		}

		tr.DevApp = tr.App.DevClone()
		if err := tr.DevApp.Destroy(ctx, c); err != nil {
			return err
		}
	}

	if err := secrets.Destroy(ctx, dev, c); err != nil {
		return err
	}

	stopSyncthing(dev)

	if err := ssh.RemoveEntry(dev.Name); err != nil {
		oktetoLog.Infof("failed to remove ssh entry: %s", err)
	}

	if !wait {
		return nil
	}

	devPodTerminationRetries := 30
	waitForDevPodsTermination(ctx, c, dev, devPodTerminationRetries)
	return nil
}

func stopSyncthing(dev *model.Dev) {
	sy, err := syncthing.New(dev)
	if err != nil {
		oktetoLog.Infof("failed to create syncthing instance")
		return
	}

	if err := sy.HardTerminate(); err != nil {
		oktetoLog.Infof("failed to hard terminate existing syncthing")
	}
}

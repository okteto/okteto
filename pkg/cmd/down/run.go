// Copyright 2020 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
)

//Run runs the "okteto down" sequence
func Run(dev *model.Dev, d *appsv1.Deployment, trList map[string]*model.Translation, wait bool, c *kubernetes.Clientset) error {
	if len(trList) == 0 {
		log.Info("no translations available in the deployment")
	}

	for _, tr := range trList {
		if tr.Deployment == nil {
			continue
		}
		dTmp, err := deployments.TranslateDevModeOff(tr.Deployment)
		if err != nil {
			return err
		}
		tr.Deployment = dTmp
	}
	if err := deployments.UpdateDeployments(trList, c); err != nil {
		return err
	}

	if err := secrets.Destroy(dev, c); err != nil {
		return err
	}

	stopSyncthing(dev)

	if err := ssh.RemoveEntry(dev.Name); err != nil {
		log.Infof("failed to remove ssh entry: %s", err)
	}

	if d == nil {
		return nil
	}

	if _, ok := d.Annotations[model.OktetoAutoCreateAnnotation]; ok {
		if err := deployments.Destroy(dev, c); err != nil {
			return err
		}
		if err := services.DestroyDev(dev, c); err != nil {
			return err
		}
	}

	if !wait {
		return nil
	}

	waitForDevPodsTermination(c, dev, 30)
	return nil
}

func stopSyncthing(dev *model.Dev) {
	sy, err := syncthing.New(dev)
	if err != nil {
		log.Infof("failed to create syncthing instance")
		return
	}

	if err := sy.Stop(true); err != nil {
		log.Infof("failed to stop existing syncthing")
	}
}

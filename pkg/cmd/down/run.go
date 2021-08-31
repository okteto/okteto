// Copyright 2021 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"k8s.io/client-go/kubernetes"
)

// Run runs the "okteto down" sequence
func Run(dev *model.Dev, k8sObject *model.K8sObject, trList map[string]*model.Translation, wait bool, c kubernetes.Interface) error {
	ctx := context.Background()
	if len(trList) == 0 {
		log.Info("no translations available in the deployment")
	}

	for _, tr := range trList {
		if tr.K8sObject == nil {
			continue
		}
		dTmp, err := apps.TranslateDevModeOff(tr.K8sObject)
		if err != nil {
			return err
		}
		tr.K8sObject = dTmp
	}
	if err := apps.UpdateK8sObjects(ctx, trList, c); err != nil {
		return err
	}

	if err := secrets.Destroy(ctx, dev, c); err != nil {
		return err
	}

	stopSyncthing(dev)

	if err := ssh.RemoveEntry(dev.Name); err != nil {
		log.Infof("failed to remove ssh entry: %s", err)
	}

	if k8sObject.Deployment == nil && k8sObject.StatefulSet == nil {
		return nil
	}

	if k8sObject.GetAnnotation(model.OktetoAutoCreateAnnotation) == model.OktetoUpCmd {
		if err := apps.DestroyDev(ctx, k8sObject, dev, c); err != nil {
			return err
		}

		if err := services.DestroyDev(ctx, dev, c); err != nil {
			return err
		}
	}

	if !wait {
		return nil
	}

	waitForDevPodsTermination(ctx, c, dev, 30)
	return nil
}

func stopSyncthing(dev *model.Dev) {
	sy, err := syncthing.New(dev)
	if err != nil {
		log.Infof("failed to create syncthing instance")
		return
	}

	if err := sy.HardTerminate(); err != nil {
		log.Infof("failed to hard terminate existing syncthing")
	}
}

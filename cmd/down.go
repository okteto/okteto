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

package cmd

import (
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Down deactivates the development environment
func Down() *cobra.Command {
	var devPath string
	var namespace string
	var rm bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivates your development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting down command")

			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}

			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}

			if err := runDown(dev); err != nil {
				analytics.TrackDown(false)
				return err
			}

			log.Success("Development environment deactivated")

			if rm {
				if err := removeVolume(dev); err != nil {
					analytics.TrackDownVolumes(false)
					return err
				}
				log.Success("Persistent volume removed")
				analytics.TrackDownVolumes(true)
			}

			log.Println()

			analytics.TrackDown(true)
			log.Info("completed down command")
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&rm, "volumes", "v", false, "remove persistent volume")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the down command is executed")
	return cmd
}

func runDown(dev *model.Dev) error {
	spinner := newSpinner("Deactivating your development environment...")
	spinner.start()
	defer spinner.stop()

	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	d, err := deployments.Get(dev, dev.Namespace, client)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	tr, err := deployments.GetTranslations(dev, d, client)
	if err != nil {
		return err
	}

	if len(tr) == 0 {
		log.Info("no translations available in the deployment")
	}

	for _, t := range tr {
		if t.Deployment == nil {
			continue
		}
		if err := deployments.DevModeOff(t.Deployment, client); err != nil {
			return err
		}
	}

	if err := secrets.Destroy(dev, client); err != nil {
		return err
	}

	//remove deprecated okteto volume
	if err := client.CoreV1().PersistentVolumeClaims(dev.Namespace).Delete(
		model.DeprecatedOktetoVolumeName,
		&metav1.DeleteOptions{}); err != nil && !strings.Contains(err.Error(), "not found") {
		log.Debugf("failed to remove deprecated okteto persistent volume: %s", err)
	}

	stopSyncthing(dev)

	if err := ssh.RemoveEntry(dev.Name); err != nil {
		log.Infof("failed to remove ssh entry: %s", err)
	}

	if d == nil {
		return nil
	}

	if _, ok := d.Annotations[model.OktetoAutoCreateAnnotation]; ok {
		if err := deployments.Destroy(dev, client); err != nil {
			return err
		}
		if err := services.DestroyDev(dev, client); err != nil {
			return err
		}
	}

	down.WaitForDevPodsTermination(client, dev, 30)
	return nil
}

func removeVolume(dev *model.Dev) error {
	spinner := newSpinner("Removing persistent volume...")
	spinner.start()
	defer spinner.stop()

	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}
	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	return volumes.Destroy(dev, client)
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

	if err := sy.RemoveFolder(); err != nil {
		log.Infof("failed to delete existing syncthing folder")
	}
}

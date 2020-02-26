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
	"fmt"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/down"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

//Redeploy builds, pushes and redeploys the target deployment
func Redeploy() *cobra.Command {
	var devPath string
	var namespace string
	var imageTag string

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Builds, pushes and redeploys source code to the target deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting push command")
			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}

			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}
			c, _, configNamespace, err := k8Client.GetLocal()
			if err != nil {
				return err
			}
			if dev.Namespace == "" {
				dev.Namespace = configNamespace
			}
			oktetoRegistryURL := ""
			n, err := namespaces.Get(dev.Namespace, c)
			if err == nil {
				if namespaces.IsOktetoNamespace(n) {
					oktetoRegistryURL, err = okteto.GetRegistry()
					if err != nil {
						return err
					}
				}
			}

			if err := runPush(dev, imageTag, oktetoRegistryURL, c); err != nil {
				analytics.TrackPush(false, oktetoRegistryURL)
				return err
			}

			log.Success("Source code pushed to the development environment '%s'", dev.Name)
			log.Println()

			analytics.TrackPush(true, oktetoRegistryURL)
			log.Info("completed push command")
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the push command is executed")
	cmd.Flags().StringVarP(&imageTag, "tag", "t", "", "image tag to build, push and redeploy")
	return cmd
}

func runPush(dev *model.Dev, imageTag, oktetoRegistryURL string, c *kubernetes.Clientset) error {
	d, err := deployments.Get(dev, dev.Namespace, c)
	if err != nil {
		return err
	}

	buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
	if err != nil {
		return err
	}

	imageTag = build.GetImageTag(dev, imageTag, d, oktetoRegistryURL)
	log.Infof("pushing with image tag %s", imageTag)

	var imageDigest string
	imageDigest, err = build.Run(buildKitHost, isOktetoCluster, ".", "Dockerfile", imageTag, "", false, nil)
	if err != nil {
		return fmt.Errorf("error building image '%s': %s", imageTag, err)
	}
	if imageDigest != "" {
		imageWithoutTag := build.GetRepoNameWithoutTag(imageTag)
		imageTag = fmt.Sprintf("%s@%s", imageWithoutTag, imageDigest)
	}

	spinner := newSpinner(fmt.Sprintf("Pushing source code to the development environment '%s'...", dev.Name))
	spinner.start()
	defer spinner.stop()
	err = down.Run(dev, imageTag, d, c)
	if err != nil {
		return err
	}

	return nil
}

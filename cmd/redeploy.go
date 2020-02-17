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
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

//Redeploy builds, pushes and redeploys the target deployment
func Redeploy() *cobra.Command {
	var devPath string
	var namespace string
	var imageTag string

	cmd := &cobra.Command{
		Use:   "redeploy",
		Short: "Builds, pushes and redeploys the target deployment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting redeploy command")

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
			isOktetoNamespace := false
			n, err := namespaces.Get(dev.Namespace, c)
			if err == nil {
				if namespaces.IsOktetoNamespace(n) {
					isOktetoNamespace = true
				}
			}

			if err := runRedeploy(dev, imageTag, isOktetoNamespace, c); err != nil {
				analytics.TrackRedeploy(false, isOktetoNamespace)
				return err
			}

			log.Success("Development environment '%s' redeployed", dev.Name)
			log.Println()

			analytics.TrackRedeploy(true, isOktetoNamespace)
			log.Info("completed redeploy command")
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the redeploy command is executed")
	cmd.Flags().StringVarP(&imageTag, "image", "i", "", "image to build and push")
	return cmd
}

func runRedeploy(dev *model.Dev, imageTag string, isOktetoNamespace bool, c *kubernetes.Clientset) error {
	d, err := deployments.Get(dev, dev.Namespace, c)
	if err != nil {
		return err
	}
	imageTag = build.GetImageTag(dev, imageTag, d, isOktetoNamespace)
	var imageDigest string
	imageDigest, err = RunBuild(".", "Dockerfile", imageTag, "", false)
	if err != nil {
		return fmt.Errorf("error building image '%s': %s", imageTag, err)
	}
	if imageDigest != "" {
		imageWithoutTag := build.GetRepoNameWithoutTag(imageTag)
		imageTag = fmt.Sprintf("%s@%s", imageWithoutTag, imageDigest)
	}

	spinner := newSpinner(fmt.Sprintf("Redeploying development environment '%s'...", dev.Name))
	spinner.start()
	defer spinner.stop()
	err = down.Run(dev, imageTag, d, c)
	if err != nil {
		return err
	}

	return nil
}

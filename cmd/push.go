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

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// Push builds, pushes and redeploys the target app
func Push(ctx context.Context) *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var imageTag string
	var autoDeploy bool
	var progress string
	var appName string
	var noCache bool

	cmd := &cobra.Command{
		Use:   "push",
		Short: "Builds, pushes and redeploys source code to the target app",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#push"),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			if err := utils.LoadEnvironment(ctx, true); err != nil {
				return err
			}

			dev, err := utils.LoadDevOrDefault(devPath, appName, namespace, k8sContext)
			if err != nil {
				return err
			}

			if len(appName) > 0 && appName != dev.Name {
				return fmt.Errorf("app name provided does not match the name field in your okteto manifest")
			}

			c, _, err := k8Client.GetLocalWithContext(dev.Context)
			if err != nil {
				return err
			}

			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			oktetoRegistryURL := ""
			n, err := namespaces.Get(ctx, dev.Namespace, c)
			if err == nil {
				if namespaces.IsOktetoNamespace(n) {
					oktetoRegistryURL, err = okteto.GetRegistry()
					if err != nil {
						return err
					}
				}
			}

			if autoDeploy {
				log.Warning(`The 'deploy' flag is deprecated and will be removed in a future release.
    Set the 'autocreate' field in your okteto manifest to get the same behavior.
    More information is available here: https://okteto.com/docs/reference/cli#up`)
			}

			if !dev.Autocreate {
				dev.Autocreate = autoDeploy
			}

			if err := runPush(ctx, dev, autoDeploy, imageTag, oktetoRegistryURL, progress, noCache, c); err != nil {
				analytics.TrackPush(false, oktetoRegistryURL)
				return err
			}

			log.Success("Source code pushed to '%s'", dev.Name)
			log.Println()

			analytics.TrackPush(true, oktetoRegistryURL)
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the push command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the push command is executed")
	cmd.Flags().StringVarP(&imageTag, "tag", "t", "", "image tag to build, push and redeploy")
	cmd.Flags().BoolVarP(&autoDeploy, "deploy", "d", false, "create deployment when the app doesn't exist in a namespace")
	cmd.Flags().StringVarP(&progress, "progress", "", "tty", "show plain/tty build output")
	cmd.Flags().StringVar(&appName, "name", "", "name of the app to push to")
	cmd.Flags().BoolVarP(&noCache, "no-cache", "", false, "do not use cache when building the image")
	return cmd
}

func runPush(ctx context.Context, dev *model.Dev, autoDeploy bool, imageTag, oktetoRegistryURL, progress string, noCache bool, c *kubernetes.Clientset) error {
	exists := true
	app, err := apps.Get(ctx, dev, dev.Namespace, c)

	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		if !dev.Autocreate {
			return errors.UserError{
				E: fmt.Errorf("Application '%s' not found in namespace '%s'", dev.Name, dev.Namespace),
				Hint: `Verify that your application has been deployed and your Kubernetes context is pointing to the right namespace
    Or set the 'autocreate' field in your okteto manifest if you want to create a standalone deployment
    More information is available here: https://okteto.com/docs/reference/cli#up`,
			}
		}

		if len(dev.Services) > 0 {
			return fmt.Errorf("'autocreate' cannot be used in combination with 'services'")
		}

		app = apps.NewDeploymentApp(apps.GetDeploymentSandbox(dev))

		app.Annotations()[model.OktetoAutoCreateAnnotation] = model.OktetoPushCmd
		exists = false

		if imageTag == "" {
			if oktetoRegistryURL == "" {
				return fmt.Errorf("you need to specify the image tag to build with the '-t' argument")
			}
			imageTag = registry.GetImageTag("", dev.Name, dev.Namespace, oktetoRegistryURL)
		}
	}

	trList, err := apps.GetTranslations(ctx, dev, app, false, c)
	if err != nil {
		return err
	}

	for _, tr := range trList {
		if len(dev.Services) == 0 {
			if tr.App.Annotations()[model.OktetoAutoCreateAnnotation] == model.OktetoUpCmd || tr.App.PodSpec().Containers[0].Name == "dev" {
				tr.App.Annotations()[model.OktetoAutoCreateAnnotation] = model.OktetoPushCmd
			}
		}
		if tr.App.Replicas() == 0 {
			tr.App.DevModeOff(tr)
		}

		if tr.App.Annotations()[model.OktetoAutoCreateAnnotation] == model.OktetoPushCmd {
			for k, v := range tr.Annotations {
				tr.App.Annotations()[k] = v
			}
		}
	}

	if app != nil && app.IsDevModeOn() {
		if err := down.Run(dev, app, trList, false, c); err != nil {
			return err
		}

		log.Information("Development container deactivated")
	}

	imageFromApp, err := getImageFromApp(trList)
	if err != nil {
		return err
	}

	imageTag, err = buildImage(ctx, dev, imageTag, imageFromApp, oktetoRegistryURL, noCache, progress)
	if err != nil {
		return err
	}

	spinner := utils.NewSpinner(fmt.Sprintf("Pushing source code to '%s'...", dev.Name))
	spinner.Start()
	defer spinner.Stop()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		if app.Annotations()[model.OktetoAutoCreateAnnotation] == model.OktetoPushCmd {
			if err := services.CreateDev(ctx, dev, c); err != nil {
				exit <- err
				return
			}
		}

		if !exists {
			app.PodSpec().Containers[0].Image = imageTag
			app.SetLastBuiltAnnotation()
			exit <- app.Create(ctx, c)
			return
		}

		for _, tr := range trList {
			if tr.App == nil {
				continue
			}
			for _, rule := range tr.Rules {
				devContainer := apps.GetDevContainer(tr.App.PodSpec(), rule.Container)
				if devContainer == nil {
					exit <- fmt.Errorf("%s '%s': container '%s' not found", app.Kind(), app.Name(), rule.Container)
					return
				}
				app.SetLastBuiltAnnotation()
				devContainer.Image = imageTag
			}

			if err := tr.App.Update(ctx, c); err != nil {
				exit <- err
				return
			}
			exit <- nil
			return
		}
	}()
	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		spinner.Stop()
		os.Exit(130)
	case err := <-exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil

}

func buildImage(ctx context.Context, dev *model.Dev, imageTag, imageFromApp, oktetoRegistryURL string, noCache bool, progress string) (string, error) {
	buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
	if err != nil {
		return "", err
	}
	log.Information("Running your build in %s...", buildKitHost)

	if imageTag == "" {
		imageTag = dev.Push.Name
	}
	buildTag := registry.GetDevImageTag(dev, imageTag, imageFromApp, oktetoRegistryURL)
	log.Infof("pushing with image tag %s", buildTag)

	buildArgs := model.SerializeBuildArgs(dev.Push.Args)
	if err := build.Run(ctx, dev.Namespace, buildKitHost, isOktetoCluster, dev.Push.Context, dev.Push.Dockerfile, buildTag, dev.Push.Target, noCache, dev.Push.CacheFrom, buildArgs, nil, progress); err != nil {
		return "", err
	}

	return buildTag, nil
}

func getImageFromApp(trList map[string]*apps.Translation) (string, error) {
	imageFromApp := ""
	for _, tr := range trList {
		if tr.App == nil {
			continue
		}
		if tr.App.Annotations()[model.OktetoAutoCreateAnnotation] != "" && len(trList) > 1 {
			continue
		}
		for _, rule := range tr.Rules {
			devContainer := apps.GetDevContainer(tr.App.PodSpec(), rule.Container)
			if devContainer == nil {
				return "", fmt.Errorf("%s '%s': container '%s' not found", tr.App.Kind(), tr.App.Name(), rule.Container)
			}
			if imageFromApp == "" {
				imageFromApp = devContainer.Image
			}
			if devContainer.Image != imageFromApp {
				return "", fmt.Errorf("cannot push code: application referenced by okteto manifest use different images")
			}
		}
	}
	return imageFromApp, nil
}

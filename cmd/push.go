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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/down"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/services"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// pushOptions refers to all the options that can be passed to Push command
type pushOptions struct {
	DevPath    string
	Namespace  string
	K8sContext string
	ImageTag   string
	AutoDeploy bool
	Progress   string
	AppName    string
	NoCache    bool
}

// Push builds, pushes and redeploys the target app
func Push(ctx context.Context) *cobra.Command {
	pushOpts := &pushOptions{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "push [svc]",
		Short:  "Build, push and redeploy source code to the target app",
		Args:   utils.MaximumNArgsAccepted(1, "https://www.okteto.com/docs/0.10/reference/cli/#push"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !utils.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) {
				oktetoLog.Warning("'okteto push' is deprecated in favor of 'okteto deploy', and will be removed in a future version")
			}
			ctxResource, err := utils.LoadManifestContext(pushOpts.DevPath)
			if err != nil {
				if oktetoErrors.IsNotExist(err) && len(pushOpts.AppName) > 0 {
					ctxResource = &model.ContextResource{}
				} else {
					return err
				}
			}

			if err := ctxResource.UpdateNamespace(pushOpts.Namespace); err != nil {
				return err
			}

			if err := ctxResource.UpdateContext(pushOpts.K8sContext); err != nil {
				return err
			}

			ctxOptions := &contextCMD.ContextOptions{
				Context:   ctxResource.Context,
				Namespace: ctxResource.Namespace,
				Show:      true,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			manifest, err := utils.DeprecatedLoadManifestOrDefault(pushOpts.DevPath, pushOpts.AppName)
			if err != nil {
				return err
			}

			devName := ""
			if len(args) == 1 {
				devName = args[0]
			}
			dev, err := utils.GetDevFromManifest(manifest, devName)
			if err != nil {
				if !errors.Is(err, utils.ErrNoDevSelected) {
					return err
				}
				selector := utils.NewOktetoSelector("Select which development container to push to:", "Development container")
				dev, err = utils.SelectDevFromManifest(manifest, selector, manifest.Dev.GetDevs())
				if err != nil {
					return err
				}
			}

			if len(pushOpts.AppName) > 0 && pushOpts.AppName != dev.Name {
				return fmt.Errorf("app name provided does not match the name field in your okteto manifest")
			}

			c, _, err := okteto.GetK8sClient()
			if err != nil {
				return err
			}

			if pushOpts.AutoDeploy {
				oktetoLog.Warning(`The 'deploy' flag is deprecated and will be removed in a future version.
    Set the 'autocreate' field in your okteto manifest to get the same behavior.
    More information is available here: https://okteto.com/docs/reference/cli#up`)
			}

			if !dev.Autocreate {
				dev.Autocreate = pushOpts.AutoDeploy
			}

			if err := runPush(ctx, dev, pushOpts, c); err != nil {
				analytics.TrackPush(false)
				return err
			}

			oktetoLog.Success("Source code pushed to '%s'", dev.Name)
			oktetoLog.Println()

			analytics.TrackPush(true)
			return nil
		},
	}

	cmd.Flags().StringVarP(&pushOpts.DevPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&pushOpts.Namespace, "namespace", "n", "", "namespace where the push command is executed")
	cmd.Flags().StringVarP(&pushOpts.K8sContext, "context", "c", "", "context where the push command is executed")
	cmd.Flags().StringVarP(&pushOpts.ImageTag, "tag", "t", "", "image tag to build, push and redeploy")
	cmd.Flags().BoolVarP(&pushOpts.AutoDeploy, "deploy", "d", false, "create deployment when the app doesn't exist in a namespace")
	cmd.Flags().StringVarP(&pushOpts.Progress, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output")
	cmd.Flags().StringVar(&pushOpts.AppName, "name", "", "name of the app to push to")
	cmd.Flags().BoolVarP(&pushOpts.NoCache, "no-cache", "", false, "do not use cache when building the image")
	return cmd
}

func runPush(ctx context.Context, dev *model.Dev, pushOpts *pushOptions, c *kubernetes.Clientset) error {
	exists := true
	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	reg := registry.NewOktetoRegistry(okteto.Config{})
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return err
		}

		if !dev.Autocreate {
			return oktetoErrors.UserError{
				E: fmt.Errorf("application '%s' not found in namespace '%s'", dev.Name, dev.Namespace),
				Hint: `Verify that your application is running and your okteto context is pointing to the right namespace
    Or set the 'autocreate' field in your okteto manifest if you want to create a standalone development container
    More information is available here: https://okteto.com/docs/reference/cli#up`,
			}
		}

		if len(dev.Services) > 0 {
			return fmt.Errorf("'autocreate' cannot be used in combination with 'services'")
		}

		app = apps.NewDeploymentApp(deployments.Sandbox(dev))

		app.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] = model.OktetoPushCmd
		exists = false

		if pushOpts.ImageTag == "" {
			registryURL := okteto.Config{}.GetRegistryURL()
			if registryURL == "" {
				return fmt.Errorf("you need to specify the image tag to build with the '-t' argument")
			}
			pushOpts.ImageTag = reg.GetImageTag("", dev.Name, dev.Namespace)
		}
	}

	trMap, err := apps.GetTranslations(ctx, dev, app, false, c)
	if err != nil {
		return err
	}

	imageFromApp, err := getImageFromApp(trMap)
	if err != nil {
		return err
	}

	pushOpts.ImageTag, err = buildImage(ctx, dev, imageFromApp, pushOpts)
	if err != nil {
		return err
	}

	oktetoLog.Spinner(fmt.Sprintf("Pushing source code to '%s'...", dev.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	for _, tr := range trMap {
		if len(dev.Services) == 0 {
			if tr.App.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] == model.OktetoUpCmd || tr.App.PodSpec().Containers[0].Name == "dev" {
				tr.App.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] = model.OktetoPushCmd
			}
		}
		if apps.IsDevModeOn(tr.App) {
			if err := down.Run(dev, app, trMap, false, c); err != nil {
				return err
			}
			oktetoLog.Information("Development container deactivated")
		}
	}

	go func() {
		if app.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] == model.OktetoPushCmd {
			if err := services.CreateDev(ctx, dev, c); err != nil {
				exit <- err
				return
			}
		}

		if !exists {
			app.PodSpec().Containers[0].Image = pushOpts.ImageTag
			apps.SetLastBuiltAnnotation(app)
			exit <- app.Deploy(ctx, c)
			return
		}

		for _, tr := range trMap {
			if tr.App == nil {
				continue
			}
			for _, rule := range tr.Rules {
				devContainer := apps.GetDevContainer(tr.App.PodSpec(), rule.Container)
				if devContainer == nil {
					exit <- fmt.Errorf("%s '%s': container '%s' not found", app.Kind(), app.ObjectMeta().Name, rule.Container)
					return
				}
				apps.SetLastBuiltAnnotation(app)
				devContainer.Image = pushOpts.ImageTag
			}

			if err := tr.App.Deploy(ctx, c); err != nil {
				exit <- err
				return
			}
			exit <- nil
			return
		}
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

func buildImage(ctx context.Context, dev *model.Dev, imageFromApp string, pushOpts *pushOptions) (string, error) {
	oktetoLog.Information("Running your build in %s...", okteto.Context().Builder)

	reg := registry.NewOktetoRegistry(okteto.Config{})
	if pushOpts.ImageTag == "" {
		pushOpts.ImageTag = dev.Push.Name
	}
	buildTag := pushOpts.ImageTag
	if pushOpts.ImageTag == "" || pushOpts.ImageTag == model.DefaultImage {
		buildTag = reg.GetImageTag(imageFromApp, dev.Name, dev.Namespace)
	}
	oktetoLog.Infof("pushing with image tag %s", buildTag)

	buildArgs := model.SerializeBuildArgs(dev.Push.Args)
	buildOptions := &types.BuildOptions{
		Path:       dev.Push.Context,
		File:       dev.Push.Dockerfile,
		Tag:        buildTag,
		Target:     dev.Push.Target,
		NoCache:    pushOpts.NoCache,
		CacheFrom:  dev.Push.CacheFrom,
		BuildArgs:  buildArgs,
		OutputMode: pushOpts.Progress,
	}
	if err := buildv1.NewBuilderFromScratch().Build(ctx, buildOptions); err != nil {
		return "", err
	}

	return buildTag, nil
}

func getImageFromApp(trMap map[string]*apps.Translation) (string, error) {
	imageFromApp := ""
	for _, tr := range trMap {
		if tr.App == nil {
			continue
		}
		if tr.App.ObjectMeta().Annotations[model.OktetoAutoCreateAnnotation] != "" && len(trMap) > 1 {
			continue
		}
		for _, rule := range tr.Rules {
			devContainer := apps.GetDevContainer(tr.App.PodSpec(), rule.Container)
			if devContainer == nil {
				return "", fmt.Errorf("%s '%s': container '%s' not found", tr.App.Kind(), tr.App.ObjectMeta().Name, rule.Container)
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

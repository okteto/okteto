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

package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	initCMD "github.com/okteto/okteto/pkg/cmd/init"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/linguist"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	stignoreFile      = ".stignore"
	defaultInitValues = "Use default values"
)

// RunInitV1 runs the sequence to generate okteto.yml
func (*Command) RunInitV1(ctx context.Context, opts *InitOpts) error {
	oktetoLog.Println("This command walks you through creating an okteto manifest.")
	oktetoLog.Println("It only covers the most common items, and tries to guess sensible defaults.")
	oktetoLog.Println("See https://okteto.com/docs/reference/manifest/ for the official documentation about the okteto manifest.")

	if err := validateDevPath(opts.DevPath, opts.Overwrite); err != nil {
		return err
	}

	checkForRunningApp := false
	if opts.Language == "" {
		checkForRunningApp = true
	}
	var err error
	opts.Language, err = GetLanguage(opts.Language, opts.Workdir)
	if err != nil {
		return err
	}

	dev, err := linguist.GetDevDefaults(opts.Language, opts.Workdir, registry.ImageMetadata{})
	if err != nil {
		return err
	}

	if checkForRunningApp {
		app, container, err := getRunningApp(ctx)
		if err != nil {
			return err
		}
		if app == nil {
			dev.Autocreate = true
			linguist.SetForwardDefaults(dev, opts.Language)
		} else {
			dev.Container = container
			if container == "" {
				container = app.PodSpec().Containers[0].Name
			}

			path := getPathFromApp(opts.Workdir, app.ObjectMeta().Name)

			suffix := fmt.Sprintf("Analyzing %s '%s'...", strings.ToLower(app.Kind()), app.ObjectMeta().Name)
			oktetoLog.Spinner(suffix)
			oktetoLog.StartSpinner()
			err = initCMD.SetDevDefaultsFromApp(ctx, dev, app, container, opts.Language, path)
			if err == nil {
				oktetoLog.Success(fmt.Sprintf("%s '%s' successfully analyzed", strings.ToLower(app.Kind()), app.ObjectMeta().Name))
			} else {
				oktetoLog.Yellow(fmt.Sprintf("%s '%s' analysis failed: %s", strings.ToLower(app.Kind()), app.ObjectMeta().Name, err))
				linguist.SetForwardDefaults(dev, opts.Language)
			}
		}

		if !supportsPersistentVolumes(ctx) {
			oktetoLog.Yellow("Default storage class not found in your cluster. Persistent volumes not enabled in your okteto manifest")
			dev.Volumes = nil
			dev.PersistentVolumeInfo = &model.PersistentVolumeInfo{
				Enabled: false,
			}
		}
	} else {
		linguist.SetForwardDefaults(dev, opts.Language)
		dev.PersistentVolumeInfo = &model.PersistentVolumeInfo{
			Enabled: true,
		}
	}

	dev.Namespace = ""
	dev.Context = ""
	if err := dev.Save(opts.DevPath); err != nil {
		return err
	}

	devDir, err := filepath.Abs(filepath.Dir(opts.DevPath))
	if err != nil {
		return err
	}
	stignore := filepath.Join(devDir, stignoreFile)

	if !filesystem.FileExists(stignore) {
		c := linguist.GetSTIgnore(opts.Language)
		if err := os.WriteFile(stignore, c, 0600); err != nil {
			oktetoLog.Infof("failed to write stignore file: %s", err)
		}
	}

	analytics.TrackInit(true, opts.Language)
	return nil
}

func getRunningApp(ctx context.Context) (apps.App, string, error) {
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		oktetoLog.Yellow("Failed to load your kubeconfig: %s", err)
		return nil, "", nil
	}

	app, err := askForRunningApp(ctx, c)
	if err != nil {
		return nil, "", err
	}
	if app == nil {
		return nil, "", nil
	}

	if apps.IsDevModeOn(app) {
		return nil, "", fmt.Errorf("%s '%s' is in development mode", app.Kind(), app.ObjectMeta().Name)
	}

	container := ""
	if len(app.PodSpec().Containers) > 1 {
		container, err = askForContainer(app)
		if err != nil {
			return nil, "", err
		}
	}

	return app, container, nil
}

func supportsPersistentVolumes(ctx context.Context) bool {
	if okteto.IsOkteto() {
		return true
	}
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		oktetoLog.Infof("couldn't get kubernetes local client: %s", err.Error())
		return false
	}

	stClassList, err := c.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		oktetoLog.Infof("error getting storage classes: %s", err.Error())
		return false
	}

	for i := range stClassList.Items {
		if stClassList.Items[i].Annotations[model.DefaultStorageClassAnnotation] == "true" {
			oktetoLog.Infof("found default storage class '%s'", stClassList.Items[i].Name)
			return true
		}
	}

	oktetoLog.Infof("default storage class not found")
	return false
}

// GetLanguage returns the language of a given folder
func GetLanguage(language, workDir string) (string, error) {
	if language != "" {
		return language, nil
	}
	l, err := linguist.ProcessDirectory(workDir)
	if err != nil {
		oktetoLog.Infof("failed to process directory: %s", err)
		l = linguist.Unrecognized
	}
	oktetoLog.Infof("language '%s' inferred for your current directory", l)
	if l == linguist.Unrecognized {
		l, err = askForLanguage()
		if err != nil {
			return "", err
		}
	}
	return l, nil
}

func askForLanguage() (string, error) {
	supportedLanguages := linguist.GetSupportedLanguages()
	return utils.AskForOptions(
		supportedLanguages,
		"Couldn't detect any language in the current folder. Pick your project's main language from the list below:",
	)
}

func askForRunningApp(ctx context.Context, c kubernetes.Interface) (apps.App, error) {
	namespace := okteto.GetContext().Namespace
	dList, err := deployments.List(ctx, namespace, "", c)
	if err != nil {
		oktetoLog.Yellow("Failed to list deployments: %s", err)
		return nil, nil
	}
	sfsList, err := statefulsets.List(ctx, namespace, "", c)
	if err != nil {
		oktetoLog.Yellow("Failed to list statefulsets: %s", err)
		return nil, nil
	}
	options := []string{}
	for i := range dList {
		if dList[i].Labels[constants.DevLabel] != "" {
			continue
		}
		if dList[i].Labels[model.DevCloneLabel] != "" {
			continue
		}
		options = append(options, dList[i].Name)
	}
	for i := range sfsList {
		if sfsList[i].Labels[constants.DevLabel] != "" {
			continue
		}
		if sfsList[i].Labels[model.DevCloneLabel] != "" {
			continue
		}
		options = append(options, sfsList[i].Name)
	}
	options = append(options, defaultInitValues)
	option, err := utils.AskForOptions(
		options,
		"Select the resource you want to develop:",
	)
	if err != nil {
		return nil, err
	}
	if option == defaultInitValues {
		return nil, nil
	}
	for i := range dList {
		if dList[i].Name == option {
			return apps.NewDeploymentApp(&dList[i]), nil
		}
	}
	for i := range sfsList {
		if sfsList[i].Name == option {
			return apps.NewStatefulSetApp(&sfsList[i]), nil
		}
	}

	return nil, nil
}

func askForContainer(app apps.App) (string, error) {
	options := []string{}
	for _, c := range app.PodSpec().Containers {
		options = append(options, c.Name)
	}
	return utils.AskForOptions(
		options,
		fmt.Sprintf("%s '%s' has %d containers. Select the container you want to replace with your development container:", app.Kind(), app.ObjectMeta().Name, len(app.PodSpec().Containers)),
	)
}

func validateDevPath(devPath string, overwrite bool) error {
	if !overwrite && filesystem.FileExists(devPath) {
		return fmt.Errorf("%s already exists. Run this command again with the '--replace' flag to overwrite it", devPath)
	}
	return nil
}

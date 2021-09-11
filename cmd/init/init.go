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

package init

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	initCMD "github.com/okteto/okteto/pkg/cmd/init"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/client"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/linguist"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	stignoreFile      = ".stignore"
	secondaryManifest = "okteto.yaml"
	defaultInitValues = "Use default values"
)

// Init automatically generates the manifest
func Init() *cobra.Command {
	var namespace string
	var k8sContext string
	var devPath string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "init",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#init"),
		Short: "Automatically generates your okteto manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := os.Getenv("OKTETO_LANGUAGE")
			workDir, err := os.Getwd()
			if err != nil {
				return err
			}

			if err := Run(namespace, k8sContext, devPath, l, workDir, overwrite); err != nil {
				return err
			}

			log.Success(fmt.Sprintf("okteto manifest (%s) created", devPath))

			if devPath == utils.DefaultDevManifest {
				log.Information("Run 'okteto up' to activate your development container")
			} else {
				log.Information("Run 'okteto up -f %s' to activate your development container", devPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace target for generating the okteto manifest")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context target for generating the okteto manifest")
	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&overwrite, "overwrite", "o", false, "overwrite existing manifest file")
	return cmd
}

// Run runs the sequence to generate okteto.yml
func Run(namespace, k8sContext, devPath, language, workDir string, overwrite bool) error {
	if k8sContext == "" {
		k8sContext = os.Getenv(client.OktetoContextVariableName)
	}

	fmt.Println("This command walks you through creating an okteto manifest.")
	fmt.Println("It only covers the most common items, and tries to guess sensible defaults.")
	fmt.Println("See https://okteto.com/docs/reference/manifest/ for the official documentation about the okteto manifest.")
	ctx := context.Background()
	devPath, err := validateDevPath(devPath, overwrite)
	if err != nil {
		return err
	}

	checkForRunningApp := false
	if language == "" {
		checkForRunningApp = true
	}

	language, err = GetLanguage(language, workDir)
	if err != nil {
		return err
	}

	dev, err := linguist.GetDevDefaults(language, workDir)
	if err != nil {
		return err
	}

	dev.Context = k8sContext

	if checkForRunningApp {
		app, container, err := getRunningApp(ctx, namespace, k8sContext)
		if err != nil {
			return err
		}
		if app == nil {
			dev.Autocreate = true
			linguist.SetForwardDefaults(dev, language)
		} else {
			dev.Container = container
			if container == "" {
				container = app.PodSpec().Containers[0].Name
			}

			suffix := fmt.Sprintf("Analyzing %s '%s'...", app.Kind(), app.Name())
			spinner := utils.NewSpinner(suffix)
			spinner.Start()
			err = initCMD.SetDevDefaultsFromApp(ctx, dev, app, container, language)
			spinner.Stop()
			if err == nil {
				log.Success(fmt.Sprintf("%s '%s' successfully analyzed", app.Kind(), app.Name()))
			} else {
				log.Yellow(fmt.Sprintf("%s '%s' analysis failed: %s", app.Kind(), app.Name(), err))
				linguist.SetForwardDefaults(dev, language)
			}
		}

		if !supportsPersistentVolumes(ctx, namespace, k8sContext) {
			log.Yellow("Default storage class not found in your cluster. Persistent volumes not enabled in your okteto manifest")
			dev.Volumes = nil
			dev.PersistentVolumeInfo = &model.PersistentVolumeInfo{
				Enabled: false,
			}
		}
	} else {
		linguist.SetForwardDefaults(dev, language)
		dev.PersistentVolumeInfo = &model.PersistentVolumeInfo{
			Enabled: true,
		}
	}

	if err := dev.Save(devPath); err != nil {
		return err
	}

	devDir, err := filepath.Abs(filepath.Dir(devPath))
	if err != nil {
		return err
	}
	stignore := filepath.Join(devDir, stignoreFile)

	if !model.FileExists(stignore) {
		c := linguist.GetSTIgnore(language)
		if err := ioutil.WriteFile(stignore, c, 0600); err != nil {
			log.Infof("failed to write stignore file: %s", err)
		}
	}

	analytics.TrackInit(true, language)
	return nil
}

func getRunningApp(ctx context.Context, namespace, k8sContext string) (apps.App, string, error) {
	c, _, err := k8Client.GetLocalWithContext(k8sContext)
	if err != nil {
		log.Yellow("Failed to load your local Kubeconfig: %s", err)
		return nil, "", nil
	}
	if namespace == "" {
		namespace = client.GetContextNamespace(k8sContext)
	}

	app, err := askForRunningApp(ctx, namespace, c)
	if err != nil {
		return nil, "", err
	}
	if app == nil {
		return nil, "", nil
	}

	if app.IsDevModeOn() {
		return nil, "", fmt.Errorf("%s '%s' is in development mode", app.Kind(), app.Name())
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

func supportsPersistentVolumes(ctx context.Context, namespace, k8sContext string) bool {
	c, _, err := k8Client.GetLocalWithContext(k8sContext)
	if err != nil {
		log.Infof("couldn't get kubernetes local client: %s", err.Error())
		return false
	}
	if namespace == "" {
		namespace = client.GetContextNamespace(k8sContext)
	}

	ns, err := namespaces.Get(ctx, namespace, c)
	if err != nil {
		log.Infof("failed to get the current namespace: %s", err.Error())
		return false
	}

	if namespaces.IsOktetoNamespace(ns) {
		return true
	}

	stClassList, err := c.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Infof("error getting storage classes: %s", err.Error())
		return false
	}

	for i := range stClassList.Items {
		if stClassList.Items[i].Annotations[model.DefaultStorageClassAnnotation] == "true" {
			log.Infof("found default storage class '%s'", stClassList.Items[i].Name)
			return true
		}
	}

	log.Infof("default storage class not found")
	return false
}

func validateDevPath(devPath string, overwrite bool) (string, error) {
	if !overwrite {
		if model.FileExists(devPath) {
			return "", fmt.Errorf("%s already exists. Run this command again with the '-o' flag to overwrite it", devPath)
		}
	}

	return devPath, nil
}

// GetLanguage returns the language of a given folder
func GetLanguage(language, workDir string) (string, error) {
	if language != "" {
		return language, nil
	}
	l, err := linguist.ProcessDirectory(workDir)
	if err != nil {
		log.Infof("failed to process directory: %s", err)
		l = linguist.Unrecognized
	}
	log.Infof("language '%s' inferred for your current directory", l)
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
	return askForOptions(
		supportedLanguages,
		"Couldn't detect any language in the current folder. Pick your project's main language from the list below:",
	)
}

func askForRunningApp(ctx context.Context, namespace string, c kubernetes.Interface) (apps.App, error) {
	dList, err := deployments.List(ctx, namespace, "", c)
	if err != nil {
		log.Yellow("Failed to list deployments: %s", err)
		return nil, nil
	}
	sfsList, err := statefulsets.List(ctx, namespace, "", c)
	if err != nil {
		log.Yellow("Failed to list statefulsets: %s", err)
		return nil, nil
	}
	options := []string{}
	for i := range dList {
		options = append(options, dList[i].Name)
	}
	for i := range sfsList {
		options = append(options, sfsList[i].Name)
	}
	options = append(options, defaultInitValues)
	option, err := askForOptions(
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
	return askForOptions(
		options,
		fmt.Sprintf("%s '%s' has %d containers. Select the container you want to replace with your development container:", app.Kind(), app.Name(), len(app.PodSpec().Containers)),
	)
}

func askForOptions(options []string, label string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: options,
		Size:  len(options),
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Selected: " ✓  {{ . | oktetoblue }}",
			Active:   fmt.Sprintf("%s {{ . | oktetoblue }}", promptui.IconSelect),
			Inactive: "  {{ . | oktetoblue }}",
			FuncMap:  promptui.FuncMap,
		},
	}

	prompt.Templates.FuncMap["oktetoblue"] = log.BlueString

	i, _, err := prompt.Run()
	if err != nil {
		log.Infof("invalid init option: %s", err)
		return "", fmt.Errorf("invalid option")
	}

	return options[i], nil
}

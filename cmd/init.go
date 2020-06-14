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
	"io/ioutil"
	"os"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	initCMD "github.com/okteto/okteto/pkg/cmd/init"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/linguist"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	stignore          = ".stignore"
	defaultManifest   = "okteto.yml"
	secondaryManifest = "okteto.yaml"
	createDeployment  = "new deployment"
)

var wrongImageNames = map[string]bool{
	"T":     true,
	"TRUE":  true,
	"Y":     true,
	"YES":   true,
	"F":     true,
	"FALSE": true,
	"N":     true,
	"NO":    true,
}

//Init automatically generates the manifest
func Init() *cobra.Command {
	var namespace string
	var devPath string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Automatically generates your okteto manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := os.Getenv("OKTETO_LANGUAGE")
			workDir, err := os.Getwd()
			if err != nil {
				return err
			}

			if err := executeInit(namespace, devPath, l, workDir, overwrite); err != nil {
				return err
			}

			log.Success(fmt.Sprintf("Okteto manifest (%s) created. Check its content in case you need to adapt it", devPath))
			log.Information("Run 'okteto up' to activate your development container")
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace target for generating the okteto manifest")
	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&overwrite, "overwrite", "o", false, "overwrite existing manifest file")
	return cmd
}

func executeInit(namespace, devPath, language, workDir string, overwrite bool) error {
	devPath, err := validateDevPath(devPath, overwrite)
	if err != nil {
		return err
	}

	ifAskingForDeployment := language == ""

	language, err = getLanguage(language, workDir)
	if err != nil {
		return err
	}

	dev, err := linguist.GetDevDefaults(language, workDir, ifAskingForDeployment)
	if err != nil {
		return err
	}

	if ifAskingForDeployment {
		d, container, err := getDeployment(namespace)
		if err != nil {
			return err
		}
		dev.Container = container
		if container == "" {
			container = d.Spec.Template.Spec.Containers[0].Name
		}

		postfix := fmt.Sprintf("Analyzing deployment '%s'...", d.Name)
		spinner := utils.NewSpinner(postfix)
		spinner.Start()
		defer spinner.Stop()
		dev, err = initCMD.SetDevDefaultsFromDeployment(dev, d, container)
		if err != nil {
			return err
		}
		spinner.Stop()
		log.Success(fmt.Sprintf("Deployment '%s' successfully analized", d.Name))
	}

	if err := dev.Save(devPath); err != nil {
		return err
	}

	if !model.FileExists(stignore) {
		log.Debugf("getting stignore for %s", language)
		c := linguist.GetSTIgnore(language)
		if err := ioutil.WriteFile(stignore, c, 0600); err != nil {
			log.Infof("failed to write stignore file: %s", err)
		}
	}

	analytics.TrackInit(true, language)
	return nil
}

func getDeployment(namespace string) (*appsv1.Deployment, string, error) {
	c, _, currentNamespace, err := k8Client.GetLocal()
	if err != nil {
		return nil, "", fmt.Errorf("failed to load your local Kubeconfig: %s", err)
	}
	if namespace == "" {
		namespace = currentNamespace
	}

	d, err := askForDeployment(namespace, c)
	if err != nil {
		return nil, "", err
	}
	if d == nil {
		return nil, "", nil
	}

	if deployments.IsDevModeOn(d) {
		return nil, "", fmt.Errorf("the deployment '%s' is in development mode", d.Name)
	}

	container := ""
	if len(d.Spec.Template.Spec.Containers) > 1 {
		container, err = askForContainer(d)
		if err != nil {
			return nil, "", err
		}
	}

	return d, container, nil
}

func validateDevPath(devPath string, overwrite bool) (string, error) {
	if !overwrite {
		if model.FileExists(devPath) {
			return "", fmt.Errorf("%s already exists. Run this command again with the '-o' flag to overwrite it", devPath)
		}
	}

	return devPath, nil
}

func getLanguage(language, workDir string) (string, error) {
	if language != "" {
		return language, nil
	}
	l, err := linguist.ProcessDirectory(workDir)
	if err != nil {
		log.Info(err)
		return "", fmt.Errorf("Failed to determine the language of the current directory")
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
		"Couldn't detect any language in current folder. Pick your project's main language from the list below:",
	)
}

func askForDeployment(namespace string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {
	dList, err := deployments.List(namespace, c)
	if err != nil {
		return nil, err
	}
	options := []string{createDeployment}
	for i := range dList {
		options = append(options, dList[i].Name)
	}
	option, err := askForOptions(
		options,
		"Pick the deployment target for your development container from the list below:",
	)
	if err != nil {
		return nil, err
	}
	if option == createDeployment {
		return nil, nil
	}
	for i := range dList {
		if dList[i].Name == option {
			return &dList[i], nil
		}
	}
	return nil, nil
}

func askForContainer(d *appsv1.Deployment) (string, error) {
	options := []string{}
	for i := range d.Spec.Template.Spec.Containers {
		options = append(options, d.Spec.Template.Spec.Containers[i].Name)
	}
	return askForOptions(
		options,
		fmt.Sprintf("The deployment '%s' has %d containers. Pick the container target for your development container from the list below:", d.Name, len(d.Spec.Template.Spec.Containers)),
	)
}

func askForOptions(options []string, label string) (string, error) {
	prompt := promptui.Select{
		Label: label,
		Items: options,
		Size:  len(options),
		Templates: &promptui.SelectTemplates{
			Label:    fmt.Sprintf("%s {{ . }}", log.BlueString("?")),
			Selected: " ✓  {{ . | oktetoblue }}",
			Active:   fmt.Sprintf("%s {{ . | oktetoblue }}", promptui.IconSelect),
			Inactive: "  {{ . | oktetoblue }}",
			FuncMap:  promptui.FuncMap,
		},
	}

	prompt.Templates.FuncMap["oktetoblue"] = log.BlueString

	i, _, err := prompt.Run()
	if err != nil {
		log.Debugf("invalid init option: %s", err)
		return "", fmt.Errorf("invalid option")
	}

	return options[i], nil
}

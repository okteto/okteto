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

package utils

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

const (
	//DefaultDevManifest default okteto manifest file
	DefaultDevManifest   = "okteto.yml"
	secondaryDevManifest = "okteto.yaml"
)

func LoadDevContext(devPath string) (*model.ContextResource, error) {
	if !model.FileExists(devPath) {
		if devPath == DefaultDevManifest {
			if model.FileExists(secondaryDevManifest) {
				return model.GetContextResource(secondaryDevManifest)
			}
		}
		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}
	return model.GetContextResource(devPath)
}

//LoadDev loads an okteto manifest checking "yml" and "yaml"
func LoadDev(devPath string) (*model.Dev, error) {
	if !model.FileExists(devPath) {
		if devPath == DefaultDevManifest {
			if model.FileExists(secondaryDevManifest) {
				return LoadDev(secondaryDevManifest)
			}
		}

		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}

	dev, err := model.Get(devPath)
	if err != nil {
		return nil, err
	}

	if err := loadDevRc(dev); err != nil {
		return nil, err
	}

	dev.Namespace = okteto.Context().Namespace
	dev.Context = okteto.Context().Name

	return dev, nil
}

func loadDevRc(dev *model.Dev) error {
	defaultDevRcPath := filepath.Join(config.GetOktetoHome(), "okteto.yml")
	secondaryDevRcPath := filepath.Join(config.GetOktetoHome(), "okteto.yaml")
	var devRc *model.DevRC
	var err error
	if model.FileExists(defaultDevRcPath) {
		devRc, err = model.GetRc(defaultDevRcPath)
		if err != nil {
			return fmt.Errorf("error while reading %s file: %s", defaultDevRcPath, err.Error())
		}
	} else if model.FileExists(secondaryDevRcPath) {
		devRc, err = model.GetRc(secondaryDevRcPath)
		if err != nil {
			return fmt.Errorf("error while reading %s file: %s", defaultDevRcPath, err.Error())
		}
	}

	if devRc != nil {
		model.MergeDevWithDevRc(dev, devRc)
	}
	return nil
}

//LoadDevOrDefault loads an okteto manifest or a default one if does not exist
func LoadDevOrDefault(devPath, name string) (*model.Dev, error) {
	dev, err := LoadDev(devPath)
	if err == nil {
		return dev, nil
	}

	if errors.IsNotExist(err) && len(name) > 0 {
		dev, err := model.Read(nil)
		if err != nil {
			return nil, err
		}
		dev.Name = name
		return dev, nil
	}

	return nil, err
}

//AskYesNo prompts for yes/no confirmation
func AskYesNo(q string) (bool, error) {
	var answer string
	for {
		fmt.Print(q)
		if _, err := fmt.Scanln(&answer); err != nil {
			return false, err
		}

		if answer == "y" || answer == "n" {
			break
		}

		log.Fail("input must be 'y' or 'n'")
	}

	return answer == "y", nil
}

func AskForOptions(options []string, label string) (string, error) {
	selectedTemplate := " ✓  {{ . | oktetoblue }}"
	activeTemplate := fmt.Sprintf("%s {{ . | oktetoblue }}", promptui.IconSelect)
	inactiveTemplate := "  {{ . | oktetoblue }}"
	if runtime.GOOS == "windows" {
		selectedTemplate = " ✓  {{ . | blue }}"
		activeTemplate = fmt.Sprintf("%s {{ . | blue }}", promptui.IconSelect)
		inactiveTemplate = "  {{ . | blue }}"
	}

	prompt := promptui.Select{
		Label: label,
		Items: options,
		Size:  len(options),
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Selected: selectedTemplate,
			Active:   activeTemplate,
			Inactive: inactiveTemplate,
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

//AskIfOktetoInit asks if okteto init should be executed
func AskIfOktetoInit(devPath string) bool {
	result, err := AskYesNo(fmt.Sprintf("okteto manifest (%s) doesn't exist, do you want to create it? [y/n] ", devPath))
	if err != nil {
		return false
	}
	return result
}

//AskIfDeploy asks if a new deployment must be created
func AskIfDeploy(name, namespace string) error {
	deploy, err := AskYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one? [y/n]: ", name, namespace))
	if err != nil {
		return fmt.Errorf("couldn't read your response")
	}
	if !deploy {
		return errors.UserError{
			E:    fmt.Errorf("deployment %s doesn't exist in namespace %s", name, namespace),
			Hint: "Deploy your application first or use 'okteto namespace' to select a different namespace and try again",
		}
	}
	return nil
}

//ParseURL validates a URL
func ParseURL(u string) (string, error) {
	url, err := url.Parse(u)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid URL", u)
	}

	if url.Scheme == "" {
		url.Scheme = "https"
	}

	return strings.TrimRight(url.String(), "/"), nil
}

//CheckIfDirectory checks if a path is a directory
func CheckIfDirectory(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Infof("error on CheckIfDirectory: %s", err.Error())
		return fmt.Errorf("'%s' does not exist", path)
	}
	if fileInfo.IsDir() {
		return nil
	}
	return fmt.Errorf("'%s' is not a directory", path)
}

//CheckIfRegularFile checks if a path is a regular file
func CheckIfRegularFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Infof("error on CheckIfRegularFile: %s", err.Error())
		return fmt.Errorf("'%s' does not exist", path)
	}
	if !fileInfo.IsDir() {
		return nil
	}
	return fmt.Errorf("'%s' is not a regular file", path)
}

func GetDownCommand(devPath string) string {
	okDownCommandHint := "okteto down -v"
	if DefaultDevManifest != devPath {
		okDownCommandHint = fmt.Sprintf("okteto down -v -f %s", devPath)
	}
	return okDownCommandHint
}

func GetApp(ctx context.Context, dev *model.Dev, c kubernetes.Interface) (apps.App, bool, error) {
	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, false, err
		}
		if dev.Autocreate {
			return apps.NewDeploymentApp(deployments.Sandbox(dev)), true, nil
		}
		if len(dev.Labels) > 0 {
			if err == errors.ErrNotFound {
				err = errors.UserError{
					E:    fmt.Errorf("didn't find an application in namespace %s that matches the labels in your Okteto manifest", dev.Namespace),
					Hint: "Update the labels or point your context to a different namespace and try again"}
			}
			return nil, false, err
		}
		return nil, false, errors.UserError{
			E: fmt.Errorf("application '%s' not found in namespace '%s'", dev.Name, dev.Namespace),
			Hint: `Verify that your application has been deployed and your Kubernetes context is pointing to the right namespace
    Or set the 'autocreate' field in your okteto manifest if you want to create a standalone development container
    More information is available here: https://okteto.com/docs/reference/cli/#up`,
		}
	}
	if dev.Divert != nil {
		dev.Name = model.DivertName(dev.Name, okteto.GetSanitizedUsername())
		return app.Divert(okteto.GetSanitizedUsername()), false, nil
	}
	return app, false, nil
}

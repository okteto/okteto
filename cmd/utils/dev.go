// Copyright 2022 The Okteto Authors
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
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/prompt"
	"k8s.io/client-go/kubernetes"
)

var (
	// ErrNoDevSelected is raised when no development environment is selected
	ErrNoDevSelected = errors.New("No Development Environment selected")
)

const (
	//DefaultManifest default okteto manifest file
	DefaultManifest   = "okteto.yml"
	secondaryManifest = "okteto.yaml"
)

// LoadManifestContext loads the contextresource from a file
func LoadManifestContext(devPath string) (*model.ContextResource, error) {
	if !filesystem.FileExists(devPath) {
		if devPath == DefaultManifest {
			if filesystem.FileExists(secondaryManifest) {
				return model.GetContextResource(secondaryManifest)
			}
		}
		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}
	return model.GetContextResource(devPath)
}

// LoadManifest loads an okteto manifest checking "yml" and "yaml"
func LoadManifest(devPath string) (*model.Manifest, error) {
	if !filesystem.FileExists(devPath) {
		if devPath == DefaultManifest {
			if filesystem.FileExists(secondaryManifest) {
				return LoadManifest(secondaryManifest)
			}
		}

		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}

	manifest, err := model.Get(devPath)
	if err != nil {
		return nil, err
	}

	if manifest.Name == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		manifest.Name = InferName(cwd)
	}
	if manifest.Namespace == "" {
		manifest.Namespace = okteto.Context().Namespace
	}

	if manifest.Context == "" {
		manifest.Context = okteto.Context().Name
	}

	for _, dev := range manifest.Dev {
		if err := LoadManifestRc(dev); err != nil {
			return nil, err
		}

		dev.Namespace = okteto.Context().Namespace
		dev.Context = okteto.Context().Name
	}

	return manifest, nil
}

func LoadManifestRc(dev *model.Dev) error {
	defaultDevRcPath := filepath.Join(config.GetOktetoHome(), "okteto.yml")
	secondaryDevRcPath := filepath.Join(config.GetOktetoHome(), "okteto.yaml")
	var devRc *model.DevRC
	var err error
	if filesystem.FileExists(defaultDevRcPath) {
		devRc, err = model.GetRc(defaultDevRcPath)
		if err != nil {
			return fmt.Errorf("error while reading %s file: %s", defaultDevRcPath, err.Error())
		}
	} else if filesystem.FileExists(secondaryDevRcPath) {
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

// LoadManifestOrDefault loads an okteto manifest or a default one if does not exist
func LoadManifestOrDefault(devPath, name string) (*model.Manifest, error) {
	dev, err := LoadManifest(devPath)
	if err == nil {
		return dev, nil
	}

	if oktetoErrors.IsNotExist(err) && len(name) > 0 {
		manifest, err := model.Read(nil)
		if err != nil {
			return nil, err
		}
		manifest.Dev[name] = model.NewDev()
		manifest.Dev[name].Name = name
		manifest.Dev[name].Namespace = okteto.Context().Namespace
		manifest.Dev[name].Context = okteto.Context().Name
		if err := manifest.Dev[name].SetDefaults(); err != nil {
			return nil, err
		}
		return manifest, nil
	}

	return nil, err
}

// GetDevFromManifest returns the dev for devName
func GetDevFromManifest(manifest *model.Manifest, devName string) (*model.Dev, error) {
	if len(manifest.Dev) == 0 {
		return nil, oktetoErrors.ErrManifestNoDevSection
	}

	options := []string{}
	for name := range manifest.Dev {
		options = append(options, name)
	}

	// if devName is empty and manifest only has one dev, return this
	if len(manifest.Dev) == 1 && devName == "" {
		for _, dev := range manifest.Dev {
			return dev, nil
		}
	}

	if devName == "" {
		return nil, ErrNoDevSelected
	}

	for _, item := range options {
		if item == devName {
			return manifest.Dev[devName], nil
		}
	}
	return nil, oktetoErrors.UserError{
		E:    fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, devName),
		Hint: fmt.Sprintf("Available options are: [%s]", strings.Join(options, ", ")),
	}
}

func SelectDevFromManifest(manifest *model.Manifest, selector prompt.OktetoSelectorInterface) (*model.Dev, error) {
	devName, err := selector.Ask()
	if err != nil {
		return nil, err
	}

	dev := manifest.Dev[devName]
	dev.Name = devName
	if dev.Namespace == "" {
		dev.Namespace = manifest.Namespace
	}

	if dev.Context == "" {
		dev.Context = manifest.Context
	}
	if err := dev.Validate(); err != nil {
		return nil, err
	}

	return manifest.Dev[devName], nil
}

func AskForOptions(options []string, label string) (string, error) {
	selectedTemplate := `{{ " ✓ " | bgGreen | black }} {{ .Label | green }}`
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
	prompt.Templates.FuncMap["oktetoblue"] = oktetoLog.BlueString

	i, _, err := prompt.Run()
	if err != nil {
		oktetoLog.Infof("invalid init option: %s", err)
		return "", fmt.Errorf("invalid option")
	}

	return options[i], nil
}

// ParseURL validates a URL
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

// CheckIfDirectory checks if a path is a directory
func CheckIfDirectory(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		oktetoLog.Infof("error on CheckIfDirectory: %s", err.Error())
		return fmt.Errorf("'%s' does not exist", path)
	}
	if fileInfo.IsDir() {
		return nil
	}
	return fmt.Errorf("'%s' is not a directory", path)
}

// CheckIfRegularFile checks if a path is a regular file
func CheckIfRegularFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		oktetoLog.Infof("error on CheckIfRegularFile: %s", err.Error())
		return fmt.Errorf("'%s' does not exist", path)
	}
	if !fileInfo.IsDir() {
		return nil
	}
	return fmt.Errorf("'%s' is not a regular file", path)
}

func GetDownCommand(devPath string) string {
	okDownCommandHint := "okteto down -v"
	if DefaultManifest != devPath && devPath != "" {
		okDownCommandHint = fmt.Sprintf("okteto down -v -f %s", devPath)
	}
	return okDownCommandHint
}

func GetApp(ctx context.Context, dev *model.Dev, c kubernetes.Interface, isRetry bool) (apps.App, bool, error) {
	app, err := apps.Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return nil, false, err
		}
		if dev.Autocreate {
			if isRetry && !doesAutocreateAppExist(ctx, dev, c) {
				return nil, false, fmt.Errorf("development container has been deactivated")
			}
			return apps.NewDeploymentApp(deployments.Sandbox(dev)), true, nil
		}
		if len(dev.Selector) > 0 {
			if err == oktetoErrors.ErrNotFound {
				err = oktetoErrors.UserError{
					E:    fmt.Errorf("didn't find an application in namespace %s that matches the labels in your Okteto manifest", dev.Namespace),
					Hint: "Update the labels or point your context to a different namespace and try again"}
			}
			return nil, false, err
		}
		return nil, false, oktetoErrors.UserError{
			E: fmt.Errorf("application '%s' not found in namespace '%s'", dev.Name, dev.Namespace),
			Hint: `Verify that your application is running and your okteto context is pointing to the right namespace
    Or set the 'autocreate' field in your okteto manifest if you want to create a standalone development container
    More information is available here: https://okteto.com/docs/reference/cli/#up`,
		}
	}
	return app, false, nil
}

func doesAutocreateAppExist(ctx context.Context, dev *model.Dev, c kubernetes.Interface) bool {
	autocreateDev := *dev
	autocreateDev.Name = model.DevCloneName(dev.Name)
	_, err := apps.Get(ctx, &autocreateDev, dev.Namespace, c)
	if err != nil && !oktetoErrors.IsNotFound(err) {
		oktetoLog.Infof("getApp autocreate k8s error, retrying...")
		_, err := apps.Get(ctx, &autocreateDev, dev.Namespace, c)
		return err == nil
	}
	return err == nil
}

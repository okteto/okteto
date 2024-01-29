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

package utils

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/devenvironment"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

var (
	// ErrNoDevSelected is raised when no development environment is selected
	ErrNoDevSelected = errors.New("No Development Environment selected")
)

const (
	// DefaultManifest default okteto manifest file
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

// DeprecatedLoadManifest loads an okteto manifest checking "yml" and "yaml".
// Deprecated: use model.GetManifestV2 instead
func DeprecatedLoadManifest(devPath string) (*model.Manifest, error) {
	if !filesystem.FileExists(devPath) {
		if devPath == DefaultManifest {
			if filesystem.FileExists(secondaryManifest) {
				return DeprecatedLoadManifest(secondaryManifest)
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
		manifest.Name = devenvironment.DeprecatedInferName(cwd)
	}
	if manifest.Namespace == "" {
		manifest.Namespace = okteto.GetContext().Namespace
	}

	if manifest.Context == "" {
		manifest.Context = okteto.GetContext().Name
	}

	for _, dev := range manifest.Dev {
		if err := LoadManifestRc(dev); err != nil {
			return nil, err
		}

		dev.Namespace = okteto.GetContext().Namespace
		dev.Context = okteto.GetContext().Name
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
			return fmt.Errorf("error while reading %s file: %w", defaultDevRcPath, err)
		}
	} else if filesystem.FileExists(secondaryDevRcPath) {
		devRc, err = model.GetRc(secondaryDevRcPath)
		if err != nil {
			return fmt.Errorf("error while reading %s file: %w", defaultDevRcPath, err)
		}
	}

	if devRc != nil {
		model.MergeDevWithDevRc(dev, devRc)
	}
	return nil
}

// DeprecatedLoadManifestOrDefault loads an okteto manifest or a default one if does not exist
// Deprecatd. It should only be used by `push` command that will be deleted on next major version. No new usages should be added
func DeprecatedLoadManifestOrDefault(devPath, name string) (*model.Manifest, error) {
	dev, err := DeprecatedLoadManifest(devPath)
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
		manifest.Dev[name].Namespace = okteto.GetContext().Namespace
		manifest.Dev[name].Context = okteto.GetContext().Name
		if err := manifest.Dev[name].SetDefaults(); err != nil {
			return nil, err
		}
		return manifest, nil
	}

	return nil, err
}

// GetDevFromManifest gets a dev from a manifest by comparing the given dev name with the dev name in the manifest
func GetDevFromManifest(manifest *model.Manifest, devName string) (*model.Dev, error) {
	if len(manifest.Dev) == 0 {
		return nil, oktetoErrors.ErrManifestNoDevSection
	} else if len(manifest.Dev) == 1 {
		for name, dev := range manifest.Dev {
			if devName != "" && devName != name {
				return nil, oktetoErrors.UserError{
					E:    fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, devName),
					Hint: fmt.Sprintf("Available options are: [%s]", name),
				}
			}
			return dev, nil
		}
	}

	if devName == "" {
		return nil, ErrNoDevSelected
	}

	var options []string
	for name, dev := range manifest.Dev {
		if name == devName {
			return dev, nil
		}
		options = append(options, name)
	}
	return nil, oktetoErrors.UserError{
		E:    fmt.Errorf(oktetoErrors.ErrDevContainerNotExists, devName),
		Hint: fmt.Sprintf("Available options are: [%s]", strings.Join(options, ", ")),
	}
}

// SelectDevFromManifest prompts the selector to choose a development container and returns the dev selected or error
func SelectDevFromManifest(manifest *model.Manifest, selector OktetoSelectorInterface, devs []string) (*model.Dev, error) {
	sort.Slice(devs, func(i, j int) bool {
		l1, l2 := len(devs[i]), len(devs[j])
		if l1 != l2 {
			return l1 < l2
		}
		return devs[i] < devs[j]
	})
	var items []SelectorItem
	for _, dev := range devs {
		items = append(items, SelectorItem{
			Name:   dev,
			Label:  dev,
			Enable: true,
		})
	}
	devKey, err := selector.AskForOptionsOkteto(items, -1)
	if err != nil {
		return nil, err
	}
	dev := manifest.Dev[devKey]

	dev.Name = devKey
	if dev.Namespace == "" {
		dev.Namespace = manifest.Namespace
	}

	if dev.Context == "" {
		dev.Context = manifest.Context
	}
	if err := dev.Validate(); err != nil {
		return nil, err
	}

	return manifest.Dev[devKey], nil
}

// YesNoDefault specifies what will be assumed when the user doesn't answer explicitly
type YesNoDefault string

const (
	YesNoDefault_Unspecified = "[y/n]"
	YesNoDefault_Yes         = "[Y/n]"
	YesNoDefault_No          = "[y/N]"
)

// AskYesNo prompts for yes/no confirmation
func AskYesNo(q string, d YesNoDefault) (bool, error) {
	var answer string
	for {
		if err := oktetoLog.Question(fmt.Sprintf("%s %s: ", q, d)); err != nil {
			return false, err
		}
		if _, err := fmt.Scanln(&answer); err != nil && err.Error() != "unexpected newline" {
			return false, err
		}

		if answer == "" && d != YesNoDefault_Unspecified {
			answer = "y"
			if d == YesNoDefault_No {
				answer = "n"
			}
			break
		}

		if answer == "y" || answer == "Y" || answer == "n" || answer == "N" {
			break
		}

		oktetoLog.Fail("input must be 'Y/y' or 'N/n'")
	}

	if answer == "y" || answer == "Y" {
		return true, nil
	}
	return false, nil
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

// AskIfOktetoInit asks if okteto init should be executed
func AskIfOktetoInit(devPath string) bool {
	result, err := AskYesNo(fmt.Sprintf("okteto manifest (%s) doesn't exist, do you want to create it?", devPath), YesNoDefault_Yes)
	if err != nil {
		return false
	}
	return result
}

// AsksQuestion asks a question to the user
func AsksQuestion(q string) (string, error) {
	var answer string

	if err := oktetoLog.Question(q); err != nil {
		oktetoLog.Infof("failed to ask question: %s", err)
	}
	if _, err := fmt.Scanln(&answer); err != nil {
		return "", err
	}

	return answer, nil
}

// AskIfDeploy asks if a new deployment must be created
func AskIfDeploy(name, namespace string) error {
	deploy, err := AskYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one?", name, namespace), YesNoDefault_Yes)
	if err != nil {
		return fmt.Errorf("couldn't read your response")
	}
	if !deploy {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("deployment %s doesn't exist in namespace %s", name, namespace),
			Hint: "Deploy your application first or use 'okteto namespace' to select a different namespace and try again",
		}
	}
	return nil
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
			if oktetoErrors.IsNotFound(err) {
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

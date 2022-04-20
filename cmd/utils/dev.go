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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/client-go/kubernetes"
)

const (
	//DefaultManifest default okteto manifest file
	DefaultManifest   = "okteto.yml"
	secondaryManifest = "okteto.yaml"
	detachModePodName = "okteto-dev-env"
)

func LoadManifestContext(devPath string) (*model.ContextResource, error) {
	if !model.FileExists(devPath) {
		if devPath == DefaultManifest {
			if model.FileExists(secondaryManifest) {
				return model.GetContextResource(secondaryManifest)
			}
		}
		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}
	return model.GetContextResource(devPath)
}

//LoadManifest loads an okteto manifest checking "yml" and "yaml"
func LoadManifest(devPath string) (*model.Manifest, error) {
	if !model.FileExists(devPath) {
		if devPath == DefaultManifest {
			if model.FileExists(secondaryManifest) {
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

//LoadManifestOrDefault loads an okteto manifest or a default one if does not exist
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

func GetDevFromManifest(manifest *model.Manifest, devName string) (*model.Dev, error) {
	if len(manifest.Dev) == 0 {
		return nil, fmt.Errorf("okteto manifest has no 'dev' section. Configure it with 'okteto init'")
	} else if len(manifest.Dev) == 1 {
		for name, dev := range manifest.Dev {
			if devName != "" && devName != name {
				return nil, fmt.Errorf("dev '%s' doesn't exist", devName)
			}
			return dev, nil
		}
	}

	if devName != "" {
		for k := range manifest.Dev {
			if k == devName {
				return manifest.Dev[devName], nil
			}
		}
		return nil, fmt.Errorf("development container '%s' doesn't exist", devName)
	}
	devs := []string{}
	for k := range manifest.Dev {
		devs = append(devs, k)
	}
	sort.Slice(devs, func(i, j int) bool {
		l1, l2 := len(devs[i]), len(devs[j])
		if l1 != l2 {
			return l1 < l2
		}
		return devs[i] < devs[j]
	})
	items := []SelectorItem{}
	for _, dev := range devs {
		items = append(items, SelectorItem{
			Name:   dev,
			Label:  dev,
			Enable: true,
		})
	}
	devKey, _, err := AskForOptionsOkteto(context.Background(), items, "Select the development container you want to activate:", "Development container")
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

// GetDevDetachMode returns a dev manifest from a
func GetDevDetachMode(manifest *model.Manifest, devs []string) (*model.Dev, error) {
	dev := model.NewDev()
	dev.Autocreate = true

	if manifest.Type == model.StackType {
		for svcName, svc := range manifest.Deploy.ComposeSection.Stack.Services {
			d, err := svc.ToDev(svcName)
			if err != nil {
				return nil, err
			}
			for _, forward := range d.Forward {
				localPort := forward.Local
				if !model.IsPortAvailable(dev.Interface, forward.Local) {
					return nil, fmt.Errorf("local port %d is already in-use in your local machine", forward.Local)
				}
				dev.Forward = append(dev.Forward, model.Forward{
					Local:       localPort,
					Remote:      forward.Remote,
					ServiceName: svcName,
					Service:     true,
				})
			}
			if len(d.Sync.Folders) == 0 {
				continue
			}
			if len(devs) > 0 && !isInDevs(svcName, devs) {
				continue
			}
			for _, f := range d.Sync.Folders {
				mountValue := filepath.Join("/", d.Name, f.RemotePath)
				if runtime.GOOS == "windows" {
					mountValue = filepath.ToSlash(mountValue)
				}
				dev.Sync.Folders = append(dev.Sync.Folders, model.SyncFolder{
					LocalPath:  f.LocalPath,
					RemotePath: mountValue,
				})
				f.LocalPath = mountValue
			}
			dev.Services = append(dev.Services, d)
		}
	} else {
		for dName, d := range manifest.Dev {
			for _, forward := range d.Forward {
				localPort := forward.Local
				if !model.IsPortAvailable(dev.Interface, forward.Local) {
					return nil, fmt.Errorf("local port %d is already in-use in your local machine", forward.Local)
				}
				dev.Forward = append(dev.Forward, model.Forward{
					Local:       localPort,
					Remote:      forward.Remote,
					ServiceName: d.Name,
					Service:     true,
				})
			}
			if len(devs) > 0 && !isInDevs(dName, devs) {
				continue
			}
			dev.Services = append(dev.Services, d)
			for _, f := range d.Sync.Folders {
				mountValue := filepath.Join("/", d.Name, f.RemotePath)
				if runtime.GOOS == "windows" {
					mountValue = filepath.ToSlash(mountValue)
				}
				dev.Sync.Folders = append(dev.Sync.Folders, model.SyncFolder{
					LocalPath:  f.LocalPath,
					RemotePath: mountValue,
				})
				f.LocalPath = mountValue
			}

		}
	}
	if err := dev.SetDefaults(); err != nil {
		return nil, err
	}
	for _, d := range dev.Services {
		if err := d.SetDefaults(); err != nil {
			return nil, err
		}
	}
	dev.Name = detachModePodName
	dev.Image = &model.BuildInfo{Name: "busybox"}
	dev.Namespace = okteto.Context().Namespace
	dev.Context = okteto.Context().Name

	return dev, nil
}

func isInDevs(svc string, devs []string) bool {
	for _, d := range devs {
		if svc == d {
			return true
		}
	}
	return false
}

//AskYesNo prompts for yes/no confirmation
func AskYesNo(q string) (bool, error) {
	var answer string
	for {
		oktetoLog.Question(q)
		if _, err := fmt.Scanln(&answer); err != nil {
			return false, err
		}

		if answer == "y" || answer == "n" {
			break
		}

		oktetoLog.Fail("input must be 'y' or 'n'")
	}

	return answer == "y", nil
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

//AskIfOktetoInit asks if okteto init should be executed
func AskIfOktetoInit(devPath string) bool {
	result, err := AskYesNo(fmt.Sprintf("okteto manifest (%s) doesn't exist, do you want to create it? [y/n] ", devPath))
	if err != nil {
		return false
	}
	return result
}

// AsksQuestion asks a question to the user
func AsksQuestion(q string) (string, error) {
	var answer string

	oktetoLog.Question(q)
	if _, err := fmt.Scanln(&answer); err != nil {
		return "", err
	}

	return answer, nil
}

//AskIfDeploy asks if a new deployment must be created
func AskIfDeploy(name, namespace string) error {
	deploy, err := AskYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one? [y/n]: ", name, namespace))
	if err != nil {
		return fmt.Errorf("couldn't read your response")
	}
	if !deploy {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("deployment %s doesn't exist in namespace %s", name, namespace),
			Hint: "Launch your application first or use 'okteto namespace' to select a different namespace and try again",
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
		oktetoLog.Infof("error on CheckIfDirectory: %s", err.Error())
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
	if DefaultManifest != devPath {
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
	if dev.Divert != nil {
		dev.Name = model.DivertName(dev.Name, okteto.GetSanitizedUsername())
		return app.Divert(okteto.GetSanitizedUsername()), false, nil
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

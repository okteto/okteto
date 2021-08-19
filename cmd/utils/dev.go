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
	"strings"

	"github.com/joho/godotenv"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	//DefaultDevManifest default okteto manifest file
	DefaultDevManifest   = "okteto.yml"
	secondaryDevManifest = "okteto.yaml"
)

//LoadDev loads an okteto manifest checking "yml" and "yaml"
func LoadDev(devPath, namespace, k8sContext string) (*model.Dev, error) {
	if !model.FileExists(devPath) {
		if devPath == DefaultDevManifest {
			if model.FileExists(secondaryDevManifest) {
				return LoadDev(secondaryDevManifest, namespace, k8sContext)
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
	loadContext(dev, k8sContext)
	loadNamespace(dev, namespace)
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

func loadContext(dev *model.Dev, k8sContext string) {
	if k8sContext != "" {
		dev.Context = k8sContext
		return
	}
	if dev.Context != "" {
		return
	}
	if os.Getenv(client.OktetoContextVariableName) != "" {
		dev.Context = os.Getenv(client.OktetoContextVariableName)
		return
	}
	dev.Context = client.GetSessionContext("")
}

func loadNamespace(dev *model.Dev, namespace string) {
	if namespace != "" {
		dev.Namespace = namespace
	}
	if dev.Namespace == "" {
		dev.Namespace = client.GetContextNamespace(dev.Context)
	}
}

//LoadDevOrDefault loads an okteto manifest or a default one if does not exist
func LoadDevOrDefault(devPath, name, namespace, k8sContext string) (*model.Dev, error) {
	dev, err := LoadDev(devPath, namespace, k8sContext)
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
			E:    fmt.Errorf("Deployment %s doesn't exist in namespace %s", name, namespace),
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

//LoadEnvironment taking into account .env files and Okteto Secrets
func LoadEnvironment(ctx context.Context, getSecrets bool) error {
	if model.FileExists(".env") {
		err := godotenv.Load()
		if err != nil {
			log.Errorf("error loading .env file: %s", err.Error())
		}
	}

	if !getSecrets {
		return nil
	}

	currentContext := client.GetSessionContext("")
	if okteto.GetClusterContext() == currentContext {
		secrets, err := okteto.GetSecrets(ctx)
		if err != nil {
			return fmt.Errorf("error loading Okteto Secrets: %s", err.Error())
		}

		currentEnv := map[string]bool{}
		rawEnv := os.Environ()
		for _, rawEnvLine := range rawEnv {
			key := strings.Split(rawEnvLine, "=")[0]
			currentEnv[key] = true
		}

		for _, secret := range secrets {
			if strings.HasPrefix(secret.Name, "github.") {
				continue
			}
			if !currentEnv[secret.Name] {
				os.Setenv(secret.Name, secret.Value)
			}
		}
	}

	return nil
}

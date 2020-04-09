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

package utils

import (
	"fmt"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

const (
	//DefaultDevManifest default okteto manifest file
	DefaultDevManifest   = "okteto.yml"
	secondaryDevManifest = "okteto.yaml"
)

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

	return model.Get(devPath)
}

//LoadDevOrDefault loads an okteto manifest or a default one if does not exist
func LoadDevOrDefault(devPath, name string) (*model.Dev, error) {
	dev, err := LoadDev(devPath)
	if err == nil {
		return dev, nil
	}

	if errors.IsNotExist(err) && len(name) > 0 {
		return &model.Dev{
			Name:   name,
			Labels: map[string]string{},
		}, nil
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

//AskIfDeploy asks if a new deployment must be created
func AskIfDeploy(name, namespace string) error {
	deploy, err := AskYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one? [y/n]: ", name, namespace))
	if err != nil {
		return fmt.Errorf("couldn't read your response")
	}
	if !deploy {
		return errors.UserError{
			E:    fmt.Errorf("Deployment %s doesn't exist in namespace %s", name, namespace),
			Hint: "Deploy your application first or use `okteto namespace` to select a different namespace and try again",
		}
	}
	return nil
}

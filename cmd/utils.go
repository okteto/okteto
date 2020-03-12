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
	"runtime"
	"strconv"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	yaml "gopkg.in/yaml.v2"
)

func loadDev(devPath string) (*model.Dev, error) {
	if !model.FileExists(devPath) {
		if devPath == defaultManifest {
			if model.FileExists(secondaryManifest) {
				return loadDev(secondaryManifest)
			}
		}

		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}

	return model.Get(devPath)
}

func loadDevOrDefault(devPath, name string) (*model.Dev, error) {
	dev, err := loadDev(devPath)
	if err == nil {
		return dev, nil
	}

	if errors.IsNotExists(err) && len(name) > 0 {
		return &model.Dev{
			Name:   name,
			Labels: map[string]string{},
		}, nil
	}

	return nil, err
}

func askYesNo(q string) (bool, error) {
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

func askIfDeploy(name, namespace string) error {
	deploy, err := askYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one? [y/n]: ", name, namespace))
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

func checkLocalWatchesConfiguration() {
	if runtime.GOOS != "linux" {
		return
	}

	w := "/proc/sys/fs/inotify/max_user_watches"
	f, err := ioutil.ReadFile(w)
	if err != nil {
		log.Infof("Fail to read %s: %s", w, err)
		return
	}

	if isWatchesConfigurationTooLow(string(f)) {
		log.Yellow("The value of /proc/sys/fs/inotify/max_user_watches is too low.")
		log.Yellow("This can affect Okteto's file synchronization performance.")
		log.Yellow("We recommend you to raise it to at least 524288 to ensure proper performance.")
		fmt.Println()
	}
}

func isWatchesConfigurationTooLow(value string) bool {
	value = strings.TrimSuffix(string(value), "\n")
	c, err := strconv.Atoi(value)
	if err != nil {
		log.Infof("Fail to parse the value of max_user_watches: %s", err)
		return false
	}
	log.Debugf("max_user_watches = %d", c)
	return c <= 8192
}

func saveManifest(dev *model.Dev, path string) error {
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Infof("failed to marshall dev environment: %s", err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := ioutil.WriteFile(path, marshalled, 0600); err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to write your manifest")
	}

	return nil
}

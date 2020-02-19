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
	if !FileExists(devPath) {
		if devPath == DefaultDevManifest {
			if FileExists(secondaryDevManifest) {
				return LoadDev(secondaryDevManifest)
			}
		}

		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}

	return model.Get(devPath)
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

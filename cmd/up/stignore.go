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

package up

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/linguist"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

func checkStignoreConfiguration(dev *model.Dev) error {
	for _, folder := range dev.Syncs {
		stignorePath := filepath.Join(folder.LocalPath, ".stignore")
		gitPath := filepath.Join(folder.LocalPath, ".git")
		if !model.FileExists(stignorePath) {
			log.Infof("'.stignore' does not exist in folder '%s'", folder.LocalPath)
			if err := askIfCreatingStignore(folder.LocalPath, stignorePath, gitPath); err != nil {
				return err
			}
			continue
		}

		log.Infof("'.stignore' exists in folder '%s'", folder.LocalPath)
		if !model.FileExists(gitPath) {
			continue
		}

		if err := askIfUpdatingStignore(folder.LocalPath, stignorePath, gitPath); err != nil {
			return err
		}
	}
	return nil
}

func askIfCreatingStignore(folder, stignorePath, gitPath string) error {
	log.Information("It is recommended to define a '.stignore' file to avoid synchronizing things like build artifacts, git metadata or third party libraries.")
	answer, err := utils.AskYesNo("    Do you want to automatically create a '.stignore' file? [y/n] ")
	if err != nil {
		return fmt.Errorf("failed to ask for adding '.stignore' to '%s': %s", folder, err.Error())
	}

	if !answer {
		stignoreContent := ""
		if model.FileExists(gitPath) {
			stignoreContent = ".git\n"
		}
		if err := ioutil.WriteFile(stignorePath, []byte(stignoreContent), 0644); err != nil {
			return fmt.Errorf("failed to create empty '%s': %s", stignorePath, err.Error())
		}
		return nil
	}

	language, err := initCMD.GetLanguage("", folder)
	if err != nil {
		return fmt.Errorf("failed to get language for '%s': %s", folder, err.Error())
	}
	c := linguist.GetSTIgnore(language)
	if err := ioutil.WriteFile(stignorePath, c, 0600); err != nil {
		return fmt.Errorf("failed to write stignore file for '%s': %s", folder, err.Error())
	}
	return nil
}

func askIfUpdatingStignore(folder, stignorePath, gitPath string) error {
	stignoreBytes, err := ioutil.ReadFile(stignorePath)
	if err != nil {
		return fmt.Errorf("failed to read '%s': %s", stignorePath, err.Error())
	}
	stignoreContent := string(stignoreBytes)
	if strings.Contains(stignoreContent, ".git") {
		return nil
	}

	log.Information("It is recommended to not synchronize the '%s' folder.", gitPath)
	answer, err := utils.AskYesNo("    Do you want to ignore it? [y/n] ")
	if err != nil {
		return fmt.Errorf("failed to ask for adding '.git' to '%s': %s", stignorePath, err.Error())
	}
	if !answer {
		return nil
	}

	log.Infof("adding '.git' to '%s'", stignorePath)
	stignoreContent = fmt.Sprintf(".git\n%s", stignoreContent)
	if err := ioutil.WriteFile(stignorePath, []byte(stignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to update '%s': %s", stignorePath, err.Error())
	}
	return nil
}

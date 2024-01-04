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

package up

import (
	"bufio"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/cmd/manifest"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/linguist"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

func addStignoreSecrets(dev *model.Dev) error {
	output := ""
	for i, folder := range dev.Sync.Folders {
		stignorePath := filepath.Join(folder.LocalPath, ".stignore")
		if !filesystem.FileExists(stignorePath) {
			continue
		}
		infile, err := os.Open(stignorePath)
		if err != nil {
			return oktetoErrors.UserError{
				E:    err,
				Hint: "Update the 'sync' field of your okteto manifest to point to a valid directory path",
			}
		}
		defer func() {
			if err := infile.Close(); err != nil {
				oktetoLog.Debugf("Error closing file %s: %s", stignorePath, err)
			}
		}()
		reader := bufio.NewReader(infile)

		stignoreName := fmt.Sprintf(".stignore-%d", i+1)
		transformedStignorePath := filepath.Join(config.GetAppHome(dev.Namespace, dev.Name), stignoreName)
		outfile, err := os.OpenFile(transformedStignorePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		defer func() {
			if err := outfile.Close(); err != nil {
				oktetoLog.Infof("Error closing file %s: %s", transformedStignorePath, err)
			}
		}()

		writer := bufio.NewWriter(outfile)
		defer writer.Flush()

		for {
			bytes, _, err := reader.ReadLine()
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}

			line := strings.TrimSpace(string(bytes))
			// ignore local lines that are empty, comments or includes more files
			// TODO: support remote #include https://github.com/okteto/okteto/issues/2832
			if strings.Compare(line, "") == 0 || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
				continue
			}

			// transform line by adding (?d) unless the line starts with ! or already has (?d)
			if !strings.HasPrefix(line, "!") && !strings.Contains(line, "(?d)") {
				line = fmt.Sprintf("(?d)%s", line)
			}

			// write new line at remote .stignore
			_, err = writer.WriteString(fmt.Sprintf("%s\n", line))
			if err != nil {
				return err
			}
			output = fmt.Sprintf("%s\n%s", output, line)
		}

		dev.Secrets = append(
			dev.Secrets,
			model.Secret{
				LocalPath:  transformedStignorePath,
				RemotePath: path.Join(folder.RemotePath, ".stignore"),
				Mode:       0644,
			},
		)
	}
	dev.Metadata.Annotations[model.OktetoStignoreAnnotation] = fmt.Sprintf("%x", sha512.Sum512([]byte(output)))
	return nil
}

func addSyncFieldHash(dev *model.Dev) error {
	output, err := json.Marshal(dev.Sync)
	if err != nil {
		return err
	}
	dev.Metadata.Annotations[model.OktetoSyncAnnotation] = fmt.Sprintf("%x", sha512.Sum512(output))
	return nil
}

func checkStignoreConfiguration(dev *model.Dev) error {
	if dev.IsHybridModeEnabled() {
		return nil
	}

	for _, folder := range dev.Sync.Folders {
		stignorePath := filepath.Join(folder.LocalPath, ".stignore")
		gitPath := filepath.Join(folder.LocalPath, ".git")
		if !filesystem.FileExists(stignorePath) {
			if err := askIfCreateStignoreDefaults(folder.LocalPath, stignorePath); err != nil {
				return err
			}
			continue
		}

		oktetoLog.Infof("'.stignore' exists in folder '%s'", folder.LocalPath)
		if !filesystem.FileExists(gitPath) {
			continue
		}

		if err := checkIfStignoreHasGitFolder(stignorePath); err != nil {
			return err
		}
	}
	return nil
}

func askIfCreateStignoreDefaults(folder, stignorePath string) error {
	autogenerateStignore := env.LoadBoolean(model.OktetoAutogenerateStignoreEnvVar)

	oktetoLog.Information("'.stignore' doesn't exist in folder '%s'.", folder)

	if autogenerateStignore {
		l, err := linguist.ProcessDirectory(stignorePath)
		if err != nil {
			oktetoLog.Infof("failed to process directory: %s", err)
			l = linguist.Unrecognized
		}
		c := linguist.GetSTIgnore(l)
		if err := os.WriteFile(stignorePath, c, 0600); err != nil {
			return fmt.Errorf("failed to write stignore file for '%s': %w", folder, err)
		}
		return nil
	}

	oktetoLog.Information("Okteto requires a '.stignore' file to ignore file patterns that help optimize the synchronization service.")
	stignoreDefaults, err := utils.AskYesNo("Do you want to infer defaults for the '.stignore' file? (otherwise, it will be left blank)", utils.YesNoDefault_Yes)
	if err != nil {
		return fmt.Errorf("failed to add '.stignore' to '%s': %w", folder, err)
	}

	if !stignoreDefaults {
		stignoreContent := ""
		if err := os.WriteFile(stignorePath, []byte(stignoreContent), 0600); err != nil {
			return fmt.Errorf("failed to create empty '%s': %w", stignorePath, err)
		}
		return nil
	}

	language, err := manifest.GetLanguage("", folder)
	if err != nil {
		return fmt.Errorf("failed to get language for '%s': %w", folder, err)
	}
	c := linguist.GetSTIgnore(language)
	if err := os.WriteFile(stignorePath, c, 0600); err != nil {
		return fmt.Errorf("failed to write stignore file for '%s': %w", folder, err)
	}
	return nil
}

func checkIfStignoreHasGitFolder(stignorePath string) error {
	stignoreBytes, err := os.ReadFile(stignorePath)
	if err != nil {
		return fmt.Errorf("failed to read '%s': %w", stignorePath, err)
	}
	stignoreContent := string(stignoreBytes)
	if strings.Contains(stignoreContent, ".git") {
		return nil
	}

	oktetoLog.Warning("The synchronization service performance could be degraded if the '.git' folder is synchronized. Please add '.git' to the '.stignore' file.")
	return nil
}

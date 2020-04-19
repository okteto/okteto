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
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/linguist"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	stignore          = ".stignore"
	defaultManifest   = "okteto.yml"
	secondaryManifest = "okteto.yaml"
)

var wrongImageNames = map[string]bool{
	"T":     true,
	"TRUE":  true,
	"Y":     true,
	"YES":   true,
	"F":     true,
	"FALSE": true,
	"N":     true,
	"NO":    true,
}

//Init automatically generates the manifest
func Init() *cobra.Command {
	var devPath string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Automatically generates your okteto manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := os.Getenv("OKTETO_LANGUAGE")
			workDir, err := os.Getwd()
			if err != nil {
				return err
			}

			if err := executeInit(devPath, overwrite, l, workDir); err != nil {
				return err
			}

			log.Success(fmt.Sprintf("Okteto manifest (%s) created", devPath))
			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&overwrite, "overwrite", "o", false, "overwrite existing manifest file")
	return cmd
}

func executeInit(devPath string, overwrite bool, language string, workDir string) error {
	if !overwrite {
		if model.FileExists(devPath) {
			return fmt.Errorf("%s already exists. Please delete it before running the command again", devPath)
		}
	}

	var dev *model.Dev
	if len(language) > 0 {
		log.Debugf("generating config for %s", language)
		dev = linguist.GetDevConfig(language)
	} else {
		l, err := linguist.ProcessDirectory(workDir)
		if err != nil {
			log.Info(err)
			return fmt.Errorf("Failed to determine the language of the current directory")
		}

		language = l
		if language == linguist.Unrecognized {
			l, err := askForLanguage()
			if err != nil {
				return err
			}

			language = l
		}

		dev = linguist.GetDevConfig(language)
		dev.Image = askForImage(language, dev.Image)
	}
	var err error
	dev.Name, err = model.GetValidNameFromFolder(workDir)
	if err != nil {
		return err
	}

	if err := dev.Save(devPath); err != nil {
		return err
	}

	if !model.FileExists(stignore) {
		log.Debugf("getting stignore for %s", language)
		c := linguist.GetSTIgnore(language)
		if err := ioutil.WriteFile(stignore, c, 0600); err != nil {
			log.Infof("failed to write stignore file: %s", err)
		}
	}

	analytics.TrackInit(true)
	return nil
}

func askForImage(language, defaultImage string) string {
	var image string
	fmt.Printf("Recommended image for development with %s: %s\n", language, log.BlueString(defaultImage))
	fmt.Printf("Which docker image do you want to use for your development environment? [%s]: ", defaultImage)
	_, err := fmt.Scanln(&image)
	fmt.Println()

	if err != nil {
		log.Debugf("Scanln failed to read dev environment image: %s", err)
		image = ""
	}

	if image == "" {
		return defaultImage
	}

	if _, ok := wrongImageNames[strings.ToUpper(image)]; ok {
		log.Yellow("Ignoring docker image name: '%s', will use '%s' instead", image, defaultImage)
		image = defaultImage
	}

	return image
}

func askForLanguage() (string, error) {
	supportedLanguages := linguist.GetSupportedLanguages()

	prompt := promptui.Select{
		Label: "Couldn't detect any language in current folder. Pick your project's main language from the list below",
		Items: supportedLanguages,
		Size:  len(supportedLanguages),
		Templates: &promptui.SelectTemplates{
			Label:    fmt.Sprintf("%s {{ . }}", log.BlueString("?")),
			Selected: " ✓  {{ . | oktetoblue }}",
			Active:   fmt.Sprintf("%s {{ . | oktetoblue }}", promptui.IconSelect),
			Inactive: "  {{ . | oktetoblue }}",
			FuncMap:  promptui.FuncMap,
		},
	}

	prompt.Templates.FuncMap["oktetoblue"] = log.BlueString

	i, _, err := prompt.Run()
	if err != nil {
		log.Debugf("invalid init option: %s", err)
		return "", fmt.Errorf("invalid option")
	}

	return supportedLanguages[i], nil
}

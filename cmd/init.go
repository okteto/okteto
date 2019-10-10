package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/linguist"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"

	yaml "gopkg.in/yaml.v2"

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
			err := executeInit(devPath, overwrite)
			if err == nil {
				log.Success(fmt.Sprintf("Okteto manifest (%s) created", devPath))
				return nil
			}

			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&overwrite, "overwrite", "o", false, "overwrite existing manifest file")
	return cmd
}

func executeInit(devPath string, overwrite bool) error {
	if !overwrite {
		if fileExists(devPath) {
			return fmt.Errorf("%s already exists. Please delete it before running the command again", devPath)
		}
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	var dev *model.Dev
	var language string
	if l, ok := os.LookupEnv("OKTETO_LANGUAGE"); ok {
		log.Debugf("generating config for %s", l)
		dev = linguist.GetDevConfig(l)
		language = l
	} else {
		l, err := linguist.ProcessDirectory(root)
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

	dev.Name = getDeploymentName(filepath.Base(root))

	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Infof("failed to marshall dev environment: %s", err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := ioutil.WriteFile(devPath, marshalled, 0600); err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to write your manifest")
	}

	if !fileExists(stignore) {
		log.Debugf("getting stignore for %s", language)
		c := linguist.GetSTIgnore(language)
		if err := ioutil.WriteFile(stignore, c, 0600); err != nil {
			log.Infof("failed to write stignore file: %s", err)
		}
	}

	analytics.TrackInit(language, dev.Image, config.VersionString, true)
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

func getDeploymentName(name string) string {
	deploymentName := filepath.Base(name)
	deploymentName = strings.ToLower(deploymentName)
	deploymentName = model.ValidKubeNameRegex.ReplaceAllString(deploymentName, "-")
	log.Infof("deployment name: %s", deploymentName)
	return deploymentName
}

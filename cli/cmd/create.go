package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"cli/cnd/pkg/linguist"
	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"
	"cli/cnd/pkg/utils"

	yaml "gopkg.in/yaml.v2"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	stignore = ".stignore"
)

const kubectlManifest = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: {{ .Name }}
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: {{ .Name }}
    spec:
      terminationGracePeriodSeconds: 0
      containers:
      - image: {{ .Image }}
        name: {{ .Name }}
        command: 
        - tail
        - -f
        - /dev/null
`

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

var validKubeNameRegex = regexp.MustCompile("[^a-zA-Z0-9/.-]+")

//Create automatically generates the manifest
func Create() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Automatically create your cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := executeCreate(devPath)
			if err == nil {
				fmt.Printf("%s %s", log.SuccessSymbol, log.GreenString("Cloud native environment created"))
				fmt.Println()
				return nil
			}

			return err
		},
	}

	addDevPathFlag(cmd, &devPath)
	return cmd
}

func executeCreate(devPath string) error {
	if fileExists(devPath) {
		return fmt.Errorf("%s already exists. Please delete it before running the command again", devPath)
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	languagesDiscovered, err := linguist.ProcessDirectory(root)
	if err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to determine the language of the current directory")
	}

	dev, language, err := getDevelopmentEnvironment(languagesDiscovered[0])
	if err != nil {
		return err
	}

	dev.Name = getDeploymentName(filepath.Base(root))
	dev.WorkDir.Source = ""
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := ioutil.WriteFile(devPath, marshalled, 0600); err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if !fileExists(stignore) {
		log.Debugf("getting stignore for %s", language)
		c := linguist.GetSTIgnore(language)
		if err := ioutil.WriteFile(stignore, c, 0600); err != nil {
			log.Infof("failed to write stignore: %s", err)
		}
	}

	if err := generateKubectlManifest(dev); err != nil {
		return err
	}

	return nil
}

func getDevelopmentEnvironment(language string) (*model.Dev, string, error) {
	var env string
	var dev *model.Dev

	if language == linguist.Unrecognized {
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
			log.Debugf("invalid create option: %s", err)
			return nil, "", fmt.Errorf("invalid option")
		}

		language = supportedLanguages[i]

		dev = linguist.GetDevConfig(language)
		fmt.Printf("Recommended image for development with %s: %s", language, log.BlueString(dev.Image))
	} else {
		dev = linguist.GetDevConfig(language)
		fmt.Printf("%s detected in your source. Recommended image for development: %s", language, log.BlueString(dev.Image))
	}

	fmt.Println()
	fmt.Printf("Which docker image do you want to use for your development environment? [%s]: ", dev.Image)
	_, err := fmt.Scanln(&env)
	fmt.Println()
	if err != nil {
		log.Debugf("Scanln failed to read dev environment image: %s", err)
		env = ""
	}
	if env != "" {
		if _, ok := wrongImageNames[strings.ToUpper(env)]; ok {
			log.Yellow("Ignoring wrong docker image name: %s", env)
		} else {
			dev.Image = env
		}
	}

	return dev, language, nil
}

func generateKubectlManifest(dev *model.Dev) error {
	path := "deployment.yaml"
	if fileExists(path) {
		return nil
	}

	create := utils.AskYesNo("Create a Kubernetes deployment manifest? [y/n]: ")
	if !create {
		return nil
	}

	t := template.Must(template.New("kubectlManifest").Parse(kubectlManifest))
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Failed to generate your Kubernetes deployment manifest")
	}

	if err := t.Execute(f, dev); err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to generate your Kubernetes deployment manifest: %s", err)
	}

	return nil
}

func fileExists(name string) bool {

	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		log.Infof("Failed to check if %s exists: %s", name, err)
	}

	return true

}

func getDeploymentName(name string) string {
	deploymentName := filepath.Base(name)
	deploymentName = strings.ToLower(deploymentName)
	deploymentName = validKubeNameRegex.ReplaceAllString(deploymentName, "")
	return deploymentName
}

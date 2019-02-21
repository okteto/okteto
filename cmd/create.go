package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/linguist"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	yaml "gopkg.in/yaml.v2"

	"regexp"

	"github.com/spf13/cobra"
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

	addDevPathFlag(cmd, &devPath, config.CNDManifestFileName())
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

	dev := linguist.GetDevConfig(languagesDiscovered[0])
	dev.Swap.Deployment.Name = getDeploymentName(root)

	var env string
	if languagesDiscovered[0] == "unrecognized" {
		fmt.Printf("Couldn't detect any language in your source. Recommended image for development: %s", log.BlueString(dev.Swap.Deployment.Image))
	} else {
		fmt.Printf("%s detected in your source. Recommended image for development: %s", languagesDiscovered[0], log.BlueString(dev.Swap.Deployment.Image))
	}
	fmt.Println()
	fmt.Printf("Which docker image do you want to use for your development environment? [%s]: ", dev.Swap.Deployment.Image)
	fmt.Scanln(&env)

	if env != "" {
		dev.Swap.Deployment.Image = env
	}

	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := ioutil.WriteFile(devPath, marshalled, 0600); err != nil {
		log.Info(err)
		return fmt.Errorf("Failed to generate your manifest")
	}

	var kubectl string
	for {
		fmt.Printf("Create a Kubernetes deployment manifest? [y/n]: ")
		fmt.Scanln(&kubectl)
		if kubectl == "y" || kubectl == "n" {
			break

		}

		fmt.Println(log.RedString("input must be y or n"))
	}

	if kubectl == "y" {
		return generateKubectlManifest(dev)
	}

	return nil
}

func generateKubectlManifest(dev *model.Dev) error {
	path := "deployment.yaml"
	if fileExists(path) {
		return fmt.Errorf("%s already exists. Please delete it before running the command again", path)
	}
	data := struct {
		Name  string
		Image string
	}{
		Name:  dev.Swap.Deployment.Name,
		Image: dev.Swap.Deployment.Image,
	}

	t := template.Must(template.New("kubectlManifest").Parse(kubectlManifest))
	f, err := os.Create("deployment.yaml")
	if err != nil {
		return fmt.Errorf("Failed to generate your Kubernetes deployment manifest")
	}

	if err := t.Execute(f, data); err != nil {
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

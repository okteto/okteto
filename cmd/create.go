package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/okteto/cnd/pkg/linguist"
	yaml "gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//Create automatically generates the manifest
func Create() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Automatically create the cnd manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeCreate()
		},
	}

	return cmd
}

func executeCreate() error {
	if fileExists(c.devPath) {
		return fmt.Errorf("%s already exists. Please delete it before running the command again", c.devPath)
	}

	root, err := os.Getwd()
	if err != nil {
		return err
	}

	languagesDiscovered, err := linguist.ProcessDirectory(root)
	if err != nil {
		log.Error(err)
		return fmt.Errorf("Failed to determine the language of the current directory")
	}

	dev := linguist.GetDevConfig(languagesDiscovered[0])
	dev.Swap.Deployment.Name = path.Base(root)
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		log.Error(err)
		return fmt.Errorf("Failed to generate your cnd manifest")
	}

	if err := ioutil.WriteFile(c.devPath, marshalled, 0600); err != nil {
		log.Error(err)
		return fmt.Errorf("Failed to generate your cnd manifest")
	}

	return nil
}

func fileExists(name string) bool {

	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}

	if err != nil {
		log.Infof("Failed to check if %s exists: %s", c.devPath, err)
	}

	return true

}

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	randomdata "github.com/Pallinder/go-randomdata"
	"github.com/okteto/app/cli/pkg/linguist"
	"github.com/okteto/app/cli/pkg/log"
	yaml "gopkg.in/yaml.v2"
)

func createManifest(devPath string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	languagesDiscovered, err := linguist.ProcessDirectory(root)
	if err != nil {
		return fmt.Errorf("Failed to determine the language of the current directory")
	}

	dev := linguist.GetDevConfig(languagesDiscovered[0])
	dev.Name = fmt.Sprintf("%s-%s", randomdata.Adjective(), randomdata.Noun())
	marshalled, err := yaml.Marshal(dev)
	if err != nil {
		return fmt.Errorf("Failed to generate your manifest")
	}

	if err := ioutil.WriteFile(devPath, marshalled, 0600); err != nil {
		return fmt.Errorf("Failed to generate your manifest")
	}

	c := linguist.GetSTIgnore(languagesDiscovered[0])
	if err := ioutil.WriteFile(filepath.Join(root, ".stignore"), c, 0600); err != nil {
		return fmt.Errorf("Failed to generate your manifest")
	}

	fmt.Printf("%s %s\n", log.InformationSymbol, log.BlueString("%s automatically generated", filepath.Base(devPath)))
	return nil
}

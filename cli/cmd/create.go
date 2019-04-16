package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	haikunator "github.com/Atrox/haikunatorgo"
	"github.com/okteto/app/cli/pkg/linguist"
	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/okteto"
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
	n, err := getName()

	if err != nil {
		return err
	}

	dev.Name = n

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

	log.Information("%s automatically generated", filepath.Base(devPath))
	return nil
}

func getName() (string, error) {
	haikunator := haikunator.New()
	haikunator.TokenLength = 0

	envs, err := okteto.GetDevEnvironments()
	if err != nil {
		return "", err
	}

	for i := 0; i < 20; i++ {
		n := haikunator.Haikunate()
		for _, e := range envs {
			if strings.HasPrefix(e.Name, n) {
				log.Debugf("%s already exists, generating new name: %+v", n, e)
				continue
			}
		}

		return n, nil
	}

	return "", fmt.Errorf("failed to generate unique name")
}

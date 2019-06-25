package cmd

import (
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

func loadDev(devPath string) (*model.Dev, error) {
	if !fileExists(devPath) {
		if devPath == defaultManifest {
			if fileExists(secondaryManifest) {
				return loadDev(secondaryManifest)
			}
		}

		return nil, fmt.Errorf("'%s' does not exist. Generate it by executing 'okteto init'", devPath)
	}

	return model.Get(devPath)
}

func askYesNo(q string) bool {
	var answer string
	for {
		fmt.Printf(q)
		fmt.Scanln(&answer)
		if answer == "y" || answer == "n" {
			break
		}

		log.Fail("input must be 'y' or 'n'")
	}

	if answer == "n" {
		return false
	}

	return true
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

package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/log"
)

//GetWarningState returns the value associated to a given warning
func GetWarningState(path, name string) string {
	filePath := filepath.Join(path, name)
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Infof("failed to read warning file '%s': %s", filePath, err)
		return ""
	}

	return string(bytes)
}

//SetWarningState sets the value associated to a given warning
func SetWarningState(path, name, value string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	filePath := filepath.Join(path, name)

	if err := ioutil.WriteFile(filePath, []byte(value), 0644); err != nil {
		return err
	}

	return nil
}

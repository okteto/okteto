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

package build

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/pkg/errors"
)

const (
	defaultDockerIgnore = ".dockerignore"
)

// GetDockerfile returns the dockerfile with the cache and registry translations
func GetDockerfile(dockerFile string, okCtx OktetoContextInterface) (string, error) {
	file, err := getTranslatedDockerFile(dockerFile, okCtx)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary build folder")
	}

	return file, nil
}

func getTranslatedDockerFile(filename string, okCtx OktetoContextInterface) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := file.Close(); err != nil {
			oktetoLog.Debugf("Error closing file %s: %s", filename, err)
		}
	}()

	scanner := bufio.NewScanner(file)

	dockerfileTmpFolder := filepath.Join(config.GetOktetoHome(), ".dockerfile")
	if err := os.MkdirAll(dockerfileTmpFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create %s: %w", dockerfileTmpFolder, err)
	}

	tmpFile, err := os.CreateTemp(dockerfileTmpFolder, "buildkit-")
	if err != nil {
		return "", err
	}

	datawriter := bufio.NewWriter(tmpFile)
	defer datawriter.Flush()

	for scanner.Scan() {
		line := scanner.Text()
		translatedLine := translateOktetoRegistryImage(line, okCtx)
		_, err = datawriter.WriteString(translatedLine + "\n")
		if err != nil {
			return "", fmt.Errorf("failed to write dockerfile: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	if err := copyDockerIgnore(filename, tmpFile.Name()); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func translateOktetoRegistryImage(input string, okCtx OktetoContextInterface) string {
	replacer := registry.NewRegistryReplacer(GetRegistryConfigFromOktetoConfig(okCtx))
	if strings.Contains(input, constants.DevRegistry) {
		tag := replacer.Replace(input, constants.DevRegistry, okCtx.GetCurrentNamespace())
		return tag
	}

	if strings.Contains(input, constants.GlobalRegistry) {
		globalNamespace := constants.DefaultGlobalNamespace

		ctxGlobalNamespace := okCtx.GetGlobalNamespace()
		if ctxGlobalNamespace != "" {
			globalNamespace = ctxGlobalNamespace
		}
		tag := replacer.Replace(input, constants.GlobalRegistry, globalNamespace)
		return tag
	}

	return input

}

func copyDockerIgnore(originalPath, translatedPath string) error {
	originalPath, err := filepath.Abs(originalPath)
	if err != nil {
		oktetoLog.Infof("could not load original dockerfile path")
		return err
	}
	originalDir := filepath.Dir(originalPath)
	originalName := filepath.Base(originalPath)
	originalDockerIgnore := filepath.Join(originalDir, fmt.Sprintf("%s%s", originalName, defaultDockerIgnore))

	translatedDir := filepath.Dir(translatedPath)
	translatedName := filepath.Base(translatedPath)
	newPath := filepath.Join(translatedDir, fmt.Sprintf("%s%s", translatedName, defaultDockerIgnore))
	if fs, err := os.Stat(originalDockerIgnore); err == nil && !fs.IsDir() {
		err := copyFile(originalDockerIgnore, newPath)
		if err != nil {
			return err
		}
	} else {
		originalDockerIgnore := filepath.Join(originalDir, defaultDockerIgnore)
		if fs, err := os.Stat(originalDockerIgnore); err == nil && !fs.IsDir() {
			err := copyFile(originalDockerIgnore, newPath)
			if err != nil {
				return err
			}
		} else {
			oktetoLog.Infof("could not detect any .dockerignore on %s", originalDir)
		}
	}
	return nil
}

func copyFile(orig, dest string) error {
	input, err := os.ReadFile(orig)
	if err != nil {
		oktetoLog.Infof("could not read %s dockerfile", orig)
		return err
	}

	err = os.WriteFile(dest, input, 0600)
	if err != nil {
		oktetoLog.Infof("error creating %s: %s", dest, err)
		return err
	}
	return nil
}

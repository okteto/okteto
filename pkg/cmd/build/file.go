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
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
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

	userId := okCtx.GetCurrentUser()
	if userId == "" {
		userId = "anonymous"
	}

	withCacheHandler := okCtx.GetCurrentBuilder() == okteto.CloudBuildKitURL

	for scanner.Scan() {
		line := scanner.Text()
		translatedLine := translateOktetoRegistryImage(line, okCtx)
		if withCacheHandler {
			translatedLine = translateCacheHandler(translatedLine, userId)
		}
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

func translateCacheHandler(input, userID string) string {
	matched, err := regexp.MatchString(`^RUN.*--mount=.*type=cache`, input)
	if err != nil {
		return input
	}

	if matched {
		matched, err = regexp.MatchString(`^RUN.*--mount=id=`, input)
		if err != nil {
			return input
		}
		if matched {
			return strings.ReplaceAll(input, "--mount=id=", fmt.Sprintf("--mount=id=%s-", userID))
		}
		matched, err = regexp.MatchString(`^RUN.*--mount=[^ ]+,id=`, input)
		if err != nil {
			return input
		}
		if matched {
			return strings.ReplaceAll(input, ",id=", fmt.Sprintf(",id=%s-", userID))
		}
		return strings.ReplaceAll(input, "--mount=", fmt.Sprintf("--mount=id=%s,", userID))
	}

	return input
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

// CreateDockerfileWithVolumeMounts creates the Dockerfile with volume mounts and returns the BuildInfo
func CreateDockerfileWithVolumeMounts(image string, volumes []build.VolumeMounts) (*build.Info, error) {
	build := &build.Info{}

	ctx, err := filepath.Abs(".")
	if err != nil {
		return build, nil
	}
	build.Context = ctx
	dockerfileTmpFolder := filepath.Join(config.GetOktetoHome(), ".dockerfile")
	if err := os.MkdirAll(dockerfileTmpFolder, 0700); err != nil {
		return build, fmt.Errorf("failed to create %s: %w", dockerfileTmpFolder, err)
	}

	tmpFile, err := os.CreateTemp(dockerfileTmpFolder, "buildkit-")
	if err != nil {
		return build, err
	}

	datawriter := bufio.NewWriter(tmpFile)
	defer datawriter.Flush()

	_, err = datawriter.Write([]byte(fmt.Sprintf("FROM %s\n", image)))
	if err != nil {
		return build, fmt.Errorf("failed to write dockerfile: %w", err)
	}
	for _, volume := range volumes {
		_, err = datawriter.Write([]byte(fmt.Sprintf("COPY %s %s\n", filepath.ToSlash(volume.LocalPath), volume.RemotePath)))
		if err != nil {
			oktetoLog.Infof("failed to write volume %s: %s", volume.LocalPath, err)
			continue
		}
	}

	build.Dockerfile = tmpFile.Name()
	build.VolumesToInclude = volumes
	return build, nil
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

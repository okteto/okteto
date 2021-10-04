// Copyright 2021 The Okteto Authors
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

package registry

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/pkg/errors"
)

// GetDockerfile returns the dockerfile with the cache and registry translations
func GetDockerfile(dockerFile string) (string, error) {
	file, err := getTranslatedDockerFile(dockerFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary build folder")
	}

	return file, nil
}

func getTranslatedDockerFile(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	dockerfileTmpFolder := filepath.Join(config.GetOktetoHome(), ".dockerfile")
	if err := os.MkdirAll(dockerfileTmpFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create %s: %s", dockerfileTmpFolder, err)
	}

	tmpFile, err := os.CreateTemp(dockerfileTmpFolder, "buildkit-")
	if err != nil {
		return "", err
	}

	datawriter := bufio.NewWriter(tmpFile)
	defer datawriter.Flush()

	userID := okteto.Context().UserID
	if userID == "" {
		userID = "anonymous"
	}

	withCacheHandler := okteto.Context().Buildkit == okteto.CloudBuildKitURL

	for scanner.Scan() {
		line := scanner.Text()
		translatedLine := translateOktetoRegistryImage(line)
		if withCacheHandler {
			translatedLine = translateCacheHandler(translatedLine, userID)
		}
		_, _ = datawriter.WriteString(translatedLine + "\n")
	}
	if err := scanner.Err(); err != nil {
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

func translateOktetoRegistryImage(input string) string {

	if strings.Contains(input, okteto.DevRegistry) {
		tag := replaceRegistry(input, okteto.DevRegistry, okteto.Context().Namespace)
		return tag
	}

	if strings.Contains(input, okteto.GlobalRegistry) {
		globalNamespace := okteto.DefaultGlobalNamespace
		if okteto.Context().GlobalNamespace != "" {
			globalNamespace = okteto.Context().GlobalNamespace
		}
		tag := replaceRegistry(input, okteto.GlobalRegistry, globalNamespace)
		return tag
	}

	return input

}

func CreateDockerfileWithVolumeMounts(image string, volumes []model.StackVolume) (*model.BuildInfo, error) {
	build := &model.BuildInfo{}

	ctx, err := filepath.Abs(".")
	if err != nil {
		return build, nil
	}
	build.Context = ctx
	dockerfileTmpFolder := filepath.Join(config.GetOktetoHome(), ".dockerfile")
	if err := os.MkdirAll(dockerfileTmpFolder, 0700); err != nil {
		return build, fmt.Errorf("failed to create %s: %s", dockerfileTmpFolder, err)
	}

	tmpFile, err := os.CreateTemp(dockerfileTmpFolder, "buildkit-")
	if err != nil {
		return build, err
	}

	datawriter := bufio.NewWriter(tmpFile)
	defer datawriter.Flush()

	_, _ = datawriter.Write([]byte(fmt.Sprintf("FROM %s\n", image)))
	for _, volume := range volumes {
		_, _ = datawriter.Write([]byte(fmt.Sprintf("COPY %s %s\n", volume.LocalPath, volume.RemotePath)))
	}

	build.Dockerfile = tmpFile.Name()
	return build, nil
}

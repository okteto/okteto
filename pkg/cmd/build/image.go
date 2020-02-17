// Copyright 2020 The Okteto Authors
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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	appsv1 "k8s.io/api/apps/v1"
)

//GetRepoNameWithoutTag returns the image name without the tag
func GetRepoNameWithoutTag(name string) string {
	var domain, remainder string
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		domain, remainder = "", name
	} else {
		domain, remainder = name[:i], name[i+1:]
	}
	i = strings.LastIndex(remainder, ":")
	if i == -1 {
		return name
	}
	if domain == "" {
		return remainder[:i]
	}
	return fmt.Sprintf("%s/%s", domain, remainder[:i])
}

//GetImageTag returns the image taag to build and push
func GetImageTag(dev *model.Dev, imageTag string, d *appsv1.Deployment, isOktetoNamespace bool) string {
	if imageTag != "" {
		return imageTag
	}
	if isOktetoNamespace {
		return fmt.Sprintf("%s/%s/%s:okteto", okteto.GetRegistry(), dev.Namespace, dev.Name)
	}
	devContainer := deployments.GetDevContainer(&d.Spec.Template.Spec, dev.Container)
	imageWithoutTag := GetRepoNameWithoutTag(devContainer.Image)
	return fmt.Sprintf("%s:%s", imageWithoutTag, string(d.UID))
}

//GetDockerfileWithCacheHandler add multitenancy to the remote builds using cache mount ids
func GetDockerfileWithCacheHandler(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	dockerfileTmpFolder := filepath.Join(config.GetHome(), ".dockerfile")
	if err := os.MkdirAll(dockerfileTmpFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create %s: %s", dockerfileTmpFolder, err)
	}

	tmpFile, err := ioutil.TempFile(dockerfileTmpFolder, "buildkit-")
	if err != nil {
		return "", err
	}

	datawriter := bufio.NewWriter(tmpFile)
	defer datawriter.Flush()

	userID := okteto.GetUserID()
	if len(userID) == 0 {
		userID = "anonymous"
	}
	for scanner.Scan() {
		line := scanner.Text()
		traslatedLine := translateCacheHandler(line, userID)
		_, _ = datawriter.WriteString(traslatedLine + "\n")
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func translateCacheHandler(input string, userID string) string {
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
			return strings.Replace(input, "--mount=id=", fmt.Sprintf("--mount=id=%s-", userID), -1)
		}
		matched, err = regexp.MatchString(`^RUN.*--mount=[^ ]+,id=`, input)
		if err != nil {
			return input
		}
		if matched {
			return strings.Replace(input, ",id=", fmt.Sprintf(",id=%s-", userID), -1)
		}
		return strings.Replace(input, "--mount=", fmt.Sprintf("--mount=id=%s,", userID), -1)
	}
	return input
}

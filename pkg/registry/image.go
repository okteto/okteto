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
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/model"
)

// GetRepoNameAndTag returns the image name without the tag and the tag
func GetRepoNameAndTag(name string) (string, string) {
	var domain, remainder string
	i := strings.IndexRune(name, '@')
	if i != -1 {
		return name[:i], name[i+1:]
	}
	i = strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		domain, remainder = "", name
	} else {
		domain, remainder = name[:i], name[i+1:]
	}
	i = strings.LastIndex(remainder, ":")
	if i == -1 {
		return name, "latest"
	}
	if domain == "" {
		return remainder[:i], remainder[i+1:]
	}
	return fmt.Sprintf("%s/%s", domain, remainder[:i]), remainder[i+1:]
}

// GetImageTag returns the image tag to build for a given services
func GetImageTag(image, service, namespace, oktetoRegistryURL string) string {
	if oktetoRegistryURL != "" {
		if IsOktetoRegistry(image) {
			return image
		}
		return fmt.Sprintf("%s/%s/%s:okteto", oktetoRegistryURL, namespace, service)
	}
	imageWithoutTag, _ := GetRepoNameAndTag(image)
	return fmt.Sprintf("%s:okteto", imageWithoutTag)
}

// GetDevImageTag returns the image tag to build and push
func GetDevImageTag(dev *model.Dev, imageTag, imageFromDeployment, oktetoRegistryURL string) string {
	if imageTag != "" && imageTag != model.DefaultImage {
		return imageTag
	}
	return GetImageTag(imageFromDeployment, dev.Name, dev.Namespace, oktetoRegistryURL)
}

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

package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
)

//GetImageTagWithDigest returns the image tag diggest
func GetImageTagWithDigest(ctx context.Context, imageTag string) (string, error) {
	registryURL, err := okteto.GetRegistry()
	if err != nil {
		return "", nil
	}
	if !strings.HasPrefix(imageTag, registryURL) {
		return imageTag, nil
	}
	username := okteto.GetUserID()
	token, err := okteto.GetToken()
	if err != nil {
		return "", nil
	}
	c, err := New("https://"+registryURL, username, token.Token)
	if err != nil {
		return "", fmt.Errorf("error creating client: %s", err.Error())
	}

	digest, err := c.ManifestDigest("pchico83/ruby-dev", "latest")
	if err != nil {
		if strings.Contains(err.Error(), "status=404") {
			return "", errors.ErrNotFound
		}
		return "", fmt.Errorf("error getting image tag diggest: %s", err.Error())
	}
	repoName := GetRepoNameWithoutTag(imageTag)
	return repoName + "@" + digest.String(), nil
}

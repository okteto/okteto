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
	"net/url"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

//GetImageTagWithDigest returns the image tag diggest
func GetImageTagWithDigest(ctx context.Context, imageTag string) (string, error) {
	registryURL, err := okteto.GetRegistry()
	if err != nil {
		if err != errors.ErrNotLogged {
			log.Infof("error accessing to okteto registry: %s", err.Error())
		}
		return imageTag, nil
	}

	expandedTag, err := ExpandOktetoDevRegistry(ctx, imageTag)
	if err != nil {
		log.Infof("error expanding okteto registry: %s", err.Error())
		return imageTag, nil
	}
	if !strings.HasPrefix(expandedTag, registryURL) {
		return imageTag, nil
	}
	username := okteto.GetUserID()
	token, err := okteto.GetToken()
	if err != nil {
		log.Infof("error getting token: %s", err.Error())
		return imageTag, nil
	}
	u, err := url.Parse(registryURL)
	if err != nil {
		log.Infof("error parsing registry url: %s", err.Error())
		return imageTag, nil
	}
	u.Scheme = "https"
	c, err := NewRegistryClient(u.String(), username, token.Token)
	if err != nil {
		log.Infof("error creating registry client: %s", err.Error())
		return imageTag, nil
	}

	repoURL, tag := GetRepoNameAndTag(expandedTag)
	index := strings.IndexRune(repoURL, '/')
	if index == -1 {
		log.Infof("malformed registry url: %s", repoURL)
		return imageTag, nil
	}
	repoName := repoURL[index+1:]
	digest, err := c.ManifestDigest(repoName, tag)
	if err != nil {
		if strings.Contains(err.Error(), "status=404") {
			return "", errors.ErrNotFound
		}
		return "", fmt.Errorf("error getting image tag diggest: %s", err.Error())
	}
	return fmt.Sprintf("%s@%s", repoName, digest.String()), nil
}

//ExpandOktetoDevRegistry translates okteto.dev
func ExpandOktetoDevRegistry(ctx context.Context, tag string) (string, error) {
	if !strings.HasPrefix(tag, okteto.DevRegistry) {
		return tag, nil
	}

	c, _, namespace, err := client.GetLocal("")
	if err != nil {
		return "", fmt.Errorf("failed to load your local Kubeconfig: %s", err)
	}
	n, err := namespaces.Get(ctx, namespace, c)
	if err != nil {
		return "", fmt.Errorf("failed to get your current namespace '%s': %s", namespace, err.Error())
	}
	if !namespaces.IsOktetoNamespace(n) {
		return "", fmt.Errorf("cannot use the okteto.dev container registry: your current namespace '%s' is not managed by okteto", namespace)
	}

	oktetoRegistryURL, err := okteto.GetRegistry()
	if err != nil {
		return "", fmt.Errorf("cannot use the okteto.dev container registry: unable to get okteto registry url: %s", err)
	}

	tag = strings.Replace(tag, okteto.DevRegistry, fmt.Sprintf("%s/%s", oktetoRegistryURL, namespace), 1)
	return tag, nil
}

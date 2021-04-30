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
	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

//GetImageTagWithDigest returns the image tag digest
func GetImageTagWithDigest(ctx context.Context, namespace, imageTag string) (string, error) {
	registryURL, err := okteto.GetRegistry()
	if err != nil {
		if err != errors.ErrNotLogged {
			log.Infof("error accessing to okteto registry: %s", err.Error())
		}
		return imageTag, nil
	}

	expandedTag, err := ExpandOktetoDevRegistry(ctx, namespace, imageTag)
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
		return "", fmt.Errorf("error getting image tag digest: %s", err.Error())
	}
	return fmt.Sprintf("%s@%s", repoName, digest.String()), nil
}

//ExpandOktetoDevRegistry translates okteto.dev
func ExpandOktetoDevRegistry(ctx context.Context, namespace, tag string) (string, error) {
	if !strings.HasPrefix(tag, okteto.DevRegistry) {
		return tag, nil
	}

	c, _, err := client.GetLocal()
	if err != nil {
		return "", fmt.Errorf("failed to load your local Kubeconfig: %s", err)
	}

	if namespace == "" {
		namespace = client.GetContextNamespace("")
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

// IsTransientError returns true if err represents a transient registry error
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case strings.Contains(err.Error(), "failed commit on ref") && strings.Contains(err.Error(), "500 Internal Server Error"),
		strings.Contains(err.Error(), "transport is closing") && strings.Contains(err.Error(), "connect: connection refused"):
		return true
	default:
		return false
	}
}

func GetErrorMessage(err error, tag string) error {
	if err == nil {
		return nil
	}
	imageRegistry, imageTag := GetRegistryAndRepo(tag)
	switch {
	case IsLoggedIntoRegistryButDontHavePermissions(err):
		err = okErrors.UserError{
			E:    fmt.Errorf("You are not authorized to push image '%s'.", imageTag),
			Hint: fmt.Sprintf("Please login into the registry '%s' with a user with push permissions to '%s' or use another image.", imageRegistry, imageTag),
		}
	case IsNotLoggedIntoRegistry(err):
		err = okErrors.UserError{
			E:    fmt.Errorf("You are not authorized to push image '%s'.", imageTag),
			Hint: fmt.Sprintf("Login into the registry '%s' and verify that you have permissions to push the image '%s'.", imageRegistry, imageTag),
		}
	case IsBuildkitServiceUnavailable(err):
		err = okErrors.UserError{
			E:    fmt.Errorf("Buildkit service is not available at the moment."),
			Hint: "Please try again later.",
		}
	}
	return err
}

// IsLoggedIntoRegistryButDontHavePermissions returns true when the error is because the user is logged into the registry but doesn't have permissions to push the image
func IsLoggedIntoRegistryButDontHavePermissions(err error) bool {
	return strings.Contains(err.Error(), "insufficient_scope: authorization failed")
}

// IsNotLoggedIntoRegistry returns true when the error is because the user is not logged into the registry
func IsNotLoggedIntoRegistry(err error) bool {
	return strings.Contains(err.Error(), "failed to authorize: failed to fetch anonymous token")
}

func IsBuildkitServiceUnavailable(err error) bool {
	return strings.Contains(err.Error(), "connect: connection refused") || strings.Contains(err.Error(), "500 Internal Server Error")
}

// SplitRegistryAndImage returns image tag and the registry to push the image
func GetRegistryAndRepo(tag string) (string, string) {
	var imageTag string
	registryTag := "docker.io"
	splittedImage := strings.Split(tag, "/")

	if len(splittedImage) == 1 {
		imageTag = splittedImage[0]
	} else if len(splittedImage) == 2 {
		imageTag = strings.Join(splittedImage[len(splittedImage)-2:], "/")
	} else {
		imageTag = strings.Join(splittedImage[len(splittedImage)-2:], "/")
		registryTag = strings.Join(splittedImage[:len(splittedImage)-2], "/")
	}
	return registryTag, imageTag
}

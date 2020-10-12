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
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/pkg/errors"
)

// Run runs the build sequence
func Run(ctx context.Context, buildKitHost string, isOktetoCluster bool, path, dockerFile, tag, target string, noCache bool, cacheFrom []string, buildArgs []string, progress string) (string, error) {
	log.Infof("building your image on %s", buildKitHost)
	buildkitClient, err := getBuildkitClient(ctx, isOktetoCluster, buildKitHost)
	if err != nil {
		return "", err
	}

	processedDockerfile, err := getDockerFile(path, dockerFile, isOktetoCluster)
	if err != nil {
		return "", err
	}

	if isOktetoCluster {
		defer os.Remove(processedDockerfile)
	}

	tag, err = expandOktetoDevRegistry(ctx, tag)
	if err != nil {
		return "", err
	}
	for i := range cacheFrom {
		cacheFrom[i], err = expandOktetoDevRegistry(ctx, cacheFrom[i])
		if err != nil {
			return "", err
		}
	}
	opt, err := getSolveOpt(path, processedDockerfile, tag, target, noCache, cacheFrom, buildArgs)
	if err != nil {
		return "", errors.Wrap(err, "failed to create build solver")
	}

	return solveBuild(ctx, buildkitClient, opt, progress)
}

func expandOktetoDevRegistry(ctx context.Context, tag string) (string, error) {
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

	oldTag := tag
	tag = strings.Replace(tag, okteto.DevRegistry, fmt.Sprintf("%s/%s", oktetoRegistryURL, namespace), 1)

	log.Information("'%s' expanded to '%s'.", oldTag, tag)
	return tag, nil
}

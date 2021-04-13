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
	"path/filepath"

	"github.com/okteto/okteto/pkg/analytics"
	okErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/pkg/errors"
)

// Run runs the build sequence
func Run(ctx context.Context, namespace, buildKitHost string, isOktetoCluster bool, path, dockerFile, tag, target string, noCache bool, cacheFrom, buildArgs, secrets []string, progress string) error {
	log.Infof("building your image on %s", buildKitHost)
	buildkitClient, err := getBuildkitClient(ctx, isOktetoCluster, buildKitHost)
	if err != nil {
		return err
	}

	if dockerFile == "" {
		dockerFile = filepath.Join(path, "Dockerfile")
	}

	if buildKitHost == okteto.CloudBuildKitURL {
		dockerFile, err = registry.GetDockerfile(path, dockerFile)
		if err != nil {
			return err
		}
		defer os.Remove(dockerFile)
	}

	tag, err = registry.ExpandOktetoDevRegistry(ctx, namespace, tag)
	if err != nil {
		return err
	}
	for i := range cacheFrom {
		cacheFrom[i], err = registry.ExpandOktetoDevRegistry(ctx, namespace, cacheFrom[i])
		if err != nil {
			return err
		}
	}
	opt, err := getSolveOpt(path, dockerFile, tag, target, noCache, cacheFrom, buildArgs, secrets)
	if err != nil {
		return errors.Wrap(err, "failed to create build solver")
	}

	err = solveBuild(ctx, buildkitClient, opt, progress)
	if registry.IsTransientError(err) {
		log.Yellow("Failed to push '%s' to the registry, retrying ...", tag)
		success := true
		err := solveBuild(ctx, buildkitClient, opt, progress)
		if err != nil {
			success = false
		}
		analytics.TrackBuildTransientError(buildKitHost, success)
		return err
	}
	if err != nil {
		imageRegistry, imageTag := registry.GetRegistryAndRepo(tag)
		if registry.IsLoggedIntoRegistryButDontHavePermissions(err) {
			err = okErrors.UserError{E: fmt.Errorf("You are not authorized to push image '%s'.", imageTag),
				Hint: fmt.Sprintf("Please login into '%s' with an user with write permissions or use an image with write permissions.", imageRegistry)}
		}
		if registry.IsNotLoggedIntoRegistry(err) {
			err = okErrors.UserError{E: fmt.Errorf("You are not authorized to push image '%s'.", imageTag),
				Hint: fmt.Sprintf("Login into '%s' and verify that you have permissions to push images.", imageRegistry)}
		}
	}

	return err
}

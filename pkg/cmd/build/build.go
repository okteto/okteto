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
	"os"
	"path/filepath"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/pkg/errors"
)

// Run runs the build sequence
func Run(ctx context.Context, namespace, buildKitHost string, isOktetoCluster bool, path, dockerFile, tag, target string, noCache bool, cacheFrom, buildArgs []string, progress string) error {
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
	opt, err := getSolveOpt(path, dockerFile, tag, target, noCache, cacheFrom, buildArgs)
	if err != nil {
		return errors.Wrap(err, "failed to create build solver")
	}

	return solveBuild(ctx, buildkitClient, opt, progress)
}

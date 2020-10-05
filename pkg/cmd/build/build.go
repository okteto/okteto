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

	"github.com/okteto/okteto/pkg/log"
	"github.com/pkg/errors"
)

// Run runs the build sequence
func Run(ctx context.Context, buildKitHost string, isOktetoCluster bool, path, dockerFile, tag, target string, noCache bool, cacheFrom string, buildArgs []string, progress string) (string, error) {
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

	opt, err := getSolveOpt(path, processedDockerfile, tag, target, noCache, cacheFrom, buildArgs)
	if err != nil {
		return "", errors.Wrap(err, "failed to create build solver")
	}

	return solveBuild(ctx, buildkitClient, opt, progress)
}

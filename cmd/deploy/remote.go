// Copyright 2023 The Okteto Authors
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

package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"

	buildV1 "github.com/okteto/okteto/cmd/build/v1"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

type remoteDeployCommand struct {
	image     string
	namespace string
	deployer  localDeployer
}

func newRemoteDeployer() *remoteDeployCommand {
	return &remoteDeployCommand{}
}

func (rd *remoteDeployCommand) deploy(ctx context.Context, deployOptions *Options) error {

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	c, _, err := okteto.NewK8sClientProvider().Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c)

	if err := buildImages(ctx, buildv2.NewBuilderFromScratch().Build, buildv2.NewBuilderFromScratch().GetServicesToBuild, deployOptions); err != nil {
		return err
	}

	imageStepsUsedToDeploy := [][]byte{
		[]byte("FROM fokingwone/okteto as okteto-cli"),
		[]byte("FROM bitnami/kubectl as kubectl"),
		[]byte(fmt.Sprintf("FROM %s as deploy", deployOptions.Manifest.Deploy.Image)),
		[]byte(fmt.Sprintf("ENV %s %s", model.OktetoContextEnvVar, okteto.Context().Name)),
		[]byte(fmt.Sprintf("ENV %s %s", model.OktetoTokenEnvVar, okteto.Context().Token)),
		[]byte(fmt.Sprintf("ENV %s true", constants.OKtetoDeployRemote)),
		[]byte("COPY --from=okteto-cli /usr/local/bin/okteto /usr/local/bin/okteto"),
		[]byte("COPY --from=kubectl /opt/bitnami/kubectl/bin/kubectl /usr/local/bin/kubectl"),
		[]byte("COPY . ."),
		[]byte("RUN okteto deploy"),
	}
	dockerfileContent := bytes.Join(imageStepsUsedToDeploy, []byte("\n"))

	fs := afero.NewOsFs()

	dockerfile, err := afero.TempFile(fs, "", "Dockerfile.okteto.deploy")
	if err != nil {
		return err
	}
	err = afero.WriteFile(fs, dockerfile.Name(), dockerfileContent, 0600)
	if err != nil {
		return err
	}

	buildInfo := &model.BuildInfo{
		Dockerfile: dockerfile.Name(),
	}

	buildOptions := build.OptsFromBuildInfo("", "", buildInfo, &types.BuildOptions{Path: cwd})
	buildOptions.Tag = ""
	if err := buildV1.NewBuilderFromScratch().Build(ctx, buildOptions); err != nil {
		return err
	}

	return nil
}

func (rd *remoteDeployCommand) cleanUp(ctx context.Context, err error) {
	//TODO
	return
}

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
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	remoteBuild "github.com/okteto/okteto/cmd/build/remote"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	dockerfileTemplate = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM {{ .UserDeployImage }} as deploy

ENV PATH="${PATH}:/okteto/bin"
COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=okteto-cli /usr/local/bin/* /okteto/bin/

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}} {{$val}}
{{end}}
ENV {{ .NamespaceEnvVar }} {{ .NamespaceValue }}
ENV {{ .ContextEnvVar }} {{ .ContextValue }}
ENV {{ .TokenEnvVar }} {{ .TokenValue }}
ENV {{ .RemoteDeployEnvVar }} true

COPY . /okteto/src
WORKDIR /okteto/src

ENV OKTETO_INVALIDATE_CACHE {{ .RandomInt }}
RUN okteto deploy --log-output=json {{ .DeployFlags }}
`
)

type dockerfileTemplateProperties struct {
	OktetoCLIImage     string
	UserDeployImage    string
	OktetoBuildEnvVars map[string]string
	ContextEnvVar      string
	ContextValue       string
	NamespaceEnvVar    string
	NamespaceValue     string
	TokenEnvVar        string
	TokenValue         string
	RemoteDeployEnvVar string
	DeployFlags        string
	RandomInt          int
}

type remoteDeployCommand struct {
	builder *buildv2.OktetoBuilder
	fs      afero.Fs
}

func newRemoteDeployer(builder *buildv2.OktetoBuilder) *remoteDeployCommand {
	return &remoteDeployCommand{
		builder: builder,
		fs:      afero.NewOsFs(),
	}
}

func (rd *remoteDeployCommand) deploy(ctx context.Context, deployOptions *Options) error {

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cwd = getOriginalCWD(cwd, deployOptions.ManifestPathFlag)

	tmpl, err := template.New("dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return err
	}

	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:     getOktetoCLIVersion(config.VersionString),
		UserDeployImage:    deployOptions.Manifest.Deploy.Image,
		OktetoBuildEnvVars: rd.builder.GetBuildEnvVars(),
		ContextEnvVar:      model.OktetoContextEnvVar,
		ContextValue:       okteto.Context().Name,
		NamespaceEnvVar:    model.OktetoNamespaceEnvVar,
		NamespaceValue:     okteto.Context().Namespace,
		TokenEnvVar:        model.OktetoTokenEnvVar,
		TokenValue:         okteto.Context().Token,
		RemoteDeployEnvVar: constants.OKtetoDeployRemote,
		RandomInt:          rand.Intn(1000),
		DeployFlags:        strings.Join(getDeployFlags(deployOptions), " "),
	}

	tmpDir, err := afero.TempDir(rd.fs, "", "")
	if err != nil {
		return err
	}

	dockerfile, err := rd.fs.Create(filepath.Join(tmpDir, "deploy"))
	if err != nil {
		return err
	}

	err = rd.createDockerignoreIfNeeded(cwd, tmpDir)
	if err != nil {
		return err
	}

	defer func() {
		if err := rd.fs.Remove(dockerfile.Name()); err != nil {
			oktetoLog.Infof("error removing dockerfile: %w", err)
		}
	}()
	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return err
	}

	buildInfo := &model.BuildInfo{
		Dockerfile: dockerfile.Name(),
	}

	// undo modification of CWD for Build command
	os.Chdir(cwd)

	buildOptions := build.OptsFromBuildInfo("", "", buildInfo, &types.BuildOptions{Path: cwd, OutputMode: "deploy"})
	buildOptions.Tag = ""

	// we need to call Build() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	if err := remoteBuild.NewBuilderFromScratch().Build(ctx, buildOptions); err != nil {
		return oktetoErrors.UserError{
			E: fmt.Errorf("Error during development environment deployment."),
		}
	}
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return nil
}

func (rd *remoteDeployCommand) cleanUp(ctx context.Context, err error) {
	return
}

func (rd *remoteDeployCommand) createDockerignoreIfNeeded(cwd, tmpDir string) error {
	dockerignoreFilePath := fmt.Sprintf("%s/%s", cwd, ".oktetodeployignore")
	if _, err := rd.fs.Stat(dockerignoreFilePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		dockerignoreContent, err := afero.ReadFile(rd.fs, dockerignoreFilePath)
		if err != nil {
			return err
		}

		err = afero.WriteFile(rd.fs, fmt.Sprintf("%s/%s", tmpDir, ".dockerignore"), dockerignoreContent, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func getDeployFlags(opts *Options) []string {
	var deployFlags []string

	if opts.Name != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--name %s", opts.Name))
	}

	if opts.Namespace != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--namespace %s", opts.Namespace))
	}

	if opts.ManifestPathFlag != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--file %s", opts.ManifestPathFlag))
	}

	if len(opts.Variables) > 0 {
		var varsToAddForDeploy []string
		for _, v := range opts.Variables {
			varsToAddForDeploy = append(varsToAddForDeploy, fmt.Sprintf("--var %s", v))
		}
		deployFlags = append(deployFlags, strings.Join(varsToAddForDeploy, " "))
	}

	return deployFlags
}

func getOriginalCWD(cwd, manifestPath string) string {
	manifestPathDir := filepath.Dir(fmt.Sprintf("/%s", manifestPath))
	return strings.TrimSuffix(cwd, manifestPathDir)
}

func getOktetoCLIVersion(versionString string) string {
	var version string
	if match, _ := regexp.MatchString(`\d+\.\d+\.\d+`, versionString); match {
		version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, versionString)
	} else {
		remoteOktetoImage := os.Getenv(constants.OKtetoDeployRemoteImage)
		if remoteOktetoImage != "" {
			version = remoteOktetoImage
		} else {
			version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, "latest")
		}
	}

	return version
}

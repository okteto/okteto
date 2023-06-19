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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	builder "github.com/okteto/okteto/cmd/build"
	remoteBuild "github.com/okteto/okteto/cmd/build/remote"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	templateName           = "dockerfile"
	dockerfileTemporalName = "deploy"
	oktetoDockerignoreName = ".oktetodeployignore"
	dockerfileTemplate     = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM {{ .InstallerImage }} as installer

FROM {{ .UserDeployImage }} as deploy

ENV PATH="${PATH}:/okteto/bin"
COPY --from=installer /app/bin/* /okteto/bin/
COPY --from=okteto-cli /usr/local/bin/* /okteto/bin/

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}} {{$val}}
{{end}}
ENV {{ .NamespaceEnvVar }} {{ .NamespaceValue }}
ENV {{ .ContextEnvVar }} {{ .ContextValue }}
ENV {{ .TokenEnvVar }} {{ .TokenValue }}
ENV {{ .RemoteDeployEnvVar }} true
{{ if ne .ActionNameValue "" }}
ENV {{ .ActionNameEnvVar }} {{ .ActionNameValue }}
{{ end }}
{{ if ne .GitCommitValue "" }}
ENV {{ .GitCommitEnvVar }} {{ .GitCommitValue }}
{{ end }}

COPY . /okteto/src
WORKDIR /okteto/src

ENV OKTETO_INVALIDATE_CACHE {{ .RandomInt }}
ARG OKTETO_TLS_CERT_BASE64
ARG INTERNAL_SERVER_NAME=""
RUN echo "$OKTETO_TLS_CERT_BASE64" | base64 -d > /etc/ssl/certs/okteto.crt
RUN okteto deploy --log-output=json --server-name="$INTERNAL_SERVER_NAME" {{ .DeployFlags }}
`
)

type dockerfileTemplateProperties struct {
	OktetoCLIImage     string
	InstallerImage     string
	UserDeployImage    string
	OktetoBuildEnvVars map[string]string
	ContextEnvVar      string
	ContextValue       string
	NamespaceEnvVar    string
	NamespaceValue     string
	TokenEnvVar        string
	TokenValue         string
	ActionNameEnvVar   string
	ActionNameValue    string
	GitCommitEnvVar    string
	GitCommitValue     string
	RemoteDeployEnvVar string
	DeployFlags        string
	RandomInt          int
}

type remoteDeployCommand struct {
	builderV2            *buildv2.OktetoBuilder
	builderV1            builder.Builder
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
	clusterMetadata      func(context.Context) (*types.ClusterMetadata, error)
}

// newRemoteDeployer creates the remote deployer from a
func newRemoteDeployer(builder *buildv2.OktetoBuilder) *remoteDeployCommand {
	fs := afero.NewOsFs()
	return &remoteDeployCommand{
		builderV2:            builder,
		builderV1:            remoteBuild.NewBuilderFromScratch(),
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewOsWorkingDirectoryCtrl(),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata:      fetchRemoteServerConfig,
	}
}

func (rd *remoteDeployCommand) deploy(ctx context.Context, deployOptions *Options) error {

	sc, err := rd.clusterMetadata(ctx)
	if err != nil {
		return err
	}

	if deployOptions.Manifest != nil && deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.Image == "" {
		deployOptions.Manifest.Deploy.Image = sc.PipelineRunnerImage
	}

	cwd, err := rd.getOriginalCWD(deployOptions.ManifestPathFlag)
	if err != nil {
		return err
	}

	tmpDir, err := rd.temporalCtrl.Create()
	if err != nil {
		return err
	}

	dockerfile, err := rd.createDockerfile(tmpDir, deployOptions, sc.PipelineInstallerImage)
	if err != nil {
		return err
	}

	defer func() {
		if err := rd.fs.Remove(dockerfile); err != nil {
			oktetoLog.Infof("error removing dockerfile: %w", err)
		}
	}()

	buildInfo := &model.BuildInfo{
		Dockerfile: dockerfile,
	}

	// undo modification of CWD for Build command
	if err := rd.workingDirectoryCtrl.Change(cwd); err != nil {
		return err
	}

	buildOptions := build.OptsFromBuildInfoForRemoteDeploy(buildInfo, &types.BuildOptions{OutputMode: "deploy"})
	buildOptions.Manifest = deployOptions.Manifest
	buildOptions.BuildArgs = append(
		buildOptions.BuildArgs,
		fmt.Sprintf("OKTETO_TLS_CERT_BASE64=%s", base64.StdEncoding.EncodeToString(sc.Certificate)),
		fmt.Sprintf("INTERNAL_SERVER_NAME=%s", sc.ServerName),
	)
	// we need to call Build() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	if err := rd.builderV1.Build(ctx, buildOptions); err != nil {
		var cmdErr build.OktetoCommandErr
		if errors.As(err, &cmdErr) {
			oktetoLog.SetStage(cmdErr.Stage)
			return oktetoErrors.UserError{
				E: cmdErr.Err,
			}
		}
		oktetoLog.SetStage("remote deploy")
		var userErr oktetoErrors.UserError
		if errors.As(err, &userErr) {
			return userErr
		}
		return oktetoErrors.UserError{
			E: err,
		}
	}
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return nil
}

func (rd *remoteDeployCommand) cleanUp(ctx context.Context, err error) {}

func (rd *remoteDeployCommand) createDockerfile(tmpDir string, opts *Options, installerImage string) (string, error) {
	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}

	tmpl := template.Must(template.New(templateName).Parse(dockerfileTemplate))

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return "", err
	}

	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:     getOktetoCLIVersion(config.VersionString),
		UserDeployImage:    opts.Manifest.Deploy.Image,
		InstallerImage:     installerImage,
		OktetoBuildEnvVars: rd.builderV2.GetBuildEnvVars(),
		ContextEnvVar:      model.OktetoContextEnvVar,
		ContextValue:       okteto.Context().Name,
		NamespaceEnvVar:    model.OktetoNamespaceEnvVar,
		NamespaceValue:     okteto.Context().Namespace,
		TokenEnvVar:        model.OktetoTokenEnvVar,
		TokenValue:         okteto.Context().Token,
		ActionNameEnvVar:   model.OktetoActionNameEnvVar,
		ActionNameValue:    os.Getenv(model.OktetoActionNameEnvVar),
		GitCommitEnvVar:    constants.OktetoGitCommitEnvVar,
		GitCommitValue:     os.Getenv(constants.OktetoGitCommitEnvVar),
		RemoteDeployEnvVar: constants.OKtetoDeployRemote,
		RandomInt:          int(randomNumber.Int64()),
		DeployFlags:        strings.Join(getDeployFlags(opts), " "),
	}

	dockerfile, err := rd.fs.Create(filepath.Join(tmpDir, dockerfileTemporalName))
	if err != nil {
		return "", err
	}

	err = rd.createDockerignore(cwd, tmpDir)
	if err != nil {
		return "", err
	}

	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return "", err
	}
	return dockerfile.Name(), nil
}

func (rd *remoteDeployCommand) createDockerignore(cwd, tmpDir string) error {
	// if we do not create a .dockerignore (with or without content) used to create
	// the remote executor, we would use the one located in root (the one used to
	// build the services) so we would create a remote executor without certain files
	// necessary for the later deployment which would cause an error when deploying
	// remotely due to the lack of these files.
	dockerignoreContent := []byte(``)
	dockerignoreFilePath := filepath.Join(cwd, oktetoDockerignoreName)
	if _, err := rd.fs.Stat(dockerignoreFilePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

	} else {
		dockerignoreContent, err = afero.ReadFile(rd.fs, dockerignoreFilePath)
		if err != nil {
			return err
		}
	}
	err := afero.WriteFile(rd.fs, fmt.Sprintf("%s/%s", tmpDir, ".dockerignore"), dockerignoreContent, 0600)
	if err != nil {
		return err
	}

	return nil
}

func getDeployFlags(opts *Options) []string {
	var deployFlags []string

	if opts.Name != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--name \"%s\"", opts.Name))
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

// getOriginalCWD returns the original cwd
func (rd *remoteDeployCommand) getOriginalCWD(manifestPath string) (string, error) {
	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}
	manifestPathDir := filepath.Dir(filepath.Clean(fmt.Sprintf("/%s", manifestPath)))
	return strings.TrimSuffix(cwd, manifestPathDir), nil
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

func fetchRemoteServerConfig(ctx context.Context) (*types.ClusterMetadata, error) {
	cp := okteto.NewOktetoClientProvider()
	c, err := cp.Provide()
	if err != nil {
		return nil, fmt.Errorf("failed to provide okteto client for fetching certs: %s", err)
	}
	uc := c.User()

	metadata, err := uc.GetClusterMetadata(ctx, okteto.Context().Namespace)
	if err != nil {
		return nil, err
	}

	if metadata.Certificate == nil {
		metadata.Certificate, err = uc.GetClusterCertificate(ctx, okteto.Context().Name, okteto.Context().Namespace)
	}

	return &metadata, err
}

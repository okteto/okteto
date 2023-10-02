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
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/okteto/okteto/pkg/config"

	"github.com/mitchellh/go-homedir"

	builder "github.com/okteto/okteto/cmd/build"
	remoteBuild "github.com/okteto/okteto/cmd/build/remote"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	templateName           = "dockerfile"
	dockerfileTemporalName = "Dockerfile.deploy"
	dockerfileTemplate     = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM {{ .UserDeployImage }} as deploy

ENV PATH="${PATH}:/okteto/bin"
COPY --from=okteto-cli /usr/local/bin/* /okteto/bin/


ENV {{ .RemoteDeployEnvVar }} true
ARG {{ .NamespaceArgName }}
ARG {{ .ContextArgName }}
ARG {{ .TokenArgName }}
ARG {{ .ActionNameArgName }}
ARG {{ .TlsCertBase64ArgName }}
ARG {{ .InternalServerName }}
RUN mkdir -p /etc/ssl/certs/
RUN echo "${{ .TlsCertBase64ArgName }}" | base64 -d > /etc/ssl/certs/okteto.crt

COPY . /okteto/src
WORKDIR /okteto/src

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}} {{$val}}
{{end}}

ARG {{ .GitCommitArgName }}
ARG {{ .InvalidateCacheArgName }}

RUN okteto registrytoken install --force --log-output=json

RUN --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  okteto deploy --log-output=json --server-name="${{ .InternalServerName }}" {{ .DeployFlags }}
`
)

type dockerfileTemplateProperties struct {
	OktetoCLIImage         string
	UserDeployImage        string
	RemoteDeployEnvVar     string
	OktetoBuildEnvVars     map[string]string
	ContextArgName         string
	NamespaceArgName       string
	TokenArgName           string
	TlsCertBase64ArgName   string
	InternalServerName     string
	ActionNameArgName      string
	GitCommitArgName       string
	InvalidateCacheArgName string
	DeployFlags            string
}

type remoteDeployCommand struct {
	getBuildEnvVars      func() map[string]string
	builderV1            builder.Builder
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
	clusterMetadata      func(context.Context) (*types.ClusterMetadata, error)

	// sshAuthSockEnvvar is the default for SSH_AUTH_SOCK. Provided mostly for testing
	sshAuthSockEnvvar string

	// knownHostsPath  is the default known_hosts file path. Provided mostly for testing
	knownHostsPath string
}

// newRemoteDeployer creates the remote deployer from a
func newRemoteDeployer(builder builderInterface) *remoteDeployCommand {
	fs := afero.NewOsFs()
	return &remoteDeployCommand{
		getBuildEnvVars:      builder.GetBuildEnvVars,
		builderV1:            remoteBuild.NewBuilderFromScratch(),
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewOsWorkingDirectoryCtrl(),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata:      fetchRemoteServerConfig,
	}
}

func (rd *remoteDeployCommand) deploy(ctx context.Context, deployOptions *Options) error {
	home, err := homedir.Dir()
	if err != nil {
		return err
	}
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

	dockerfile, err := rd.createDockerfile(tmpDir, deployOptions)
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

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}

	buildOptions := build.OptsFromBuildInfoForRemoteDeploy(buildInfo, &types.BuildOptions{OutputMode: "deploy"})
	buildOptions.Manifest = deployOptions.Manifest
	buildOptions.BuildArgs = append(
		buildOptions.BuildArgs,
		fmt.Sprintf("%s=%s", model.OktetoContextEnvVar, okteto.Context().Name),
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.Context().Namespace),
		fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, okteto.Context().Token),
		fmt.Sprintf("%s=%s", constants.OktetoTlsCertBase64EnvVar, base64.StdEncoding.EncodeToString(sc.Certificate)),
		fmt.Sprintf("%s=%s", constants.OktetoInternalServerNameEnvVar, sc.ServerName),
		fmt.Sprintf("%s=%s", model.OktetoActionNameEnvVar, os.Getenv(model.OktetoActionNameEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitCommitEnvVar, os.Getenv(constants.OktetoGitCommitEnvVar)),
		fmt.Sprintf("%s=%d", constants.OktetoInvalidateCacheEnvVar, int(randomNumber.Int64())),
	)

	if sc.ServerName != "" {
		registryUrl := okteto.Context().Registry
		subdomain := strings.TrimPrefix(registryUrl, "registry.")
		ip, _, err := net.SplitHostPort(sc.ServerName)
		if err != nil {
			return fmt.Errorf("failed to parse server name network address: %w", err)
		}
		buildOptions.ExtraHosts = []types.HostMap{
			{Hostname: registryUrl, IP: ip},
			{Hostname: fmt.Sprintf("kubernetes.%s", subdomain), IP: ip},
		}
	}

	sshSock := os.Getenv(rd.sshAuthSockEnvvar)
	if sshSock == "" {
		sshSock = os.Getenv("SSH_AUTH_SOCK")
	}

	if sshSock != "" {
		if _, err := os.Stat(sshSock); err != nil {
			oktetoLog.Debugf("Not mounting ssh agent. Error reading socket: %s", err.Error())
		} else {
			sshSession := types.BuildSshSession{Id: "remote", Target: sshSock}
			buildOptions.SshSessions = append(buildOptions.SshSessions, sshSession)
		}

		// TODO: check if ~/.ssh/config exists and has UserKnownHostsFile defined
		knownHostsPath := rd.knownHostsPath
		if knownHostsPath == "" {
			knownHostsPath = filepath.Join(home, ".ssh", "known_hosts")
		}
		if _, err := os.Stat(knownHostsPath); err != nil {
			oktetoLog.Debugf("Not know_hosts file. Error reading file: %s", err.Error())
		} else {
			oktetoLog.Debugf("reading known hosts from %s", knownHostsPath)
			buildOptions.Secrets = append(buildOptions.Secrets, fmt.Sprintf("id=known_hosts,src=%s", knownHostsPath))
		}

	} else {
		oktetoLog.Debug("no ssh agent found. Not mouting ssh-agent for build")
	}

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

func (rd *remoteDeployCommand) createDockerfile(tmpDir string, opts *Options) (string, error) {
	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}

	tmpl := template.
		Must(template.New(templateName).
			Funcs(template.FuncMap{"join": strings.Join}).
			Parse(dockerfileTemplate))

	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:         getOktetoCLIVersion(config.VersionString),
		UserDeployImage:        opts.Manifest.Deploy.Image,
		RemoteDeployEnvVar:     constants.OktetoDeployRemote,
		ContextArgName:         model.OktetoContextEnvVar,
		OktetoBuildEnvVars:     rd.getBuildEnvVars(),
		NamespaceArgName:       model.OktetoNamespaceEnvVar,
		TlsCertBase64ArgName:   constants.OktetoTlsCertBase64EnvVar,
		InternalServerName:     constants.OktetoInternalServerNameEnvVar,
		TokenArgName:           model.OktetoTokenEnvVar,
		ActionNameArgName:      model.OktetoActionNameEnvVar,
		GitCommitArgName:       constants.OktetoGitCommitEnvVar,
		InvalidateCacheArgName: constants.OktetoInvalidateCacheEnvVar,
		DeployFlags:            strings.Join(getDeployFlags(opts), " "),
	}

	dockerfile, err := rd.fs.Create(filepath.Join(tmpDir, dockerfileTemporalName))
	if err != nil {
		return "", err
	}

	// if we do not create a .dockerignore (with or without content) used to create
	// the remote executor, we would use the one located in root (the one used to
	// build the services) so we would create a remote executor without certain files
	// necessary for the later deployment which would cause an error when deploying
	// remotely due to the lack of these files.
	if err := remote.CreateDockerignoreFileWithFilesystem(cwd, tmpDir, opts.ManifestPathFlag, rd.fs); err != nil {
		return "", err
	}

	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return "", err
	}
	return dockerfile.Name(), nil
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

	if opts.Wait {
		deployFlags = append(deployFlags, "--wait")
	}

	deployFlags = append(deployFlags, fmt.Sprintf("--timeout %s", opts.Timeout))

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
	if match, err := regexp.MatchString(`\d+\.\d+\.\d+`, versionString); match {
		version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, versionString)
	} else {
		oktetoLog.Infof("invalid version string: %s, using latest: %s", versionString, err)
		remoteOktetoImage := os.Getenv(constants.OktetoDeployRemoteImage)
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

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

	"github.com/mitchellh/go-homedir"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
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
ARG {{ .OktetoDeployable }}
ARG {{ .GitHubRepositoryArgName }}
ARG {{ .BuildKitHostArgName }}
ARG {{ .OktetoRegistryURLArgName }}
RUN mkdir -p /etc/ssl/certs/
RUN echo "${{ .TlsCertBase64ArgName }}" | base64 -d > /etc/ssl/certs/okteto.crt

COPY . /okteto/src
WORKDIR /okteto/src

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}} {{$val}}
{{end}}

{{range $key, $val := .OktetoDependencyEnvVars }}
ENV {{$key}} {{$val}}
{{end}}

ARG {{ .GitCommitArgName }}
ARG {{ .GitBranchArgName }}
ARG {{ .InvalidateCacheArgName }}

RUN okteto registrytoken install --force --log-output=json

RUN --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run --log-output=json --server-name="${{ .InternalServerName }}" {{ .CommandFlags }}
`
)

// remoteRunner is the interface to run the deploy command remotely. The implementation is using
// an image builder like BuildKit
type remoteRunner interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
}

type dockerfileTemplateProperties struct {
	OktetoCLIImage           string
	UserDeployImage          string
	RemoteDeployEnvVar       string
	OktetoBuildEnvVars       map[string]string
	OktetoDependencyEnvVars  map[string]string
	ContextArgName           string
	NamespaceArgName         string
	TokenArgName             string
	TlsCertBase64ArgName     string
	InternalServerName       string
	ActionNameArgName        string
	GitCommitArgName         string
	GitBranchArgName         string
	InvalidateCacheArgName   string
	CommandFlags             string
	OktetoDeployable         string
	GitHubRepositoryArgName  string
	BuildKitHostArgName      string
	OktetoRegistryURLArgName string
}

type environGetter func() []string

type buildEnvVarsGetter func() map[string]string

type dependencyEnvVarsGetter func(environGetter environGetter) map[string]string

type remoteDeployer struct {
	getBuildEnvVars      buildEnvVarsGetter
	runner               remoteRunner
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
	clusterMetadata      func(context.Context) (*types.ClusterMetadata, error)

	// ioCtrl is the controller for the output of the Build logs
	ioCtrl *io.Controller

	getDependencyEnvVars dependencyEnvVarsGetter

	// sshAuthSockEnvvar is the default for SSH_AUTH_SOCK. Provided mostly for testing
	sshAuthSockEnvvar string

	// knownHostsPath  is the default known_hosts file path. Provided mostly for testing
	knownHostsPath string

	useInternalNetwork bool
}

// newRemoteDeployer creates the remote deployer from a
func newRemoteDeployer(buildVarsGetter buildEnvVarsGetter, ioCtrl *io.Controller, getDependencyEnvVars dependencyEnvVarsGetter) *remoteDeployer {
	fs := afero.NewOsFs()
	runner := buildCmd.NewOktetoBuilder(
		&okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		fs,
	)
	return &remoteDeployer{
		getBuildEnvVars:      buildVarsGetter,
		runner:               runner,
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewOsWorkingDirectoryCtrl(),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		clusterMetadata:      fetchRemoteServerConfig,
		ioCtrl:               ioCtrl,
		getDependencyEnvVars: getDependencyEnvVars,
		useInternalNetwork:   !buildCmd.IsDepotEnabled(),
	}
}

func (rd *remoteDeployer) Deploy(ctx context.Context, deployOptions *Options) error {
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
			oktetoLog.Infof("error removing dockerfile: %s", err)
		}
	}()

	buildInfo := &build.Info{
		Dockerfile: dockerfile,
		Context:    rd.getContextPath(cwd, deployOptions.ManifestPathFlag),
	}

	// undo modification of CWD for Build command
	if err := rd.workingDirectoryCtrl.Change(cwd); err != nil {
		return err
	}

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}

	deployable := deployable.Entity{
		Divert:   deployOptions.Manifest.Deploy.Divert,
		Commands: deployOptions.Manifest.Deploy.Commands,
		External: deployOptions.Manifest.External,
	}

	b, err := yaml.Marshal(deployable)
	if err != nil {
		return err
	}

	buildOptions := buildCmd.OptsFromBuildInfoForRemoteDeploy(buildInfo, &types.BuildOptions{OutputMode: buildCmd.DeployOutputModeOnBuild})
	buildOptions.Manifest = deployOptions.Manifest
	buildOptions.BuildArgs = append(
		buildOptions.BuildArgs,
		fmt.Sprintf("%s=%s", model.OktetoContextEnvVar, okteto.GetContext().Name),
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.GetContext().Namespace),
		fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, okteto.GetContext().Token),
		fmt.Sprintf("%s=%s", model.OktetoActionNameEnvVar, os.Getenv(model.OktetoActionNameEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitCommitEnvVar, os.Getenv(constants.OktetoGitCommitEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitBranchEnvVar, os.Getenv(constants.OktetoGitBranchEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoTlsCertBase64EnvVar, base64.StdEncoding.EncodeToString(sc.Certificate)),
		fmt.Sprintf("%s=%d", constants.OktetoInvalidateCacheEnvVar, int(randomNumber.Int64())),
		fmt.Sprintf("%s=%s", constants.OktetoDeployableEnvVar, base64.StdEncoding.EncodeToString(b)),
		fmt.Sprintf("%s=%s", model.GithubRepositoryEnvVar, os.Getenv(model.GithubRepositoryEnvVar)),
		fmt.Sprintf("%s=%s", model.OktetoRegistryURLEnvVar, os.Getenv(model.OktetoRegistryURLEnvVar)),
		fmt.Sprintf("%s=%s", model.OktetoBuildkitHostURLEnvVar, os.Getenv(model.OktetoBuildkitHostURLEnvVar)),
	)

	if rd.useInternalNetwork {
		buildOptions.BuildArgs = append(
			buildOptions.BuildArgs,
			fmt.Sprintf("%s=%s", constants.OktetoInternalServerNameEnvVar, sc.ServerName),
		)
		if sc.ServerName != "" {
			registryUrl := okteto.GetContext().Registry
			subdomain := strings.TrimPrefix(registryUrl, "registry.")
			ip, _, err := net.SplitHostPort(sc.ServerName)
			if err != nil {
				return fmt.Errorf("failed to parse server name network address: %w", err)
			}
			buildOptions.ExtraHosts = getExtraHosts(registryUrl, subdomain, ip, *sc)
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
		oktetoLog.Debug("no ssh agent found. Not mounting ssh-agent for build")
	}

	// we need to call Run() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	if err := rd.runner.Run(ctx, buildOptions, rd.ioCtrl); err != nil {
		var cmdErr buildCmd.OktetoCommandErr
		if errors.As(err, &cmdErr) {
			oktetoLog.SetStage(cmdErr.Stage)
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying application: %s", cmdErr.Err.Error())
			return fmt.Errorf("error deploying application: %w", cmdErr.Err)
		}
		oktetoLog.SetStage("remote deploy")
		var userErr oktetoErrors.UserError
		if errors.As(err, &userErr) {
			return userErr
		}
		return fmt.Errorf("error deploying application: %w", err)
	}

	return nil
}

func (rd *remoteDeployer) createDockerfile(tmpDir string, opts *Options) (string, error) {
	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}

	tmpl := template.
		Must(template.New(templateName).
			Funcs(template.FuncMap{"join": strings.Join}).
			Parse(dockerfileTemplate))

	commandFlags, err := getCommandFlags(opts)
	if err != nil {
		return "", err
	}

	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:           getOktetoCLIVersion(config.VersionString),
		UserDeployImage:          opts.Manifest.Deploy.Image,
		RemoteDeployEnvVar:       constants.OktetoDeployRemote,
		ContextArgName:           model.OktetoContextEnvVar,
		OktetoBuildEnvVars:       rd.getBuildEnvVars(),
		NamespaceArgName:         model.OktetoNamespaceEnvVar,
		TlsCertBase64ArgName:     constants.OktetoTlsCertBase64EnvVar,
		InternalServerName:       constants.OktetoInternalServerNameEnvVar,
		TokenArgName:             model.OktetoTokenEnvVar,
		ActionNameArgName:        model.OktetoActionNameEnvVar,
		GitCommitArgName:         constants.OktetoGitCommitEnvVar,
		GitBranchArgName:         constants.OktetoGitBranchEnvVar,
		InvalidateCacheArgName:   constants.OktetoInvalidateCacheEnvVar,
		CommandFlags:             strings.Join(commandFlags, " "),
		OktetoDeployable:         constants.OktetoDeployableEnvVar,
		GitHubRepositoryArgName:  model.GithubRepositoryEnvVar,
		BuildKitHostArgName:      model.OktetoBuildkitHostURLEnvVar,
		OktetoRegistryURLArgName: model.OktetoRegistryURLEnvVar,
		OktetoDependencyEnvVars:  rd.getDependencyEnvVars(os.Environ),
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
	if err := remote.CreateDockerignoreFileWithFilesystem(cwd, tmpDir, filesystem.CleanManifestPath(opts.ManifestPathFlag), rd.fs); err != nil {
		return "", err
	}

	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return "", err
	}
	return dockerfile.Name(), nil
}

func getCommandFlags(opts *Options) ([]string, error) {
	var commandFlags []string
	commandFlags = append(commandFlags, fmt.Sprintf("--name %q", opts.Name))
	if len(opts.Variables) > 0 {
		var varsToAddForDeploy []string
		variables, err := parse(opts.Variables)
		if err != nil {
			return nil, err
		}
		for _, v := range variables {
			varsToAddForDeploy = append(varsToAddForDeploy, fmt.Sprintf("--var %s=%q", v.Name, v.Value))
		}
		commandFlags = append(commandFlags, strings.Join(varsToAddForDeploy, " "))
	}

	return commandFlags, nil
}

// getOriginalCWD returns the original cwd
func (rd *remoteDeployer) getOriginalCWD(manifestPath string) (string, error) {
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
		if err != nil {
			oktetoLog.Infof("invalid version string: %s. Error: %s, using latest", versionString, err)
		} else {
			oktetoLog.Infof("invalid version string: %s, using latest", versionString)
		}
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
		return nil, fmt.Errorf("failed to provide okteto client for fetching certs: %w", err)
	}
	uc := c.User()

	metadata, err := uc.GetClusterMetadata(ctx, okteto.GetContext().Namespace)
	if err != nil {
		return nil, err
	}

	if metadata.Certificate == nil {
		metadata.Certificate, err = uc.GetClusterCertificate(ctx, okteto.GetContext().Name, okteto.GetContext().Namespace)
	}

	return &metadata, err
}

func getExtraHosts(registryURL, subdomain, ip string, metadata types.ClusterMetadata) []types.HostMap {
	extraHosts := []types.HostMap{
		{Hostname: registryURL, IP: ip},
		{Hostname: fmt.Sprintf("kubernetes.%s", subdomain), IP: ip},
	}

	if metadata.BuildKitInternalIP != "" {
		extraHosts = append(extraHosts, types.HostMap{Hostname: fmt.Sprintf("buildkit.%s", subdomain), IP: metadata.BuildKitInternalIP})
	}

	if metadata.PublicDomain != "" {
		extraHosts = append(extraHosts, types.HostMap{Hostname: metadata.PublicDomain, IP: ip})
	}

	return extraHosts
}

func (rd *remoteDeployer) getContextPath(cwd, manifestPath string) string {
	if manifestPath == "" {
		return cwd
	}

	path := manifestPath
	if !filepath.IsAbs(manifestPath) {
		path = filepath.Join(cwd, manifestPath)
	}
	fInfo, err := rd.fs.Stat(path)
	if err != nil {
		oktetoLog.Infof("error getting file info: %s", err)
		return cwd

	}
	if fInfo.IsDir() {
		return path
	}

	possibleCtx := filepath.Dir(path)
	if strings.HasSuffix(possibleCtx, ".okteto") {
		return filepath.Dir(possibleCtx)
	}
	return possibleCtx
}

func (*remoteDeployer) CleanUp(_ context.Context, _ error) {
	// Do nothing
}

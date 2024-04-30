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

package remote

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
	"strconv"
	"strings"
	"text/template"

	"github.com/mitchellh/go-homedir"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
)

const (
	// TestCommand is the command to run test remotely
	TestCommand = "test"
	// DeployCommand is the command to deploy a dev environment remotely
	DeployCommand = "deploy"
	// DestroyCommand is the command to destroy a dev environment remotely
	DestroyCommand         = "destroy"
	oktetoDockerignoreName = ".oktetodeployignore"
	dockerfileTemplate     = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM {{ .UserRunnerImage }} as runner

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
ARG {{ .OktetoIsPreviewEnv }}
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

RUN \
  {{range $key, $path := .Caches }}--mount=type=cache,target={{$path}}{{end}}\
  --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  /okteto/bin/okteto remote-run {{ .Command }} --log-output=json --server-name="${{ .InternalServerName }}" {{ .CommandFlags }}
`
)

type OktetoClientProvider interface {
	Provide(opts ...okteto.Option) (types.OktetoInterface, error)
}

// Builder is the interface to run the build of the Dockerfile
// to execute remote commands like deploy and destroy
type Builder interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
}

// Runner struct in charge of creating the Dockerfile for remote execution
// of commands like deploy and destroy and running the build
type Runner struct {
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
	builder              Builder
	oktetoClientProvider OktetoClientProvider
	ioCtrl               *io.Controller
	// sshAuthSockEnvvar is the default for SSH_AUTH_SOCK. Provided mostly for testing
	sshAuthSockEnvvar  string
	useInternalNetwork bool
}

// Params struct to pass the necessary parameters to create the Dockerfile
type Params struct {
	BuildEnvVars        map[string]string
	DependenciesEnvVars map[string]string
	Manifest            *model.Manifest
	BaseImage           string
	ManifestPathFlag    string
	TemplateName        string
	DockerfileName      string
	KnownHostsPath      string
	Command             string
	// ContextAbsolutePathOverride is the absolute path for the build context. Optional.
	// If this values is not defined it will default to the folder location of the
	// okteto manifest which is resolved through params.ManifestPathFlag
	ContextAbsolutePathOverride string
	// CacheInvalidationKey is the value use to invalidate the cache. Defaults
	// to a random value which essentially means no-cache. Setting this to a
	// static or known value will reuse the build cache
	CacheInvalidationKey string
	Deployable           deployable.Entity
	CommandFlags         []string
	Caches               []string
}

// dockerfileTemplateProperties internal struct with the information needed by the Dockerfile template
type dockerfileTemplateProperties struct {
	OktetoCLIImage           string
	UserRunnerImage          string
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
	Command                  string
	OktetoIsPreviewEnv       string
	Caches                   []string
}

// NewRunner creates a new Runner for remote
func NewRunner(ioCtrl *io.Controller, builder Builder) *Runner {
	fs := afero.NewOsFs()
	return &Runner{
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewOsWorkingDirectoryCtrl(),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		useInternalNetwork:   !buildCmd.IsDepotEnabled(),
		ioCtrl:               ioCtrl,
		builder:              builder,
		oktetoClientProvider: okteto.NewOktetoClientProvider(),
	}
}

// Run This function is the one in charge of creating the Dockerfile needed to run
// remote execution of commands like destroy and deploy and triggers the build
func (r *Runner) Run(ctx context.Context, params *Params) error {
	home, err := homedir.Dir()
	if err != nil {
		return err
	}

	sc, err := r.fetchClusterMetadata(ctx)
	if err != nil {
		return err
	}

	if params.BaseImage == "" {
		params.BaseImage = sc.PipelineRunnerImage
	}

	cwd, err := r.getOriginalCWD(params.ManifestPathFlag)
	if err != nil {
		return err
	}

	tmpDir, err := r.temporalCtrl.Create()
	if err != nil {
		return err
	}

	dockerfile, err := r.createDockerfile(tmpDir, params)
	if err != nil {
		return err
	}

	defer func() {
		if err := r.fs.Remove(dockerfile); err != nil {
			oktetoLog.Infof("error removing dockerfile: %s", err)
		}
	}()

	var buildCtx string
	if params.ContextAbsolutePathOverride != "" {
		buildCtx = params.ContextAbsolutePathOverride
	} else {
		buildCtx = r.getContextPath(cwd, params.ManifestPathFlag)
	}

	buildInfo := &build.Info{
		Dockerfile: dockerfile,
		Context:    buildCtx,
	}

	// undo modification of CWD for Build command
	if err := r.workingDirectoryCtrl.Change(cwd); err != nil {
		return err
	}

	cacheKey := params.CacheInvalidationKey
	if cacheKey == "" {
		randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
		if err != nil {
			return err
		}
		cacheKey = strconv.Itoa(int(randomNumber.Int64()))
	}

	b, err := yaml.Marshal(params.Deployable)
	if err != nil {
		return err
	}

	outputMode := buildCmd.DeployOutputModeOnBuild
	if params.Command == DestroyCommand {
		outputMode = buildCmd.DestroyOutputModeOnBuild

	}
	buildOptions := buildCmd.OptsFromBuildInfoForRemoteDeploy(buildInfo, &types.BuildOptions{OutputMode: outputMode})
	buildOptions.Manifest = params.Manifest
	buildOptions.BuildArgs = append(
		buildOptions.BuildArgs,
		fmt.Sprintf("%s=%s", model.OktetoContextEnvVar, okteto.GetContext().Name),
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.GetContext().Namespace),
		fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, okteto.GetContext().Token),
		fmt.Sprintf("%s=%s", model.OktetoActionNameEnvVar, os.Getenv(model.OktetoActionNameEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitCommitEnvVar, os.Getenv(constants.OktetoGitCommitEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitBranchEnvVar, os.Getenv(constants.OktetoGitBranchEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoTlsCertBase64EnvVar, base64.StdEncoding.EncodeToString(sc.Certificate)),
		fmt.Sprintf("%s=%s", constants.OktetoInvalidateCacheEnvVar, cacheKey),
		fmt.Sprintf("%s=%s", constants.OktetoDeployableEnvVar, base64.StdEncoding.EncodeToString(b)),
		fmt.Sprintf("%s=%s", model.GithubRepositoryEnvVar, os.Getenv(model.GithubRepositoryEnvVar)),
		fmt.Sprintf("%s=%s", model.OktetoRegistryURLEnvVar, os.Getenv(model.OktetoRegistryURLEnvVar)),
		fmt.Sprintf("%s=%s", model.OktetoBuildkitHostURLEnvVar, os.Getenv(model.OktetoBuildkitHostURLEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoIsPreviewEnvVar, os.Getenv(constants.OktetoIsPreviewEnvVar)),
	)

	if r.useInternalNetwork {
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

	sshSock := os.Getenv(r.sshAuthSockEnvvar)
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
		knownHostsPath := params.KnownHostsPath
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
	return r.builder.Run(ctx, buildOptions, r.ioCtrl)
}

// getOriginalCWD returns the original cwd from the manifest path
func (r *Runner) getOriginalCWD(manifestPath string) (string, error) {
	cwd, err := r.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}
	manifestPathDir := filepath.Dir(filepath.Clean(fmt.Sprintf("/%s", manifestPath)))
	return strings.TrimSuffix(cwd, manifestPathDir), nil
}

// createDockerfile renders the template of the Dockerfile and creates the temporary file
func (r *Runner) createDockerfile(tmpDir string, params *Params) (string, error) {
	cwd, err := r.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}

	tmpl := template.
		Must(template.New(params.TemplateName).
			Funcs(template.FuncMap{"join": strings.Join}).
			Parse(dockerfileTemplate))

	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:           getOktetoCLIVersion(config.VersionString),
		UserRunnerImage:          params.BaseImage,
		RemoteDeployEnvVar:       constants.OktetoDeployRemote,
		ContextArgName:           model.OktetoContextEnvVar,
		OktetoBuildEnvVars:       params.BuildEnvVars,
		NamespaceArgName:         model.OktetoNamespaceEnvVar,
		TlsCertBase64ArgName:     constants.OktetoTlsCertBase64EnvVar,
		InternalServerName:       constants.OktetoInternalServerNameEnvVar,
		TokenArgName:             model.OktetoTokenEnvVar,
		ActionNameArgName:        model.OktetoActionNameEnvVar,
		GitCommitArgName:         constants.OktetoGitCommitEnvVar,
		GitBranchArgName:         constants.OktetoGitBranchEnvVar,
		InvalidateCacheArgName:   constants.OktetoInvalidateCacheEnvVar,
		CommandFlags:             strings.Join(params.CommandFlags, " "),
		OktetoDeployable:         constants.OktetoDeployableEnvVar,
		GitHubRepositoryArgName:  model.GithubRepositoryEnvVar,
		BuildKitHostArgName:      model.OktetoBuildkitHostURLEnvVar,
		OktetoRegistryURLArgName: model.OktetoRegistryURLEnvVar,
		OktetoDependencyEnvVars:  params.DependenciesEnvVars,
		Command:                  params.Command,
		OktetoIsPreviewEnv:       constants.OktetoIsPreviewEnvVar,
		Caches:                   params.Caches,
	}

	dockerfile, err := r.fs.Create(filepath.Join(tmpDir, params.DockerfileName))
	if err != nil {
		return "", err
	}

	// if we do not create a .dockerignore (with or without content) used to create
	// the remote executor, we would use the one located in root (the one used to
	// build the services) so we would create a remote executor without certain files
	// necessary for the later deployment which would cause an error when deploying
	// remotely due to the lack of these files.
	if err := CreateDockerignoreFileWithFilesystem(cwd, tmpDir, r.fs); err != nil {
		return "", err
	}

	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return "", err
	}
	return dockerfile.Name(), nil
}

func (r *Runner) getContextPath(cwd, manifestPath string) string {
	if manifestPath == "" {
		return cwd
	}

	path := manifestPath
	if !filepath.IsAbs(manifestPath) {
		path = filepath.Join(cwd, manifestPath)
	}
	fInfo, err := r.fs.Stat(path)
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

func (r *Runner) fetchClusterMetadata(ctx context.Context) (*types.ClusterMetadata, error) {
	c, err := r.oktetoClientProvider.Provide()
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

// CreateDockerignoreFileWithFilesystem creates a .dockerignore file in the tmpDir with the content of the
// .dockerignore file in the cwd
func CreateDockerignoreFileWithFilesystem(cwd, tmpDir string, fs afero.Fs) error {
	dockerignoreContent := []byte(``)
	dockerignoreFilePath := filepath.Join(cwd, oktetoDockerignoreName)
	if _, err := fs.Stat(dockerignoreFilePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

	} else {
		dockerignoreContent, err = afero.ReadFile(fs, dockerignoreFilePath)
		if err != nil {
			return err
		}
	}

	// write the content into the .dockerignore used for building the remote image
	filename := fmt.Sprintf("%s/%s", tmpDir, ".dockerignore")

	return afero.WriteFile(fs, filename, dockerignoreContent, 0600)
}

// getOktetoCLIVersion gets the CLI version to be used in the Dockerfile to extract the CLI binaries
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

// getExtraHosts returns the extra hosts to be added to the Dockerfile
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

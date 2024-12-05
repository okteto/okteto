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
	"strconv"
	"strings"
	"text/template"

	"github.com/docker/docker/pkg/namesgenerator"
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

USER 0
ENV PATH="${PATH}:/okteto/bin"
WORKDIR /okteto
RUN chown -R $(id -u):$(id -g) /okteto || (echo '{"level":"error", "stage": "remote deploy", "message":"The /okteto directory or its subdirectories have incorrect user/group ownership. Check your \"deploy.image\" container definition or contact your instance admin."}' && exit 1)
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
RUN mkdir -p /okteto/.ssl/certs && echo "${{ .TlsCertBase64ArgName }}" | base64 -d > /okteto/.ssl/certs/okteto.crt
ENV SSL_CERT_DIR=/etc/ssl/certs:/okteto/.ssl/certs

WORKDIR /okteto/src
COPY --chmod=0777 . /okteto/src

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}}={{$val}}
{{end}}
{{range $key, $val := .OktetoCommandSpecificEnvVars }}
ENV {{$key}}={{$val}}
{{end}}
{{range $key, $val := .OktetoDependencyEnvVars }}
ENV {{$key}}={{$val}}
{{end}}

ENV {{ .SSHAgentHostnameArgName }}="{{ .SSHAgentHostname }}"
ENV {{ .SSHAgentPortArgName }}="{{ .SSHAgentPort }}"
ENV {{ .SSHAgentSocketArgName }}="{{ .SSHAgentSocket }}"

ARG {{ .GitCommitArgName }}
ARG {{ .GitBranchArgName }}
ARG {{ .InvalidateCacheArgName }}

RUN echo "${{ .InvalidateCacheArgName }}" > /okteto/.oktetocachekey
RUN okteto registrytoken install --force --log-output=json

{{ range $key, $val := .OktetoExecutionEnvVars}}
ENV {{$key}}={{$val}}
{{ end }}

RUN \
  {{range $key, $path := .Caches }}--mount=type=cache,target={{$path}},sharing=private {{end}}\
  --mount=type=secret,id=known_hosts \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts $HOME/.ssh/known_hosts" >> $HOME/.ssh/config && \
{{ if .SSHAgentHostname }}  export SSH_AUTH_SOCK={{ .SSHAgentSocket }} && \{{ end }}
  /okteto/bin/okteto remote-run {{ .Command }} --log-output=json --server-name="${{ .InternalServerName }}" {{ .CommandFlags }}{{ if eq .Command "test" }} || true{{ end }}

{{range $key, $artifact := .Artifacts }}
RUN if [ -e /okteto/src/{{$artifact.Path}} ]; then \
    mkdir -p $(dirname /okteto/artifacts/{{$artifact.Destination}}) && \
    cp -r /okteto/src/{{$artifact.Path}} /okteto/artifacts/{{$artifact.Destination}}; \
  fi
{{end}}

FROM scratch
{{ if gt (len .Artifacts) 0 -}}
COPY --from=runner /okteto/artifacts/ /
{{ else -}}
COPY --from=runner /okteto/.oktetocachekey /okteto/.oktetocachekey
{{ end }}
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

type socketNameGenerator func() string

// Runner struct in charge of creating the Dockerfile for remote execution
// of commands like deploy and destroy and running the build
type Runner struct {
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
	builder              Builder
	oktetoClientProvider OktetoClientProvider
	ioCtrl               *io.Controller
	getEnviron           func() []string
	generateSocketName   socketNameGenerator
	useInternalNetwork   bool
}

// Params struct to pass the necessary parameters to create the Dockerfile
type Params struct {
	BuildEnvVars                 map[string]string
	DependenciesEnvVars          map[string]string
	OktetoCommandSpecificEnvVars map[string]string
	ExecutionEnvVars             map[string]string
	Manifest                     *model.Manifest
	Command                      string
	TemplateName                 string
	DockerfileName               string
	BaseImage                    string
	// ContextAbsolutePathOverride is the absolute path for the build context. Optional.
	// If this values is not defined it will default to the folder location of the
	// okteto manifest which is resolved through params.ManifestPathFlag
	ContextAbsolutePathOverride string
	ManifestPathFlag            string
	// SSHAgentHostname hostname of the ssh-agent service
	SSHAgentHostname string
	// SSHAgentPort port of the ssh-agent service
	SSHAgentPort string
	Deployable   deployable.Entity
	CommandFlags []string
	Caches       []string
	Hosts        []model.Host
	// IgnoreRules are the ignoring rules added to this build execution.
	// Rules follow the .dockerignore syntax as defined in:
	// https://docs.docker.com/build/building/context/#syntax
	IgnoreRules []string
	// Artifacts are the files and or folder to export from this build operation.
	// They are the path INSIDE the build container relative to /okteto/src. They
	// will be exported to "{context_dir}/{artifact}"
	Artifacts []model.Artifact
	// UseOktetoDeployIgnoreFile if enabled loads the docker ignore file from an
	// .oktetodeployignore file. Disabled by default
	UseOktetoDeployIgnoreFile bool

	NoCache bool
}

// dockerfileTemplateProperties internal struct with the information needed by the Dockerfile template
type dockerfileTemplateProperties struct {
	OktetoCLIImage               string
	UserRunnerImage              string
	RemoteDeployEnvVar           string
	OktetoBuildEnvVars           map[string]string
	OktetoDependencyEnvVars      map[string]string
	OktetoPrefixEnvVars          map[string]string
	OktetoCommandSpecificEnvVars map[string]string
	OktetoExecutionEnvVars       map[string]string
	ContextArgName               string
	NamespaceArgName             string
	TokenArgName                 string
	TlsCertBase64ArgName         string
	InternalServerName           string
	ActionNameArgName            string
	GitCommitArgName             string
	GitBranchArgName             string
	InvalidateCacheArgName       string
	CommandFlags                 string
	OktetoDeployable             string
	GitHubRepositoryArgName      string
	BuildKitHostArgName          string
	OktetoRegistryURLArgName     string
	Command                      string
	SSHAgentHostnameArgName      string
	SSHAgentHostname             string
	SSHAgentPortArgName          string
	SSHAgentPort                 string
	SSHAgentSocketArgName        string
	SSHAgentSocket               string
	Caches                       []string
	Artifacts                    []model.Artifact
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
		getEnviron:           os.Environ,
		generateSocketName:   generateRandomSocketNameString,
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

	cwd, err := GetOriginalCWD(r.workingDirectoryCtrl, params.ManifestPathFlag)
	if err != nil {
		return err
	}

	tmpDir, err := r.temporalCtrl.Create()
	if err != nil {
		return err
	}

	params.SSHAgentHostname = sc.SSHAgentHostname
	params.SSHAgentPort = sc.SSHAgentPort

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

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}
	cacheKey := strconv.Itoa(int(randomNumber.Int64()))

	b, err := yaml.Marshal(params.Deployable)
	if err != nil {
		return err
	}

	var outputMode string
	switch params.Command {
	case TestCommand:
		outputMode = buildCmd.TestOutputModeOnBuild
	case DeployCommand:
		outputMode = buildCmd.DeployOutputModeOnBuild
	case DestroyCommand:
		outputMode = buildCmd.DestroyOutputModeOnBuild
	default:
		outputMode = buildCmd.DeployOutputModeOnBuild
	}

	buildOptions := buildCmd.OptsFromBuildInfoForRemoteDeploy(buildInfo, &types.BuildOptions{OutputMode: outputMode, NoCache: params.NoCache})
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

		if sc.SSHAgentHostname != "" {
			buildOptions.ExtraHosts = append(buildOptions.ExtraHosts, types.HostMap{Hostname: sc.SSHAgentHostname, IP: sc.SSHAgentInternalIP})
		}
	}

	buildOptions.ExtraHosts = addDefinedHosts(buildOptions.ExtraHosts, params.Hosts)

	// TODO: check if ~/.ssh/config exists and has UserKnownHostsFile defined
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); err != nil {
		oktetoLog.Debugf("Not know_hosts file. Error reading file: %s", err.Error())
	} else {
		oktetoLog.Debugf("reading known hosts from %s", knownHostsPath)
		buildOptions.Secrets = append(buildOptions.Secrets, fmt.Sprintf("id=known_hosts,src=%s", knownHostsPath))
	}

	if len(params.Artifacts) > 0 {
		buildOptions.LocalOutputPath = buildCtx
	}
	r.ioCtrl.Logger().Infof("Executing remote with the following base image: %s", params.BaseImage)

	// we need to call Run() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	return r.builder.Run(ctx, buildOptions, r.ioCtrl)
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
		OktetoCLIImage:               config.NewImageConfig(r.ioCtrl).GetRemoteImage(config.VersionString),
		UserRunnerImage:              params.BaseImage,
		RemoteDeployEnvVar:           constants.OktetoDeployRemote,
		ContextArgName:               model.OktetoContextEnvVar,
		OktetoBuildEnvVars:           params.BuildEnvVars,
		NamespaceArgName:             model.OktetoNamespaceEnvVar,
		TlsCertBase64ArgName:         constants.OktetoTlsCertBase64EnvVar,
		InternalServerName:           constants.OktetoInternalServerNameEnvVar,
		TokenArgName:                 model.OktetoTokenEnvVar,
		ActionNameArgName:            model.OktetoActionNameEnvVar,
		GitCommitArgName:             constants.OktetoGitCommitEnvVar,
		GitBranchArgName:             constants.OktetoGitBranchEnvVar,
		InvalidateCacheArgName:       constants.OktetoInvalidateCacheEnvVar,
		CommandFlags:                 strings.Join(params.CommandFlags, " "),
		OktetoDeployable:             constants.OktetoDeployableEnvVar,
		GitHubRepositoryArgName:      model.GithubRepositoryEnvVar,
		BuildKitHostArgName:          model.OktetoBuildkitHostURLEnvVar,
		OktetoRegistryURLArgName:     model.OktetoRegistryURLEnvVar,
		OktetoDependencyEnvVars:      params.DependenciesEnvVars,
		OktetoPrefixEnvVars:          getOktetoPrefixEnvVars(r.getEnviron()),
		OktetoCommandSpecificEnvVars: params.OktetoCommandSpecificEnvVars,
		OktetoExecutionEnvVars:       params.ExecutionEnvVars,
		Command:                      params.Command,
		Caches:                       params.Caches,
		Artifacts:                    params.Artifacts,
		SSHAgentHostname:             params.SSHAgentHostname,
		SSHAgentHostnameArgName:      constants.OktetoSshAgentHostnameEnvVar,
		SSHAgentPort:                 params.SSHAgentPort,
		SSHAgentPortArgName:          constants.OktetoSshAgentPortEnvVar,
		SSHAgentSocketArgName:        constants.OktetoSshAgentSocketEnvVar,
		SSHAgentSocket:               r.generateSocketName(),
	}

	dockerfileSyntax.prepareEnvVars()

	dockerfile, err := r.fs.Create(filepath.Join(tmpDir, params.DockerfileName))
	if err != nil {
		return "", err
	}

	// if we do not create a .dockerignore (with or without content) used to create
	// the remote executor, we would use the one located in root (the one used to
	// build the services) so we would create a remote executor without certain files
	// necessary for the later deployment which would cause an error when deploying
	// remotely due to the lack of these files.
	if err := createDockerignoreFileWithFilesystem(cwd, tmpDir, params.IgnoreRules, params.UseOktetoDeployIgnoreFile, r.fs); err != nil {
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

// createDockerignoreFileWithFilesystem creates a .dockerignore file in the tmpDir with the content of the
// .dockerignore file in the cwd
func createDockerignoreFileWithFilesystem(cwd, tmpDir string, rules []string, useOktetoDeployIgnoreFile bool, fs afero.Fs) error {
	dockerignoreContent := []byte(``)

	if useOktetoDeployIgnoreFile {
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
			oktetoLog.Warning("Ignoring files through %s is deprecated and will be removed in future versions. Please use .oktetoignore. More info here: https://www.okteto.com/docs/core/remote-execution/#ignoring-files", oktetoDockerignoreName)
			dockerignoreContent = append(dockerignoreContent, []byte("\n")...)
		}
	}

	// write the content into the .dockerignore used for building the remote image
	filename := fmt.Sprintf("%s/%s", tmpDir, ".dockerignore")

	// append rules to the contents of the .oktetodeployignore rules
	for _, rule := range rules {
		dockerignoreContent = append(dockerignoreContent, []byte(rule)...)
		dockerignoreContent = append(dockerignoreContent, []byte("\n")...)
	}
	if len(dockerignoreContent) == 0 {
		dockerignoreContent = []byte("# Okteto docker ignore\n")
	}
	return afero.WriteFile(fs, filename, dockerignoreContent, 0600)
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

func addDefinedHosts(extraHosts []types.HostMap, definedHosts []model.Host) []types.HostMap {
	for _, host := range definedHosts {
		extraHosts = append(extraHosts, types.HostMap{Hostname: host.Hostname, IP: host.IP})
	}
	return extraHosts
}

// GetOriginalCWD returns the original cwd from the manifest path
func GetOriginalCWD(workingDirectoryCtrl filesystem.WorkingDirectoryInterface, manifestPath string) (string, error) {
	cwd, err := workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}
	manifestPathDir := filepath.Dir(filepath.Clean(fmt.Sprintf("/%s", manifestPath)))
	return strings.TrimSuffix(cwd, manifestPathDir), nil
}

func getOktetoPrefixEnvVars(environ []string) map[string]string {
	bannedOktetoEnvVars := map[string]bool{
		"OKTETO_HOME":   true,
		"OKTETO_FOLDER": true,
		"OKTETO_PATH":   true,
	}
	prefixEnvVars := make(map[string]string)
	envFormatParts := 2
	for _, v := range environ {
		result := strings.SplitN(v, "=", envFormatParts)
		if len(result) != envFormatParts {
			continue
		}
		key := result[0]
		if bannedOktetoEnvVars[key] {
			continue
		}
		if strings.HasPrefix(key, "OKTETO_") {
			value := result[1]
			prefixEnvVars[key] = value
		}
	}
	return prefixEnvVars
}

func (dfP *dockerfileTemplateProperties) prepareEnvVars() {
	for k, v := range dfP.OktetoBuildEnvVars {
		dfP.OktetoBuildEnvVars[k] = formatEnvVarValueForDocker(v)
	}
	for k, v := range dfP.OktetoDependencyEnvVars {
		dfP.OktetoDependencyEnvVars[k] = formatEnvVarValueForDocker(v)
	}
	for k, v := range dfP.OktetoPrefixEnvVars {
		dfP.OktetoPrefixEnvVars[k] = formatEnvVarValueForDocker(v)
	}
	for k, v := range dfP.OktetoExecutionEnvVars {
		dfP.OktetoExecutionEnvVars[k] = formatEnvVarValueForDocker(v)
	}
}

func formatEnvVarValueForDocker(value string) string {
	lines := strings.Split(value, "\n")
	var result strings.Builder

	result.WriteString("\"")
	for i, line := range lines {
		result.WriteString(line)
		if i < len(lines)-1 {
			result.WriteString("\\n")
		}
	}
	result.WriteString("\"")

	return result.String()
}

func generateRandomSocketNameString() string {
	random := strings.ReplaceAll(namesgenerator.GetRandomName(-1), "_", "-")
	return fmt.Sprintf("/tmp/okteto-%s.sock", random)
}

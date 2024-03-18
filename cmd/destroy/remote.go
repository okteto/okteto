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

package destroy

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
	builder "github.com/okteto/okteto/cmd/build"
	remoteBuild "github.com/okteto/okteto/cmd/build/remote"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	templateName           = "destroy-dockerfile"
	dockerfileTemporalNane = "Dockerfile.destroy"
	dockerfileTemplate     = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM {{ .UserDestroyImage }} as deploy

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

ARG {{ .GitCommitArgName }}
ARG {{ .GitBranchArgName }}
ARG {{ .InvalidateCacheArgName }}

RUN okteto registrytoken install --force --log-output=json

RUN --mount=type=secret,id=known_hosts --mount=id=remote,type=ssh \
  mkdir -p $HOME/.ssh && echo "UserKnownHostsFile=/run/secrets/known_hosts" >> $HOME/.ssh/config && \
  okteto destroy --log-output=json --server-name="${{ .InternalServerName }}" {{ .DestroyFlags }}
`
)

type dockerfileTemplateProperties struct {
	OktetoCLIImage         string
	UserDestroyImage       string
	RemoteDeployEnvVar     string
	ContextArgName         string
	NamespaceArgName       string
	TokenArgName           string
	TlsCertBase64ArgName   string
	InternalServerName     string
	ActionNameArgName      string
	GitCommitArgName       string
	GitBranchArgName       string
	InvalidateCacheArgName string
	DestroyFlags           string
}

type remoteDestroyCommand struct {
	builder              builder.Builder
	destroyImage         string
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
	manifest             *model.Manifest
	registry             remoteBuild.OktetoRegistryInterface
	clusterMetadata      func(context.Context) (*types.ClusterMetadata, error)

	// sshAuthSockEnvvar is the default for SSH_AUTH_SOCK. Provided mostly for testing
	sshAuthSockEnvvar string

	// knownHostsPath  is the default known_hosts file path. Provided mostly for testing
	knownHostsPath string
}

func newRemoteDestroyer(manifest *model.Manifest, ioCtrl *io.IOController) *remoteDestroyCommand {
	fs := afero.NewOsFs()
	builder := remoteBuild.NewBuilderFromScratch(ioCtrl)
	if manifest.Destroy == nil {
		manifest.Destroy = &model.DestroyInfo{}
	}
	return &remoteDestroyCommand{
		builder:              builder,
		destroyImage:         manifest.Destroy.Image,
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewOsWorkingDirectoryCtrl(),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
		manifest:             manifest,
		registry:             builder.Registry,
		clusterMetadata:      fetchClusterMetadata,
	}
}

func (rd *remoteDestroyCommand) destroy(ctx context.Context, opts *Options) error {
	home, err := homedir.Dir()
	if err != nil {
		return err
	}

	sc, err := rd.clusterMetadata(ctx)
	if err != nil {
		return err
	}

	if rd.destroyImage == "" {
		rd.destroyImage = sc.PipelineRunnerImage
	}

	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return err
	}

	tmpDir, err := rd.temporalCtrl.Create()
	if err != nil {
		return err
	}

	dockerfile, err := rd.createDockerfile(tmpDir, opts)
	if err != nil {
		return err
	}

	defer func() {
		if err := rd.fs.Remove(dockerfile); err != nil {
			oktetoLog.Infof("error removing dockerfile: %w", err)
		}
	}()

	buildInfo := &build.Info{
		Dockerfile: dockerfile,
		Context:    rd.getContextPath(cwd, opts.ManifestPath),
	}

	// undo modification of CWD for Build command
	if err := rd.workingDirectoryCtrl.Change(cwd); err != nil {
		return err
	}

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return err
	}

	buildOptions := buildCmd.OptsFromBuildInfoForRemoteDeploy(buildInfo, &types.BuildOptions{Path: cwd, OutputMode: "destroy"})
	buildOptions.Manifest = rd.manifest
	buildOptions.BuildArgs = append(
		buildOptions.BuildArgs,
		fmt.Sprintf("%s=%s", model.OktetoContextEnvVar, okteto.Context().Name),
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.Context().Namespace),
		fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, okteto.Context().Token),
		fmt.Sprintf("%s=%s", constants.OktetoTlsCertBase64EnvVar, base64.StdEncoding.EncodeToString(sc.Certificate)),
		fmt.Sprintf("%s=%s", constants.OktetoInternalServerNameEnvVar, sc.ServerName),
		fmt.Sprintf("%s=%s", model.OktetoActionNameEnvVar, os.Getenv(model.OktetoActionNameEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitCommitEnvVar, os.Getenv(constants.OktetoGitCommitEnvVar)),
		fmt.Sprintf("%s=%s", constants.OktetoGitBranchEnvVar, os.Getenv(constants.OktetoGitBranchEnvVar)),
		fmt.Sprintf("%s=%d", constants.OktetoInvalidateCacheEnvVar, int(randomNumber.Int64())),
	)

	if sc.ServerName != "" {
		registryUrl := okteto.Context().Registry
		subdomain := strings.TrimPrefix(registryUrl, "registry.")
		ip, _, err := net.SplitHostPort(sc.ServerName)
		if err != nil {
			return fmt.Errorf("failed to parse server name network address: %w", err)
		}

		buildOptions.ExtraHosts = getExtraHosts(registryUrl, subdomain, ip, *sc)
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
	if err := rd.builder.Build(ctx, buildOptions); err != nil {
		var cmdErr buildCmd.OktetoCommandErr
		if errors.As(err, &cmdErr) {
			oktetoLog.SetStage(cmdErr.Stage)
			return oktetoErrors.UserError{
				E: fmt.Errorf("error during development environment deployment: %w", cmdErr.Err),
			}
		}
		oktetoLog.SetStage("remote deploy")
		var userErr oktetoErrors.UserError
		if errors.As(err, &userErr) {
			return userErr
		}
		return oktetoErrors.UserError{
			E: fmt.Errorf("error during destroy of the development environment: %w", err),
		}
	}
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return nil
}

func (rd *remoteDestroyCommand) createDockerfile(tmpDir string, opts *Options) (string, error) {
	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}

	destroyFlags, err := getDestroyFlags(opts, cwd, rd.fs)
	if err != nil {
		return "", err
	}

	tmpl := template.Must(template.New(templateName).Parse(dockerfileTemplate))
	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:         getOktetoCLIVersion(config.VersionString),
		UserDestroyImage:       rd.destroyImage,
		RemoteDeployEnvVar:     constants.OktetoDeployRemote,
		ContextArgName:         model.OktetoContextEnvVar,
		NamespaceArgName:       model.OktetoNamespaceEnvVar,
		TokenArgName:           model.OktetoTokenEnvVar,
		TlsCertBase64ArgName:   constants.OktetoTlsCertBase64EnvVar,
		InternalServerName:     constants.OktetoInternalServerNameEnvVar,
		ActionNameArgName:      model.OktetoActionNameEnvVar,
		GitCommitArgName:       constants.OktetoGitCommitEnvVar,
		GitBranchArgName:       constants.OktetoGitBranchEnvVar,
		InvalidateCacheArgName: constants.OktetoInvalidateCacheEnvVar,
		DestroyFlags:           strings.Join(destroyFlags, " "),
	}

	dockerfile, err := rd.fs.Create(filepath.Join(tmpDir, dockerfileTemporalNane))
	if err != nil {
		return "", err
	}

	cleanManifestPath := filesystem.CleanManifestPath(opts.ManifestPath)
	if err = remote.CreateDockerignoreFileWithFilesystem(cwd, tmpDir, cleanManifestPath, rd.fs); err != nil {
		return "", err
	}

	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return "", err
	}
	return dockerfile.Name(), nil

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

func getDestroyFlags(opts *Options, cwd string, fs afero.Fs) ([]string, error) {
	var destroyFlags []string

	if opts.Name != "" {
		destroyFlags = append(destroyFlags, fmt.Sprintf("--name \"%s\"", opts.Name))
	}

	if opts.Namespace != "" {
		destroyFlags = append(destroyFlags, fmt.Sprintf("--namespace %s", opts.Namespace))
	}

	// we calculate the absolute path of the manifest file to be able to check whether the provided path is a directory
	// in that case, the --file (or -f) flag can be omitted as the autodiscovery will take place
	absManifestFilePath := filepath.Join(cwd, filepath.Clean(opts.ManifestPath))
	isManifestFilePathDir, err := afero.IsDir(fs, absManifestFilePath)
	if err != nil {
		return nil, err
	}
	if opts.ManifestPathFlag != "" && !isManifestFilePathDir {
		cleanManifestPath := filesystem.CleanManifestPath(opts.ManifestPathFlag)
		destroyFlags = append(destroyFlags, fmt.Sprintf("--file %s", cleanManifestPath))
	}

	if opts.DestroyVolumes {
		destroyFlags = append(destroyFlags, "--volumes")
	}

	if opts.ForceDestroy {
		destroyFlags = append(destroyFlags, "--force-destroy")
	}

	return destroyFlags, nil
}

func getOktetoCLIVersion(versionString string) string {
	var version string
	if match, err := regexp.MatchString(`\d+\.\d+\.\d+`, versionString); match {
		version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, versionString)
	} else {
		oktetoLog.Infof("invalid okteto CLI version %s: %s", versionString, err)
		oktetoLog.Info("using latest okteto CLI image")
		remoteOktetoImage := os.Getenv(constants.OktetoDeployRemoteImage)
		if remoteOktetoImage != "" {
			version = remoteOktetoImage
		} else {
			version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, "latest")
		}
	}

	return version
}

func fetchClusterMetadata(ctx context.Context) (*types.ClusterMetadata, error) {
	cp := okteto.NewOktetoClientProvider()
	c, err := cp.Provide()
	if err != nil {
		return nil, fmt.Errorf("failed to provide okteto client for fetching certs: %w", err)
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

func (rd *remoteDestroyCommand) getContextPath(cwd, manifestPath string) string {
	if manifestPath == "" {
		return cwd
	}

	path := manifestPath
	if !filepath.IsAbs(manifestPath) {
		path = filepath.Join(cwd, manifestPath)
	}
	if strings.HasSuffix(strings.TrimSuffix(path, string(filepath.Separator)), ".okteto") {
		return filepath.Dir(path)
	}

	possibleCtx := filepath.Dir(path)

	if strings.HasSuffix(possibleCtx, ".okteto") {
		return filepath.Dir(possibleCtx)
	}

	fInfo, err := rd.fs.Stat(path)
	if err != nil {
		oktetoLog.Infof("error getting file info: %s", err)
		return cwd
	}
	if fInfo.IsDir() {
		return path
	}
	return possibleCtx
}

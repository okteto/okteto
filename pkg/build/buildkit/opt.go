// Copyright 2024 The Okteto Authors
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

package buildkit

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	dockerConfig "github.com/docker/cli/cli/config"
	"github.com/moby/buildkit/client"
	buildctl "github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// PermissionsOwnerOnly is the permission for the secret temp folder
	PermissionsOwnerOnly = 0700
)

// SolveOptBuilder is a builder for SolveOpt
type SolveOptBuilder struct {
	logger                *io.Controller
	imageCtrl             registry.ImageCtrl
	reg                   registry.OktetoRegistry
	okCtx                 OktetoContextInterface
	fs                    afero.Fs
	secretTempFolder      string
	dockerFrontendVersion uint64
}

type ClientFactoryIface interface {
	GetBuildkitClient(ctx context.Context) (*client.Client, error)
}

// OktetoContextInterface is an interface to interact with the okteto context
type OktetoContextInterface interface {
	GetCurrentName() string
	GetNamespace() string
	GetGlobalNamespace() string
	GetCurrentBuilder() string
	GetCurrentCertStr() string
	GetCurrentCfg() *clientcmdapi.Config
	GetCurrentToken() string
	GetCurrentUser() string
	ExistsContext() bool
	IsOktetoCluster() bool
	IsInsecure() bool
	UseContextByBuilder()
	GetTokenByContextName(name string) (string, error)
	GetRegistryURL() string
}

// NewSolveOptBuilder creates a new SolveOptBuilder
func NewSolveOptBuilder(ctx context.Context, clientFactory ClientFactoryIface, reg registry.OktetoRegistry, okCtx OktetoContextInterface, fs afero.Fs, logger *io.Controller) (*SolveOptBuilder, error) {
	buildkitClient, err := clientFactory.GetBuildkitClient(ctx)
	if err != nil {
		logger.Infof("failed to get buildkit client: %s", err)
		return nil, err
	}
	info, err := buildkitClient.Info(ctx)
	if err != nil {
		logger.Infof("failed to get buildkit info: %s", err)
		return nil, err
	}
	buildkitVersion, err := semver.NewVersion(info.BuildkitVersion.Version)
	if err != nil {
		logger.Infof("failed to parse buildkit version: %s", err)
		return nil, err
	}
	// We substract 6 as it's the difference between the buildkit version and the docker frontend version
	dockerfileFrontendVersion := buildkitVersion.Minor() - 6

	secretTempFolder, err := getSecretTempFolder(fs)
	if err != nil {
		return nil, err
	}

	return &SolveOptBuilder{
		logger:                logger,
		dockerFrontendVersion: dockerfileFrontendVersion,
		imageCtrl:             registry.NewImageCtrl(okCtx),
		reg:                   reg,
		fs:                    fs,
		secretTempFolder:      secretTempFolder,
		okCtx:                 okCtx,
	}, nil
}

// Build creates a new SolveOpt
func (b *SolveOptBuilder) Build(buildOptions *types.BuildOptions) (*client.SolveOpt, error) {
	// check that the images are in the right format
	if err := b.validateTags(buildOptions.Tag); err != nil {
		return nil, err
	}

	// extend okteto.dev images into the extended syntax
	buildOptions.Tag = b.extendRegistries(buildOptions.Tag)
	for i := range buildOptions.CacheFrom {
		buildOptions.CacheFrom[i] = b.extendRegistries(buildOptions.CacheFrom[i])
	}
	for i := range buildOptions.ExportCache {
		buildOptions.ExportCache[i] = b.extendRegistries(buildOptions.ExportCache[i])
	}

	// inject secrets to buildkit from temp folder
	if err := b.replaceSecretsSourceEnvWithTempFile(buildOptions); err != nil {
		return nil, fmt.Errorf("%w: secret should have the format 'id=mysecret,src=/local/secret'", err)
	}

	var localDirs map[string]string
	var frontendAttrs map[string]string

	if uri, err := url.ParseRequestURI(buildOptions.Path); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {

		if buildOptions.File == "" {
			buildOptions.File = filepath.Join(buildOptions.Path, "Dockerfile")
		}
		if _, err := b.fs.Stat(buildOptions.File); os.IsNotExist(err) {
			return nil, fmt.Errorf("file '%s' not found: %w", buildOptions.File, err)
		}
		localDirs = map[string]string{
			"context":    buildOptions.Path,
			"dockerfile": filepath.Dir(buildOptions.File),
		}
		frontendAttrs = map[string]string{
			"filename": filepath.Base(buildOptions.File),
		}
	} else {
		frontendAttrs = map[string]string{
			"context": buildOptions.Path,
		}
	}

	if buildOptions.Platform != "" {
		frontendAttrs["platform"] = buildOptions.Platform
	}
	if buildOptions.Target != "" {
		frontendAttrs["target"] = buildOptions.Target
	}
	if buildOptions.NoCache {
		frontendAttrs["no-cache"] = ""
	}

	frontend := NewFrontendRetriever(b.dockerFrontendVersion, b.logger).GetFrontend(buildOptions)
	if frontend.Image != "" {
		frontendAttrs["source"] = frontend.Image
	}

	if len(buildOptions.ExtraHosts) > 0 {
		hosts := ""
		for _, eh := range buildOptions.ExtraHosts {
			hosts += fmt.Sprintf("%s=%s,", eh.Hostname, eh.IP)
		}
		frontendAttrs["add-hosts"] = strings.TrimSuffix(hosts, ",")
	}

	maxArgFormatParts := 2
	for _, buildArg := range buildOptions.BuildArgs {
		kv := strings.SplitN(buildArg, "=", maxArgFormatParts)
		if len(kv) != maxArgFormatParts {
			return nil, fmt.Errorf("invalid build-arg value %s", buildArg)
		}
		frontendAttrs["build-arg:"+kv[0]] = kv[1]
	}
	attachable := []session.Attachable{}
	if b.okCtx.IsOktetoCluster() {
		apCtx := &authProviderContext{
			isOkteto: b.okCtx.IsOktetoCluster(),
			context:  b.okCtx.GetCurrentName(),
			token:    b.okCtx.GetCurrentToken(),
			cert:     b.okCtx.GetCurrentCertStr(),
		}

		ap := newDockerAndOktetoAuthProvider(b.okCtx.GetRegistryURL(), b.okCtx.GetCurrentUser(), b.okCtx.GetCurrentToken(), apCtx, os.Stderr)
		attachable = append(attachable, ap)
	} else {
		dockerCfg := dockerConfig.LoadDefaultConfigFile(os.Stderr)
		attachable = append(attachable, authprovider.NewDockerAuthProvider(dockerCfg, map[string]*authprovider.AuthTLSConfig{}))
	}

	for _, sess := range buildOptions.SshSessions {
		oktetoLog.Debugf("mounting ssh agent to build from %s with key %s", sess.Target, sess.Id)
		ssh, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{
			ID:    sess.Id,
			Paths: []string{sess.Target},
		}})

		if err != nil {
			return nil, fmt.Errorf("Failed to mount ssh agent for %s: %w", sess.Id, err)
		}
		attachable = append(attachable, ssh)
	}

	if len(buildOptions.Secrets) > 0 {
		secretProvider, err := buildctl.ParseSecret(buildOptions.Secrets)
		if err != nil {
			return nil, err
		}
		attachable = append(attachable, secretProvider)
	}
	opt := &client.SolveOpt{
		LocalDirs:     localDirs,
		Frontend:      string(frontend.Frontend),
		FrontendAttrs: frontendAttrs,
		Session:       attachable,
		CacheImports:  []client.CacheOptionsEntry{},
		CacheExports:  []client.CacheOptionsEntry{},
	}

	if buildOptions.Tag != "" {
		opt.Exports = []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"name": buildOptions.Tag,
					"push": "true",
				},
			},
		}
	}

	if buildOptions.LocalOutputPath != "" {
		opt.Exports = append(opt.Exports, client.ExportEntry{
			Type:      "local",
			OutputDir: buildOptions.LocalOutputPath,
		})
	}

	for _, cacheFromImage := range buildOptions.CacheFrom {
		opt.CacheImports = append(
			opt.CacheImports,
			client.CacheOptionsEntry{
				Type:  "registry",
				Attrs: map[string]string{"ref": cacheFromImage},
			},
		)
	}

	for _, exportCacheTo := range buildOptions.ExportCache {
		exportType := "inline"
		if exportCacheTo != buildOptions.Tag {
			exportType = "registry"
		}
		opt.CacheExports = append(
			opt.CacheExports,
			client.CacheOptionsEntry{
				Type: exportType,
				Attrs: map[string]string{
					"ref":  exportCacheTo,
					"mode": "max",
				},
			},
		)
	}
	// TODO(#3548): remove when we upgrade buildkit to 0.11
	if len(opt.CacheExports) > 1 {
		opt.CacheExports = opt.CacheExports[:1]
	}

	return opt, nil
}

// validate validates the build options
func (b *SolveOptBuilder) validateTags(imageTag string) error {
	if imageTag == "" {
		return nil
	}
	if strings.HasPrefix(imageTag, b.okCtx.GetCurrentBuilder()) && strings.Count(imageTag, "/") == 2 {
		return nil
	}

	numberOfSlashToBeCorrect := 2
	tags := strings.Split(imageTag, ",")
	for _, tag := range tags {
		if b.reg.IsOktetoRegistry(tag) {
			prefix := constants.DevRegistry
			if b.reg.IsGlobalRegistry(tag) {
				tag = b.imageCtrl.ExpandOktetoGlobalRegistry(tag)
				prefix = constants.GlobalRegistry
			} else {
				tag = b.imageCtrl.ExpandOktetoDevRegistry(tag)
			}
			if strings.Count(tag, "/") != numberOfSlashToBeCorrect {
				return oktetoErrors.UserError{
					E:    fmt.Errorf("'%s' isn't a valid image tag", tag),
					Hint: fmt.Sprintf("The Okteto Registry syntax is: '%s/image_name'", prefix),
				}
			}
		}
	}
	return nil
}

// extendRegistries extends the registries
func (b *SolveOptBuilder) extendRegistries(image string) string {
	if b.okCtx.IsOktetoCluster() {
		image = b.imageCtrl.ExpandOktetoDevRegistry(image)
		image = b.imageCtrl.ExpandOktetoGlobalRegistry(image)
	}
	return image
}

// getSecretTempFolder returns the secret temp folder
func getSecretTempFolder(fs afero.Fs) (string, error) {
	secretTempFolder := filepath.Join(config.GetOktetoHome(), ".secret")

	if err := fs.MkdirAll(secretTempFolder, PermissionsOwnerOnly); err != nil {
		return "", fmt.Errorf("failed to create %s: %s", secretTempFolder, err)
	}

	return secretTempFolder, nil
}

// replaceSecretsSourceEnvWithTempFile reads the content of the src of a secret and replaces the envs to mount into dockerfile
func (b *SolveOptBuilder) replaceSecretsSourceEnvWithTempFile(buildOptions *types.BuildOptions) error {
	// for each secret at buildOptions extract the src
	// read the content of the file
	// create a new file under secretTempFolder with the expanded content
	// replace the src of the secret with the tempSrc
	for indx, s := range buildOptions.Secrets {
		csvReader := csv.NewReader(strings.NewReader(s))
		fields, err := csvReader.Read()
		if err != nil {
			return fmt.Errorf("error reading the csv secret, %w", err)
		}

		newFields := make([]string, len(fields))
		for indx, field := range fields {
			key, value, found := strings.Cut(field, "=")
			if !found {
				return fmt.Errorf("secret format error")
			}

			if key == "src" || key == "source" {
				tempFileName, err := b.createTempFileWithExpandedEnvsAtSource(value)
				if err != nil {
					return fmt.Errorf("error creating the temp file with expanded values: %w", err)
				}
				value = tempFileName
			}
			newFields[indx] = fmt.Sprintf("%s=%s", key, value)
		}
		buildOptions.Secrets[indx] = strings.Join(newFields, ",")
	}
	return nil
}

// createTempFileWithExpandedEnvsAtSource creates a temp file with the expanded values of envs in local secrets
func (b *SolveOptBuilder) createTempFileWithExpandedEnvsAtSource(sourceFile string) (string, error) {
	srcFile, err := b.fs.Open(sourceFile)
	if err != nil {
		return "", err
	}

	// create temp file
	tmpfile, err := afero.TempFile(b.fs, b.secretTempFolder, "secret-")
	if err != nil {
		return "", err
	}

	writer := bufio.NewWriter(tmpfile)

	sc := bufio.NewScanner(srcFile)
	for sc.Scan() {
		// expand content
		srcContent, err := env.ExpandEnv(sc.Text())
		if err != nil {
			return "", err
		}

		// save expanded to temp file
		if _, err = writer.Write([]byte(fmt.Sprintf("%s\n", srcContent))); err != nil {
			return "", fmt.Errorf("unable to write to temp file: %w", err)
		}
		writer.Flush()
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}
	if err := srcFile.Close(); err != nil {
		return "", err
	}
	return tmpfile.Name(), sc.Err()
}

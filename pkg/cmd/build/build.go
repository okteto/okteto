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

package build

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/build/buildkit"
	"github.com/okteto/okteto/pkg/build/buildkit/connector"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	warningDockerfilePath   string = "Build '%s': Dockerfile '%s' is not in a relative path to context '%s'"
	doubleDockerfileWarning string = "Build '%s': Two Dockerfiles discovered in both the root and context path, defaulting to '%s/%s'"

	OktetoBuildQueueEnabledEnvVar string = "OKTETO_BUILD_QUEUE_ENABLED"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	GetBuilder() string
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
}

// BuildkitConnector is the interface for the buildkit connector
type BuildkitConnector interface {
	// Start establishes the connection to the buildkit server
	Start(ctx context.Context) error
	// WaitUntilIsReady waits for the buildkit server to be ready
	WaitUntilIsReady(ctx context.Context) error
	// Stop closes the connection to the buildkit server
	Stop()
	// GetBuildkitClient returns the buildkit client
	GetBuildkitClient(ctx context.Context) (*client.Client, error)
}

// OktetoBuilder runs the build of an image
type OktetoBuilder struct {
	OktetoContext OktetoContextInterface
	Fs            afero.Fs
	metadata      *buildkit.BuildMetadata
	logger        *io.Controller
	connector     BuildkitConnector
}

func (ob *OktetoBuilder) GetMetadata() *buildkit.BuildMetadata {
	return ob.metadata
}

// GetConnector returns the buildkit connector used by this builder.
// This allows the connector to be reused across multiple build operations.
func (ob *OktetoBuilder) GetConnector() BuildkitConnector {
	return ob.connector
}

// OktetoRegistryInterface checks if an image is at the registry
type OktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
}

// GetBuildkitConnector creates and returns a buildkit connector based on the OKTETO_BUILD_QUEUE_ENABLED environment variable.
// If the queue is enabled, it tries to create a port forwarder. If that fails, it falls back to a direct connector.
func GetBuildkitConnector(okCtx OktetoContextInterface, logger *io.Controller) BuildkitConnector {
	var buildkitConnector BuildkitConnector
	var err error
	if env.LoadBooleanOrDefault(OktetoBuildQueueEnabledEnvVar, false) {
		buildkitConnector, err = connector.NewPortForwarder(context.Background(), okCtx, logger)
		if err != nil {
			logger.Infof("could not create buildkit connector for port forwarding: %s", err)
			logger.Infof("falling back to ingress connector")
			logger.Out().Warning("Could not create buildkit connector for port forwarding, falling back to ingress connector")
			buildkitConnector = connector.NewIngressConnector(okCtx, logger)
		}
	} else {
		buildkitConnector = connector.NewIngressConnector(okCtx, logger)
	}
	return buildkitConnector
}

// NewOktetoBuilder creates a new instance of OktetoBuilder.
// It takes an OktetoContextInterface, afero.Fs, logger and a buildkit connector as parameters and returns a pointer to OktetoBuilder.
func NewOktetoBuilder(okCtx OktetoContextInterface, fs afero.Fs, logger *io.Controller, conn BuildkitConnector) *OktetoBuilder {
	return &OktetoBuilder{
		OktetoContext: okCtx, Fs: fs,
		metadata:  &buildkit.BuildMetadata{},
		logger:    logger,
		connector: conn,
	}
}

func (ob *OktetoBuilder) GetBuilder() string {
	return ob.OktetoContext.GetCurrentBuilder()
}

// Run runs the build sequence
func (ob *OktetoBuilder) Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error {
	isRemoteExecution := buildOptions.OutputMode == DeployOutputModeOnBuild || buildOptions.OutputMode == DestroyOutputModeOnBuild || buildOptions.OutputMode == TestOutputModeOnBuild
	buildOptions.OutputMode = setOutputMode(buildOptions.OutputMode)

	if !isRemoteExecution {
		builder := ob.GetBuilder()
		buildMsg := fmt.Sprintf("Building '%s'", buildOptions.File)
		ioCtrl.Out().Infof("%s in %s...", buildMsg, builder)
	}

	return ob.buildWithOkteto(ctx, buildOptions, ioCtrl, SolveBuild)
}

func setOutputMode(outputMode string) string {
	if outputMode != "" {
		return outputMode
	}
	switch os.Getenv(model.BuildkitProgressEnvVar) {
	case oktetoLog.PlainFormat:
		return oktetoLog.PlainFormat
	case oktetoLog.JSONFormat:
		return oktetoLog.PlainFormat
	default:
		return oktetoLog.TTYFormat
	}

}

func GetRegistryConfigFromOktetoConfig(okCtx OktetoContextInterface) *okteto.ConfigStateless {

	return &okteto.ConfigStateless{
		Cert:                        okCtx.GetCurrentCertStr(),
		IsOkteto:                    okCtx.IsOktetoCluster(),
		ContextName:                 okCtx.GetCurrentName(),
		Namespace:                   okCtx.GetNamespace(),
		RegistryUrl:                 okCtx.GetRegistryURL(),
		UserId:                      okCtx.GetCurrentUser(),
		Token:                       okCtx.GetCurrentToken(),
		GlobalNamespace:             okCtx.GetGlobalNamespace(),
		InsecureSkipTLSVerifyPolicy: okCtx.IsInsecure(),
	}
}

func (ob *OktetoBuilder) buildWithOkteto(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller, run buildkit.SolveBuildFn) error {
	oktetoLog.Infof("building your image on %s", ob.OktetoContext.GetCurrentBuilder())

	repoURL := ""
	if buildOptions.Manifest != nil && buildOptions.Manifest.ManifestPath != "" {
		repo := repository.NewRepository(buildOptions.Manifest.ManifestPath)
		repoURL = repo.GetAnonymizedRepo()
	}
	if buildOptions.Manifest != nil && repoURL == "" {
		repoURL = buildOptions.Manifest.ManifestPath
	}

	var err error
	if buildOptions.File != "" {
		buildOptions.File, err = GetDockerfile(buildOptions.File, ob.OktetoContext, repoURL, buildOptions.File, buildOptions.Target)
		if err != nil {
			return err
		}
		defer os.Remove(buildOptions.File)
	}

	// create a temp folder - this will be remove once the build has finished
	secretTempFolder, err := createSecretTempFolder()
	if err != nil {
		return err
	}
	defer os.RemoveAll(secretTempFolder)

	reg := registry.NewOktetoRegistry(GetRegistryConfigFromOktetoConfig(ob.OktetoContext))

	if err := ob.connector.WaitUntilIsReady(ctx); err != nil {
		return err
	}

	optBuilder, err := buildkit.NewSolveOptBuilder(ob.connector, reg, ob.OktetoContext, ob.Fs, ioCtrl)
	if err != nil {
		return err
	}

	opt, err := optBuilder.Build(ctx, buildOptions)
	if err != nil {
		return fmt.Errorf("failed to create build solver: %w", err)
	}

	buildSolver := buildkit.NewBuildkitRunner(ob.connector, reg, run, ioCtrl)

	if err := buildSolver.Run(ctx, opt, buildOptions.OutputMode); err != nil {
		return err
	}

	return nil
}

func validateImages(okctx OktetoContextInterface, imageTags string) error {
	reg := registry.NewOktetoRegistry(GetRegistryConfigFromOktetoConfig(okctx))

	if strings.HasPrefix(imageTags, okctx.GetRegistryURL()) && strings.Count(imageTags, "/") == 2 {
		return nil
	}
	numberOfSlashToBeCorrect := 2
	tags := strings.Split(imageTags, ",")
	imgCtrl := registry.NewImageCtrl(okctx)
	for _, tag := range tags {
		if reg.IsOktetoRegistry(tag) {
			prefix := constants.DevRegistry
			if reg.IsGlobalRegistry(tag) {
				tag = imgCtrl.ExpandOktetoGlobalRegistry(tag)
				prefix = constants.GlobalRegistry
			} else {
				tag = imgCtrl.ExpandOktetoDevRegistry(tag)
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

type regInterface interface {
	IsOktetoRegistry(image string) bool
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
}

// OptsFromBuildInfo returns the parsed options for the build from the manifest
func OptsFromBuildInfo(manifest *model.Manifest, svcName string, b *build.Info, o *types.BuildOptions, reg regInterface, okCtx OktetoContextInterface) *types.BuildOptions {
	if o == nil {
		o = &types.BuildOptions{}
	}
	if o.Target != "" {
		b.Target = o.Target
	}
	if len(o.CacheFrom) != 0 {
		b.CacheFrom = o.CacheFrom
	}
	if o.Tag != "" {
		b.Image = o.Tag
	}

	name := ""
	if manifest != nil && manifest.Name != "" {
		name = manifest.Name
	}
	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(name)
	if okCtx.IsOktetoCluster() && b.Image == "" {
		b.Image = fmt.Sprintf("%s/%s-%s:%s", constants.DevRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag)
	}

	file := b.Dockerfile
	if b.Context != "" && b.Dockerfile != "" {
		file = extractFromContextAndDockerfile(b.Context, b.Dockerfile, svcName, os.Getwd)
	}

	args := []build.Arg{}
	optionsBuildArgs := map[string]string{}
	minArgFormatParts := 1
	maxArgFormatParts := 2
	for _, arg := range o.BuildArgs {

		splittedArg := strings.SplitN(arg, "=", maxArgFormatParts)
		if len(splittedArg) == minArgFormatParts {
			optionsBuildArgs[splittedArg[0]] = ""
			args = append(args, build.Arg{
				Name: splittedArg[0], Value: "",
			})
		} else if len(splittedArg) == maxArgFormatParts {
			optionsBuildArgs[splittedArg[0]] = splittedArg[1]
			args = append(args, build.Arg{
				Name: splittedArg[0], Value: splittedArg[1],
			})
		} else {
			oktetoLog.Infof("invalid build-arg '%s'", arg)
		}
	}

	for _, e := range b.Args {
		if _, exists := optionsBuildArgs[e.Name]; exists {
			continue
		}
		args = append(args, e)
	}

	if reg.IsOktetoRegistry(b.Image) {
		defaultBuildArgs := map[string]string{
			model.OktetoContextEnvVar:   okCtx.GetCurrentName(),
			model.OktetoNamespaceEnvVar: okCtx.GetNamespace(),
		}

		for _, e := range b.Args {
			if _, exists := defaultBuildArgs[e.Name]; !exists {
				continue
			}
			// we don't want to replace build arguments that were already set by the user
			delete(defaultBuildArgs, e.Name)
		}

		for key, val := range defaultBuildArgs {
			if val == "" {
				continue
			}

			args = append(args, build.Arg{
				Name: key, Value: val,
			})
		}
	}

	opts := &types.BuildOptions{
		Manifest:    manifest,
		CacheFrom:   b.CacheFrom,
		Target:      b.Target,
		Path:        b.Context,
		Tag:         b.Image,
		File:        file,
		BuildArgs:   build.SerializeArgs(args),
		NoCache:     o.NoCache,
		ExportCache: b.ExportCache,
		Platform:    o.Platform,
	}

	// if secrets are present at the cmd flag, copy them to opts.Secrets
	if o.Secrets != nil {
		opts.Secrets = o.Secrets
	}
	// add to the build the secrets from the manifest build
	for id, src := range b.Secrets {
		opts.Secrets = append(opts.Secrets, fmt.Sprintf("id=%s,src=%s", id, src))
	}

	outputMode := oktetoLog.GetOutputFormat()
	if o != nil && o.OutputMode != "" {
		outputMode = o.OutputMode
	}
	opts.OutputMode = setOutputMode(outputMode)

	return opts
}

// OptsFromBuildInfoForRemoteDeploy returns the options for the remote deploy
func OptsFromBuildInfoForRemoteDeploy(b *build.Info, o *types.BuildOptions) *types.BuildOptions {
	opts := &types.BuildOptions{
		Path:       b.Context,
		OutputMode: o.OutputMode,
		File:       b.Dockerfile,
		Platform:   o.Platform,
	}
	return opts
}

func extractFromContextAndDockerfile(context, dockerfile, svcName string, getWd func() (string, error)) string {
	if filepath.IsAbs(dockerfile) {
		return dockerfile
	}

	fs := afero.NewOsFs()

	joinPath := filepath.Join(context, dockerfile)
	if !filesystem.FileExistsAndNotDir(joinPath, fs) {
		oktetoLog.Warning(fmt.Sprintf(warningDockerfilePath, svcName, dockerfile, context))
		return dockerfile
	}

	wd, err := getWd()
	if err != nil {
		return joinPath
	}

	if !filepath.IsAbs(joinPath) {
		joinPath = filepath.Join(wd, joinPath)
	}

	if joinPath != filepath.Join(wd, filepath.Clean(dockerfile)) && filesystem.FileExistsAndNotDir(dockerfile, fs) {
		oktetoLog.Warning(fmt.Sprintf(doubleDockerfileWarning, svcName, context, dockerfile))
	}

	return joinPath
}

func createSecretTempFolder() (string, error) {
	secretTempFolder := filepath.Join(config.GetOktetoHome(), ".secret")
	if err := os.MkdirAll(secretTempFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create %s: %s", secretTempFolder, err)
	}

	return secretTempFolder, nil
}

// replaceSecretsSourceEnvWithTempFile reads the content of the src of a secret and replaces the envs to mount into dockerfile
func replaceSecretsSourceEnvWithTempFile(fs afero.Fs, secretTempFolder string, buildOptions *types.BuildOptions) error {
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
				tempFileName, err := createTempFileWithExpandedEnvsAtSource(fs, value, secretTempFolder)
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
func createTempFileWithExpandedEnvsAtSource(fs afero.Fs, sourceFile, tempFolder string) (string, error) {
	srcFile, err := fs.Open(sourceFile)
	if err != nil {
		return "", err
	}

	// create temp file
	tmpfile, err := afero.TempFile(fs, tempFolder, "secret-")
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

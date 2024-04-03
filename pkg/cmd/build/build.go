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

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/okteto/okteto/pkg/build"
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
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	warningDockerfilePath   string = "Build '%s': Dockerfile '%s' is not in a relative path to context '%s'"
	doubleDockerfileWarning string = "Build '%s': Two Dockerfiles discovered in both the root and context path, defaulting to '%s/%s'"
)

var (
	errDockerDaemonConnection = oktetoErrors.UserError{
		E:    fmt.Errorf("cannot connect to Docker Daemon"),
		Hint: "Please start the Docker Daemon or configure a builder endpoint with 'okteto context --builder BUILDKIT_URL",
	}
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	GetBuilder() string
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
}

// OktetoBuilder runs the build of an image
type OktetoBuilder struct {
	OktetoContext OktetoContextInterface
	Fs            afero.Fs
}

// OktetoRegistryInterface checks if an image is at the registry
type OktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
}

// NewOktetoBuilder creates a new instance of OktetoBuilder.
// It takes an OktetoContextInterface and afero.Fs as parameters and returns a pointer to OktetoBuilder.
func NewOktetoBuilder(context OktetoContextInterface, fs afero.Fs) *OktetoBuilder {
	return &OktetoBuilder{
		OktetoContext: context,
		Fs:            fs,
	}
}

func (ob *OktetoBuilder) GetBuilder() string {
	return ob.OktetoContext.GetCurrentBuilder()
}

// Run runs the build sequence
func (ob *OktetoBuilder) Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error {
	isDeployOrDestroy := buildOptions.OutputMode == DeployOutputModeOnBuild || buildOptions.OutputMode == DestroyOutputModeOnBuild
	buildOptions.OutputMode = setOutputMode(buildOptions.OutputMode)
	depotToken := os.Getenv(DepotTokenEnvVar)
	depotProject := os.Getenv(DepotProjectEnvVar)

	if !isDeployOrDestroy {
		builder := ob.GetBuilder()
		buildMsg := fmt.Sprintf("Building '%s'", buildOptions.File)
		depotEnabled := IsDepotEnabled()
		if depotEnabled {
			ioCtrl.Out().Infof("%s on depot's machine...", buildMsg)
		} else if builder == "" {
			ioCtrl.Out().Infof("%s using your local docker daemon", buildMsg)
		} else {
			ioCtrl.Out().Infof("%s in %s...", buildMsg, builder)
		}
	}

	switch {
	// When depot is available we only go to depot if it's not a deploy or a destroy.
	// On depot the workload id is not working correctly and the users would not be able to
	// use the internal cluster ip as if they were running their scripts on the k8s cluster
	case IsDepotEnabled() && !isDeployOrDestroy:
		depotManager := newDepotBuilder(depotProject, depotToken, ob.OktetoContext, ioCtrl)
		return depotManager.Run(ctx, buildOptions, runAndHandleBuild)
	case ob.OktetoContext.GetCurrentBuilder() == "":
		return ob.buildWithDocker(ctx, buildOptions)
	default:
		return ob.buildWithOkteto(ctx, buildOptions, ioCtrl, runAndHandleBuild)
	}
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
		IsOkteto:                    okCtx.IsOkteto(),
		ContextName:                 okCtx.GetCurrentName(),
		Namespace:                   okCtx.GetCurrentNamespace(),
		RegistryUrl:                 okCtx.GetCurrentRegister(),
		UserId:                      okCtx.GetCurrentUser(),
		Token:                       okCtx.GetCurrentToken(),
		GlobalNamespace:             okCtx.GetGlobalNamespace(),
		InsecureSkipTLSVerifyPolicy: okCtx.IsInsecure(),
	}
}

func (ob *OktetoBuilder) buildWithOkteto(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller, run runAndHandleBuildFn) error {
	oktetoLog.Infof("building your image on %s", ob.OktetoContext.GetCurrentBuilder())

	var err error
	if buildOptions.File != "" {
		buildOptions.File, err = GetDockerfile(buildOptions.File, ob.OktetoContext)
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

	opt, err := getSolveOpt(buildOptions, ob.OktetoContext, secretTempFolder, ob.Fs)
	if err != nil {
		return errors.Wrap(err, "failed to create build solver")
	}

	buildkitClient, err := getBuildkitClient(ctx, ob.OktetoContext)
	if err != nil {
		return err
	}

	return run(ctx, buildkitClient, opt, buildOptions, ob.OktetoContext, ioCtrl)
}

// https://github.com/docker/cli/blob/56e5910181d8ac038a634a203a4f3550bb64991f/cli/command/image/build.go#L209
func (ob *OktetoBuilder) buildWithDocker(ctx context.Context, buildOptions *types.BuildOptions) error {

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	if versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.39") {
		err = buildWithDockerDaemonBuildkit(ctx, buildOptions, cli)
		if err != nil {
			return translateDockerErr(err)
		}
	} else {
		err = buildWithDockerDaemon(ctx, buildOptions, cli)
		if err != nil {
			return translateDockerErr(err)
		}
	}
	if buildOptions.Tag != "" {
		return pushImage(ctx, buildOptions.Tag, cli)
	}
	return nil
}

func validateImage(okctx OktetoContextInterface, imageTag string) error {
	reg := registry.NewOktetoRegistry(GetRegistryConfigFromOktetoConfig(okctx))
	if strings.HasPrefix(imageTag, okctx.GetCurrentRegister()) && strings.Count(imageTag, "/") == 2 {
		return nil
	}
	if (reg.IsOktetoRegistry(imageTag)) && strings.Count(imageTag, "/") != 1 {
		prefix := constants.DevRegistry
		if reg.IsGlobalRegistry(imageTag) {
			prefix = constants.GlobalRegistry
		}
		return oktetoErrors.UserError{
			E:    fmt.Errorf("'%s' isn't a valid image tag", imageTag),
			Hint: fmt.Sprintf("The Okteto Registry syntax is: '%s/image_name'", prefix),
		}
	}
	return nil
}

func translateDockerErr(err error) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), "failed to dial gRPC: cannot connect to the Docker daemon") {
		return errDockerDaemonConnection
	}
	return err
}

type regInterface interface {
	IsOktetoRegistry(image string) bool
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
}

// OptsFromBuildInfo returns the parsed options for the build from the manifest
func OptsFromBuildInfo(manifestName, svcName string, b *build.Info, o *types.BuildOptions, reg regInterface, okCtx OktetoContextInterface) *types.BuildOptions {
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

	// manifestName can be not sanitized when option name is used at deploy
	sanitizedName := format.ResourceK8sMetaString(manifestName)
	if okCtx.IsOkteto() && b.Image == "" {
		// if flag --global, point to global registry
		targetRegistry := constants.DevRegistry
		if o != nil && o.BuildToGlobal {
			targetRegistry = constants.GlobalRegistry
		}
		b.Image = fmt.Sprintf("%s/%s-%s:%s", targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag)
	}

	file := b.Dockerfile
	if b.Context != "" && b.Dockerfile != "" {
		file = extractFromContextAndDockerfile(b.Context, b.Dockerfile, svcName)
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
			model.OktetoNamespaceEnvVar: okCtx.GetCurrentNamespace(),
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

func extractFromContextAndDockerfile(context, dockerfile, svcName string) string {
	if filepath.IsAbs(dockerfile) {
		return dockerfile
	}

	fs := afero.NewOsFs()

	joinPath := filepath.Join(context, dockerfile)
	if !filesystem.FileExistsAndNotDir(joinPath, fs) {
		oktetoLog.Warning(fmt.Sprintf(warningDockerfilePath, svcName, dockerfile, context))
		return dockerfile
	}

	if joinPath != filepath.Clean(dockerfile) && filesystem.FileExistsAndNotDir(dockerfile, fs) {
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

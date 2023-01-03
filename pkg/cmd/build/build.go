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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
)

const (
	warningDockerfilePath   string = "Build '%s': Dockerfile '%s' is not in a relative path to context '%s'"
	doubleDockerfileWarning string = "Build '%s': Two Dockerfiles discovered in both the root and context path, defaulting to '%s/%s'"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions) error
}

// OktetoBuilder runs the build of an image
type OktetoBuilder struct{}

// OktetoRegistryInterface checks if an image is at the registry
type OktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
}

// Run runs the build sequence
func (*OktetoBuilder) Run(ctx context.Context, buildOptions *types.BuildOptions) error {
	buildOptions.OutputMode = setOutputMode(buildOptions.OutputMode)
	if okteto.Context().Builder == "" {
		if err := buildWithDocker(ctx, buildOptions); err != nil {
			return err
		}
	} else {
		if err := buildWithOkteto(ctx, buildOptions); err != nil {
			return err
		}
	}
	return nil
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

func buildWithOkteto(ctx context.Context, buildOptions *types.BuildOptions) error {
	oktetoLog.Infof("building your image on %s", okteto.Context().Builder)
	buildkitClient, err := getBuildkitClient(ctx)
	if err != nil {
		return err
	}

	if buildOptions.File != "" {
		buildOptions.File, err = registry.GetDockerfile(buildOptions.File)
		if err != nil {
			return err
		}
		defer os.Remove(buildOptions.File)
	}

	if buildOptions.Tag != "" {
		err = validateImage(buildOptions.Tag)
		if err != nil {
			return err
		}
	}

	if okteto.IsOkteto() {
		buildOptions.Tag = registry.ExpandOktetoDevRegistry(buildOptions.Tag)
		buildOptions.Tag = registry.ExpandOktetoGlobalRegistry(buildOptions.Tag)
		for i := range buildOptions.CacheFrom {
			buildOptions.CacheFrom[i] = registry.ExpandOktetoDevRegistry(buildOptions.CacheFrom[i])
			buildOptions.CacheFrom[i] = registry.ExpandOktetoGlobalRegistry(buildOptions.CacheFrom[i])
		}
		if buildOptions.ExportCache != "" {
			buildOptions.ExportCache = registry.ExpandOktetoDevRegistry(buildOptions.ExportCache)
			buildOptions.ExportCache = registry.ExpandOktetoGlobalRegistry(buildOptions.ExportCache)
		}
	}

	// create a temp folder - this will be remove once the build has finished
	secretTempFolder := filepath.Join(config.GetOktetoHome(), ".secret")
	if err := os.MkdirAll(secretTempFolder, 0700); err != nil {
		return fmt.Errorf("failed to create %s: %s", secretTempFolder, err)
	}
	defer os.RemoveAll(secretTempFolder)

	// inject secrets to buildkit from temp folder
	if err := parseTempSecrets(secretTempFolder, buildOptions); err != nil {
		return err
	}

	opt, err := getSolveOpt(buildOptions)
	if err != nil {
		return errors.Wrap(err, "failed to create build solver")
	}

	err = solveBuild(ctx, buildkitClient, opt, buildOptions.OutputMode)
	if err != nil {
		oktetoLog.Infof("Failed to build image: %s", err.Error())
	}
	if registry.IsTransientError(err) {
		oktetoLog.Yellow(`Failed to push '%s' to the registry:
  %s,
  Retrying ...`, buildOptions.Tag, err.Error())
		success := true
		err := solveBuild(ctx, buildkitClient, opt, buildOptions.OutputMode)
		if err != nil {
			success = false
			oktetoLog.Infof("Failed to build image: %s", err.Error())
		}
		err = registry.GetErrorMessage(err, buildOptions.Tag)
		analytics.TrackBuildTransientError(okteto.Context().Builder, success)
		return err
	}

	if err == nil && buildOptions.Tag != "" {
		if _, err := registry.NewOktetoRegistry().GetImageTagWithDigest(buildOptions.Tag); err != nil {
			oktetoLog.Yellow(`Failed to push '%s' metadata to the registry:
	  %s,
	  Retrying ...`, buildOptions.Tag, err.Error())
			success := true
			err := solveBuild(ctx, buildkitClient, opt, buildOptions.OutputMode)
			if err != nil {
				success = false
				oktetoLog.Infof("Failed to build image: %s", err.Error())
			}
			err = registry.GetErrorMessage(err, buildOptions.Tag)
			analytics.TrackBuildPullError(okteto.Context().Builder, success)
			return err
		}
	}

	err = registry.GetErrorMessage(err, buildOptions.Tag)
	return err
}

// https://github.com/docker/cli/blob/56e5910181d8ac038a634a203a4f3550bb64991f/cli/command/image/build.go#L209
func buildWithDocker(ctx context.Context, buildOptions *types.BuildOptions) error {

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

func validateImage(imageTag string) error {
	if strings.HasPrefix(imageTag, okteto.Context().Registry) && strings.Count(imageTag, "/") == 2 {
		return nil
	}
	if (registry.IsOktetoRegistry(imageTag)) && strings.Count(imageTag, "/") != 1 {
		prefix := okteto.DevRegistry
		if registry.IsGlobalRegistry(imageTag) {
			prefix = okteto.GlobalRegistry
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
		return oktetoErrors.UserError{
			E:    fmt.Errorf("cannot connect to Docker Daemon"),
			Hint: "Please start the Docker Daemon or configure a builder endpoint with 'okteto context --builder BUILDKIT_URL",
		}
	}
	return err
}

// OptsFromBuildInfo returns the parsed options for the build from the manifest
func OptsFromBuildInfo(manifestName, svcName string, b *model.BuildInfo, o *types.BuildOptions) *types.BuildOptions {
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
	if okteto.Context().IsOkteto && b.Image == "" {
		// if flag --global, point to global registry
		targetRegistry := okteto.DevRegistry
		if o != nil && o.BuildToGlobal {
			targetRegistry = okteto.GlobalRegistry
		}
		b.Image = fmt.Sprintf("%s/%s-%s:%s", targetRegistry, sanitizedName, svcName, model.OktetoDefaultImageTag)
		if len(b.VolumesToInclude) > 0 {
			b.Image = fmt.Sprintf("%s/%s-%s:%s", targetRegistry, sanitizedName, svcName, model.OktetoImageTagWithVolumes)
		}
	}

	file := b.Dockerfile
	if b.Context != "" && b.Dockerfile != "" {
		file = extractFromContextAndDockerfile(b.Context, b.Dockerfile, svcName)
	}

	if registry.IsOktetoRegistry(b.Image) {
		defaultBuildArgs := map[string]string{
			model.OktetoContextEnvVar:   okteto.Context().Name,
			model.OktetoNamespaceEnvVar: okteto.Context().Namespace,
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

			b.Args = append(b.Args, model.BuildArg{
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
		BuildArgs:   model.SerializeBuildArgs(b.Args),
		NoCache:     o.NoCache,
		ExportCache: b.ExportCache,
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

func extractFromContextAndDockerfile(context, dockerfile, svcName string) string {
	if filepath.IsAbs(dockerfile) {
		return dockerfile
	}

	joinPath := filepath.Join(context, dockerfile)
	if !filesystem.FileExistsAndNotDir(joinPath) {
		oktetoLog.Warning(fmt.Sprintf(warningDockerfilePath, svcName, dockerfile, context))
		return dockerfile
	}

	if joinPath != filepath.Clean(dockerfile) && filesystem.FileExistsAndNotDir(dockerfile) {
		oktetoLog.Warning(fmt.Sprintf(doubleDockerfileWarning, svcName, context, dockerfile))
	}

	return joinPath
}

// GetVolumesToInclude checks if the path exists, if it doesn't it skip it
func GetVolumesToInclude(volumesToInclude []model.StackVolume) []model.StackVolume {
	result := []model.StackVolume{}
	for _, p := range volumesToInclude {
		if _, err := os.Stat(p.LocalPath); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// parseTempSecrets reads the content of the src of a secret and replaces the envs to mount into dockerfile
func parseTempSecrets(secretTempFolder string, buildOptions *types.BuildOptions) error {
	// for each secret at buildOptions extract the src
	// read the content of the file
	// create a new file under tempFolder with the expanded content
	// replace the src of the secret with the tempSrc
	for indx, s := range buildOptions.Secrets {
		splitSecret := strings.SplitN(s, "src=", 2)
		if len(splitSecret) <= 0 {
			return fmt.Errorf("unable to get secret src")
		}
		srcFileName := strings.TrimSpace(splitSecret[1])

		// read source file
		srcFile, err := os.Open(srcFileName)
		if err != nil {
			return err
		}

		// create temp file
		tmpfile, err := os.CreateTemp(secretTempFolder, "secret-")
		if err != nil {
			return err
		}
		writer := bufio.NewWriter(tmpfile)

		sc := bufio.NewScanner(srcFile)
		for sc.Scan() {
			// expand content
			srcContent, err := model.ExpandEnv(sc.Text(), true)
			if err != nil {
				return err
			}

			// save expanded to temp file
			_, _ = writer.Write([]byte(fmt.Sprintf("%s\n", srcContent)))
			writer.Flush()
		}
		if err := tmpfile.Close(); err != nil {
			return err
		}
		if err := srcFile.Close(); err != nil {
			return err
		}
		if sc.Err() != nil {
			return sc.Err()
		}

		// replace src
		buildOptions.Secrets[indx] = fmt.Sprintf("%ssrc=%s", splitSecret[0], tmpfile.Name())
	}

	return nil
}

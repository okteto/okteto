// Copyright 2022 The Okteto Authors
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
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/pkg/errors"
)

//BuildOptions define the options available for build
type BuildOptions struct {
	BuildArgs     []string
	CacheFrom     []string
	File          string
	NoCache       bool
	OutputMode    string
	Path          string
	Secrets       []string
	Tag           string
	Target        string
	Namespace     string
	BuildToGlobal bool
	K8sContext    string
	ExportCache   string
}

// Run runs the build sequence
func Run(ctx context.Context, buildOptions *BuildOptions) error {
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

func buildWithOkteto(ctx context.Context, buildOptions *BuildOptions) error {
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
		if _, err := registry.GetImageTagWithDigest(buildOptions.Tag); err != nil {
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
func buildWithDocker(ctx context.Context, buildOptions *BuildOptions) error {

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

func getBuildOptionsFromManifest(service string, manifestBuildInfo *model.BuildInfo) *BuildOptions {
	buildOptions := &BuildOptions{
		Target:      manifestBuildInfo.Target,
		Path:        manifestBuildInfo.Context,
		CacheFrom:   manifestBuildInfo.CacheFrom,
		ExportCache: manifestBuildInfo.ExportCache,
		Tag:         manifestBuildInfo.Image,
	}
	if len(manifestBuildInfo.Args) > 0 {
		buildOptions.BuildArgs = model.SerializeBuildArgs(manifestBuildInfo.Args)
	}

	buildOptions.File = manifestBuildInfo.Dockerfile
	if !filepath.IsAbs(manifestBuildInfo.Dockerfile) && !model.FileExistsAndNotDir(manifestBuildInfo.Dockerfile) {
		buildOptions.File = filepath.Join(manifestBuildInfo.Context, manifestBuildInfo.Dockerfile)
	}

	if okteto.Context().IsOkteto && buildOptions.Tag == "" {
		if manifestBuildInfo.Name == "" {
			manifestBuildInfo.Name = os.Getenv(model.OktetoNameEnvVar)
		}

		envGitCommit := os.Getenv(model.OktetoGitCommitEnvVar)
		isLocalEnvGitCommit := strings.HasPrefix(envGitCommit, model.OktetoGitCommitPrefix)
		isPipeline := envGitCommit != "" && !isLocalEnvGitCommit

		targetRegistry := okteto.DevRegistry
		tag := model.OktetoDefaultImageTag

		if len(manifestBuildInfo.VolumesToInclude) > 0 {
			tag = model.OktetoImageTagWithVolumes
		} else if isPipeline {
			// if build is running at a pipeline with OKTETO_GIT_COMMIT, tag is replaced by a sha of this params
			params := strings.Join(buildOptions.BuildArgs, "") + envGitCommit
			tag = fmt.Sprintf("%x", sha256.Sum256([]byte(params)))
		}
		if manifestBuildInfo.Name != "" {
			buildOptions.Tag = fmt.Sprintf("%s/%s-%s:%s", targetRegistry, manifestBuildInfo.Name, service, tag)
		} else {
			buildOptions.Tag = fmt.Sprintf("%s/%s:%s", targetRegistry, service, tag)
		}
	}

	return buildOptions
}

func overrideManifestBuildOptions(manifestBuildOptions, cmdBuildOptions *BuildOptions) *BuildOptions {
	if cmdBuildOptions == nil {
		cmdBuildOptions = &BuildOptions{}
	}
	if manifestBuildOptions == nil {
		manifestBuildOptions = &BuildOptions{}
	}

	// copy the remaining of cmdOptions into the manifestBuildOptions
	if cmdBuildOptions.K8sContext != manifestBuildOptions.K8sContext {
		manifestBuildOptions.K8sContext = cmdBuildOptions.K8sContext
	}
	if cmdBuildOptions.Namespace != manifestBuildOptions.Namespace {
		manifestBuildOptions.Namespace = cmdBuildOptions.Namespace
	}
	if len(cmdBuildOptions.Secrets) > 0 {
		manifestBuildOptions.Secrets = cmdBuildOptions.Secrets
	}
	if cmdBuildOptions.NoCache != manifestBuildOptions.NoCache {
		manifestBuildOptions.NoCache = cmdBuildOptions.NoCache
	}
	if len(cmdBuildOptions.BuildArgs) > 0 {
		manifestBuildOptions.BuildArgs = cmdBuildOptions.BuildArgs
	}
	if len(cmdBuildOptions.CacheFrom) > 0 {
		manifestBuildOptions.CacheFrom = cmdBuildOptions.CacheFrom
	}
	if cmdBuildOptions.ExportCache != manifestBuildOptions.ExportCache {
		manifestBuildOptions.ExportCache = cmdBuildOptions.ExportCache
	}

	// override output mode from cmdBuildOptions
	if cmdBuildOptions.OutputMode != "" {
		manifestBuildOptions.OutputMode = setOutputMode(cmdBuildOptions.OutputMode)
	} else if manifestBuildOptions.OutputMode == "" {
		manifestBuildOptions.OutputMode = setOutputMode(oktetoLog.GetOutputFormat())
	}

	if cmdBuildOptions.Tag != "" {
		manifestBuildOptions.Tag = cmdBuildOptions.Tag
	}

	// override Tag if global registry
	if okteto.Context().IsOkteto && cmdBuildOptions.BuildToGlobal {
		manifestBuildOptions.Tag = registry.ReplaceTargetRepository(manifestBuildOptions.Tag, okteto.GlobalRegistry, manifestBuildOptions.Namespace)
	}

	return manifestBuildOptions
}

// OptsFromManifest returns the parsed options for the build from the manifest
func OptsFromManifest(service string, manifestBuildInfo *model.BuildInfo, cmdBuildOptions *BuildOptions) *BuildOptions {
	manifestBuildOptions := getBuildOptionsFromManifest(service, manifestBuildInfo)
	return overrideManifestBuildOptions(manifestBuildOptions, cmdBuildOptions)
}

// optimizedGlobalBuild returns image with digest tag if found at the global registry
func optimizedGlobalBuild(image string) (string, error) {

	if registry.IsDevRegistry(image) || registry.IsExtendedOktetoRegistry(image) {
		image = registry.ReplaceTargetRepository(image, okteto.GlobalRegistry, okteto.DefaultGlobalNamespace)
	}
	tagWithDigest, err := registry.GetImageTagWithDigest(image)
	if err != nil {
		return "", err
	}
	return tagWithDigest, nil
}

// OptimizeBuildWithDigest returns the image with digest tag if found at global or dev okteto registry.
// If image is not present at any global or dev registry will return empty string.
// If err is different from NotFound, it will return err and empty string.
func OptimizeBuildWithDigest(service string, opts *BuildOptions) (string, error) {
	if opts.NoCache {
		oktetoLog.Debug("skipping optimization: --no-cache option active")
		return "", nil
	}
	if !registry.IsOktetoRegistry(opts.Tag) {
		oktetoLog.Debug("skipping optimization: image tag is not at okteto registry")
		return "", nil
	}

	oktetoLog.Debug("build optimization: check on global registry")

	// first check global registry
	tagWithDigest, err := optimizedGlobalBuild(opts.Tag)
	if err != nil && err != oktetoErrors.ErrNotFound {
		return "", fmt.Errorf("build optimization: check on global registry: %v", err)
	}
	if tagWithDigest != "" {
		oktetoLog.Debugf("build optimization: image found at global registry: %s", opts.Tag)
		return tagWithDigest, nil
	}

	// check dev registry if not found at global
	oktetoLog.Debug("build optimization: check on dev registry")
	tagWithDigest, err = registry.GetImageTagWithDigest(opts.Tag)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debugf("build optimization: image not found at dev registry: %s", opts.Tag)
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("build optimization: check on dev registry: %v", err)
	}
	oktetoLog.Debugf("build optimization: image found at dev registry: %s", opts.Tag)
	return tagWithDigest, nil
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

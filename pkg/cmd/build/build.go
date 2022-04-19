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
		BuildArgs:   model.SerializeBuildArgs(manifestBuildInfo.Args),
		OutputMode:  setOutputMode(oktetoLog.GetOutputFormat()),
		Tag:         manifestBuildInfo.Image,
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

		targetRegistry := okteto.DevRegistry
		tag := model.OktetoDefaultImageTag

		if len(manifestBuildInfo.VolumesToInclude) > 0 {
			tag = model.OktetoImageTagWithVolumes
		} else if envGitCommit != "" && !isLocalEnvGitCommit {
			// if build is running at a pipeline with OKTETO_GIT_COMMIT, tag is replaced by a sha of this params
			params := strings.Join(buildOptions.BuildArgs, "") + envGitCommit
			tag = fmt.Sprintf("%x", sha256.Sum256([]byte(params)))
		}
		buildOptions.Tag = fmt.Sprintf("%s/%s-%s:%s", targetRegistry, manifestBuildInfo.Name, service, tag)
	}

	return buildOptions
}

func overrideManifestBuildOptions(manifestBuildOptions, cmdBuildOptions *BuildOptions) *BuildOptions {
	if cmdBuildOptions == nil {
		cmdBuildOptions = &BuildOptions{}
	}

	// copy the remaining of cmdOptions into the manifestBuildOptions
	if cmdBuildOptions.K8sContext != "" {
		manifestBuildOptions.K8sContext = cmdBuildOptions.K8sContext
	}
	if cmdBuildOptions.Namespace != "" {
		manifestBuildOptions.Namespace = cmdBuildOptions.Namespace
	}
	if len(cmdBuildOptions.Secrets) > 0 {
		manifestBuildOptions.Secrets = cmdBuildOptions.Secrets
	}
	if manifestBuildOptions.NoCache != cmdBuildOptions.NoCache {
		manifestBuildOptions.NoCache = cmdBuildOptions.NoCache
	}

	// override output mode from cmdBuildOptions
	if cmdBuildOptions.OutputMode != "" {
		manifestBuildOptions.OutputMode = setOutputMode(cmdBuildOptions.OutputMode)
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

// optimizedGlobalBuild returns if build should be optimized because the image is already at the global registry
func (opts *BuildOptions) optimizedGlobalBuild() (string, bool, error) {
	envGitCommit := os.Getenv(model.OktetoGitCommitEnvVar)
	isLocalEnvGitCommit := strings.HasPrefix(envGitCommit, model.OktetoGitCommitPrefix)
	shouldApply := registry.IsOktetoRegistry(opts.Tag) && envGitCommit != "" && !isLocalEnvGitCommit
	if !shouldApply {
		return "", false, nil
	}
	oktetoLog.Debugf("Applying global build optimization for image %s", opts.Tag)

	globalReference := opts.Tag
	if registry.IsDevRegistry(opts.Tag) || registry.IsExtendedOktetoRegistry(opts.Tag) {
		globalReference = registry.ReplaceTargetRepository(opts.Tag, okteto.GlobalRegistry, okteto.DefaultGlobalNamespace)

	}
	tagWithDigest, err := registry.GetImageTagWithDigest(globalReference)
	if err != nil {
		return opts.Tag, false, err
	}
	oktetoLog.Debugf("skipping build: image %s is already built", globalReference)
	return tagWithDigest, true, nil
}

// checkImageAtRegistry returns if build should be optimized because the image is already at the dev registry
func (opts *BuildOptions) checkImageAtRegistry() (string, bool, error) {
	oktetoLog.Debugf("Checking registry for image %s", opts.Tag)
	tagWithDigest, err := registry.GetImageTagWithDigest(opts.Tag)
	if err != nil {
		return opts.Tag, false, err
	}
	oktetoLog.Debugf("skipping build: image %s is already built", opts.Tag)
	return tagWithDigest, true, nil
}

// SkipBuild returns if build has to be skipped and the tag with digest if found at registry
func (opts *BuildOptions) SkipBuild(service string) (string, bool, error) {
	if opts.NoCache {
		return "", false, nil
	}
	if !registry.IsOktetoRegistry(opts.Tag) {
		return "", false, nil
	}

	if tagWithDigest, ok, err := opts.optimizedGlobalBuild(); ok {
		// global optimization has been applied and use global tag for deployment
		oktetoLog.Debugf("Skipping '%s' build. Image already exists at Okteto Registry", service)
		return tagWithDigest, true, nil
	} else if err != nil && err != oktetoErrors.ErrNotFound {
		// not applicated and error while checking - return err
		return "", false, fmt.Errorf("error checking image at registry %s: %v", opts.Tag, err)
	} else if !ok {
		// check dev registry
		if tagWithDigest, ok, err := opts.checkImageAtRegistry(); ok {
			oktetoLog.Debugf("Skipping '%s' build. Image already exists at Okteto Registry", service)
			return tagWithDigest, true, nil
		} else if err != nil && err == oktetoErrors.ErrNotFound {
			// image not found - have to build
			oktetoLog.Debug("image not found, building image")
			return "", false, nil
		}
	}
	return "", false, nil
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

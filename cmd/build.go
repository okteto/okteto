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

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/spf13/cobra"
)

//Build build and optionally push a Docker image
func Build(ctx context.Context) *cobra.Command {

	options := build.BuildOptions{}
	cmd := &cobra.Command{
		Use:   "build [service...]",
		Short: "Build and push the images defined in the 'build' section of your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {

			ctxOpts := &contextCMD.ContextOptions{}

			manifest, errManifest := model.GetManifestV2(options.File)
			if errManifest != nil {
				oktetoLog.Debug("error getting manifest v2 from file %s: %v. Fallback to build v1", options.File, errManifest)
			}

			isBuildV2 := errManifest == nil &&
				manifest.IsV2 &&
				len(manifest.Build) != 0

			if isBuildV2 {
				if manifest.Context != "" {
					ctxOpts.Context = manifest.Context
					if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
						return err
					}
				}

				if options.Namespace == "" && manifest.Namespace != "" {
					ctxOpts.Namespace = manifest.Namespace
				}
			} else {
				maxV1Args := 1
				docsURL := "https://okteto.com/docs/reference/cli/#build"
				if len(args) > maxV1Args {
					return oktetoErrors.UserError{
						E:    fmt.Errorf("%q accepts at most %d arg(s), but received %d", cmd.CommandPath(), maxV1Args, len(args)),
						Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
					}
				}

				if options.Namespace != "" {
					ctxOpts.Namespace = options.Namespace
				}
			}

			if okteto.IsOkteto() && ctxOpts.Namespace != "" {
				create, err := utils.ShouldCreateNamespace(ctx, ctxOpts.Namespace)
				if err != nil {
					return err
				}
				if create {
					nsCmd, err := namespace.NewCommand()
					if err != nil {
						return err
					}
					if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: ctxOpts.Namespace}); err != nil {
						return err
					}
				}
			}

			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			if isBuildV2 {
				return buildV2(manifest, &options, args)
			}

			return buildV1(&options, args)
		},
	}

	cmd.Flags().StringVarP(&options.File, "file", "f", "", "path to the Okteto Manifest (default is 'okteto.yml')")
	cmd.Flags().StringVarP(&options.Tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&options.Target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&options.CacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringVarP(&options.ExportCache, "export-cache", "", "", "export cache image")
	cmd.Flags().StringVarP(&options.OutputMode, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output")
	cmd.Flags().StringArrayVar(&options.BuildArgs, "build-arg", nil, "set build-time variables")
	cmd.Flags().StringArrayVar(&options.Secrets, "secret", nil, "secret files exposed to the build. Format: id=mysecret,src=/local/secret")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "namespace against which the image will be consumed. Default is the one defined at okteto context or okteto manifest")
	cmd.Flags().BoolVarP(&options.BuildToGlobal, "global", "", false, "push the image to the global registry")
	return cmd
}

func validateSelectedServices(selected []string, buildManifest model.ManifestBuild) error {
	invalid := []string{}
	for _, service := range selected {
		if _, ok := buildManifest[service]; !ok {
			invalid = append(invalid, service)

		}
	}
	if len(invalid) != 0 {
		return fmt.Errorf("invalid services names, not found at manifest: %v", invalid)
	}
	return nil
}

func isSelectedService(service string, selected []string) bool {
	for _, s := range selected {
		if s == service {
			return true
		}
	}
	return false
}

func buildV2(manifest *model.Manifest, cmdOptions *build.BuildOptions, args []string) error {
	buildManifest := manifest.Build
	selectedArgs := []string{}
	if len(args) != 0 {
		selectedArgs = args
	}
	buildSelected := len(selectedArgs) > 0
	isSingleService := len(selectedArgs) == 1

	// cmd flags only allowed when single service build
	if !isSingleService && (cmdOptions.Tag != "" ||
		cmdOptions.Target != "" ||
		cmdOptions.CacheFrom != nil ||
		cmdOptions.Secrets != nil ||
		cmdOptions.ExportCache != "") {
		return fmt.Errorf("flags only allowed when building a single image with `okteto build [NAME]`")
	}

	if buildSelected {
		err := validateSelectedServices(selectedArgs, buildManifest)
		if err != nil {
			return err
		}
	}
	isStack := manifest.Type == model.StackType
	for service, buildInfo := range buildManifest {
		if buildSelected && !isSelectedService(service, selectedArgs) {
			continue
		}

		if isSingleService {
			if cmdOptions.Target != "" {
				buildInfo.Target = cmdOptions.Target
			}
			if len(cmdOptions.CacheFrom) != 0 {
				buildInfo.CacheFrom = cmdOptions.CacheFrom
			}
			if cmdOptions.Tag != "" {
				buildInfo.Image = cmdOptions.Tag
			}
			if cmdOptions.ExportCache != "" {
				buildInfo.ExportCache = cmdOptions.ExportCache
			}
		}
		if !okteto.Context().IsOkteto && buildInfo.Image == "" {
			return fmt.Errorf("'build.%s.image' is required if your context is not managed by Okteto", service)
		}

		if manifest.Name != "" {
			buildInfo.Name = manifest.Name
		} else if cwd, err := os.Getwd(); err == nil && manifest.Name == "" {
			buildInfo.Name = utils.InferName(cwd)
		}

		volumesToInclude := build.GetVolumesToInclude(buildInfo.VolumesToInclude)
		if len(volumesToInclude) > 0 {
			buildInfo.VolumesToInclude = nil
		}

		if isStack && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
			buildInfo.Image = ""
		}
		cmdOptsFromManifest := build.OptsFromManifest(service, buildInfo, cmdOptions)

		// check if image is at registry and skip
		if build.ShouldOptimizeBuild(cmdOptsFromManifest.Tag) && !cmdOptions.BuildToGlobal {
			oktetoLog.Debug("found OKTETO_GIT_COMMIT, optimizing the build flow")
			globalReference := strings.Replace(cmdOptsFromManifest.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)
			if _, err := registry.GetImageTagWithDigest(globalReference); err == nil {
				oktetoLog.Debugf("Skipping '%s' build. Image already exists at the Okteto Registry", service)
				continue
			}
			if registry.IsDevRegistry(cmdOptsFromManifest.Tag) {
				// check if image already is at the registry
				if _, err := registry.GetImageTagWithDigest(cmdOptsFromManifest.Tag); err == nil {
					oktetoLog.Debugf("skipping build: image %s is already built", cmdOptsFromManifest.Tag)
					continue
				}
			}
		}
		// when single build, transfer the secrets from the flag to the options
		if isSingleService {
			cmdOptsFromManifest.Secrets = cmdOptions.Secrets
		}

		if err := buildV1(cmdOptsFromManifest, []string{cmdOptsFromManifest.Path}); err != nil {
			return err
		}
		if len(volumesToInclude) > 0 {
			oktetoLog.Information("Including volume hosts for service '%s'", service)
			svcBuild, err := registry.CreateDockerfileWithVolumeMounts(cmdOptsFromManifest.Tag, volumesToInclude)
			if err != nil {
				return err
			}
			svcBuild.VolumesToInclude = volumesToInclude
			svcBuild.Name = buildInfo.Name
			options := build.OptsFromManifest(service, svcBuild, cmdOptions)
			if err := buildV1(options, []string{options.Path}); err != nil {
				return err
			}

		}
	}
	return nil
}

func buildV1(options *build.BuildOptions, args []string) error {
	path := "."
	if len(args) == 1 {
		path = args[0]
	}

	if err := utils.CheckIfDirectory(path); err != nil {
		return fmt.Errorf("invalid build context: %s", err.Error())
	}
	options.Path = path

	if options.File == "" {
		options.File = filepath.Join(path, "Dockerfile")
	}

	if err := utils.CheckIfRegularFile(options.File); err != nil {
		return fmt.Errorf("invalid Dockerfile: %s", err.Error())
	}

	buildMsg := "Building the image"
	if options.Tag != "" {
		buildMsg = fmt.Sprintf("Building the image '%s'", options.Tag)
	}
	if okteto.Context().Builder == "" {
		oktetoLog.Information("%s using your local docker daemon", buildMsg)
	} else {
		oktetoLog.Information("%s in %s...", buildMsg, okteto.Context().Builder)
	}

	ctx := context.Background()
	if err := build.Run(ctx, options); err != nil {
		analytics.TrackBuild(okteto.Context().Builder, false)
		return err
	}

	if options.Tag == "" {
		oktetoLog.Success("Build succeeded")
		oktetoLog.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
	} else {
		oktetoLog.Success(fmt.Sprintf("Image '%s' successfully pushed", options.Tag))
	}

	analytics.TrackBuild(okteto.Context().Builder, true)
	return nil
}

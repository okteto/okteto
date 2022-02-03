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
		Use:   "build [PATH]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#build"),
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}
			if okteto.IsOkteto() && options.Namespace != "" {
				create, err := utils.ShouldCreateNamespace(ctx, options.Namespace)
				if err != nil {
					return err
				}
				if create {
					nsCmd, err := namespace.NewCommand()
					if err != nil {
						return err
					}
					nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: options.Namespace})
				}
			}

			ctxOpts := &contextCMD.ContextOptions{
				Namespace: options.Namespace,
			}

			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			if contextCMD.IsManifestV2Enabled() {

				manifest, err := model.GetManifestV2(options.File)
				if err != nil {
					return err
				}

				if manifest.Build != nil {
					if manifest.Namespace != "" {
						ctxOpts.Namespace = manifest.Namespace
					}
					if manifest.Context != "" {
						ctxOpts.Context = manifest.Context
					}
					if manifest.Namespace != "" || manifest.Context != "" {
						if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
							return err
						}
					}

					return buildV2(manifest.Build, options, args)
				}
				oktetoLog.Information("Build manifest not found. Looking for Dockerfile to run the build")
			}

			return buildV1(options, args)
		},
	}

	cmd.Flags().StringVarP(&options.File, "file", "f", "", "name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&options.Tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&options.Target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&options.CacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringVarP(&options.OutputMode, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output")
	cmd.Flags().StringArrayVar(&options.BuildArgs, "build-arg", nil, "set build-time variables")
	cmd.Flags().StringArrayVar(&options.Secrets, "secret", nil, "secret files exposed to the build. Format: id=mysecret,src=/local/secret")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "namespace against which the image will be consumed. Default is the one defined at okteto context or okteto manifest")
	cmd.Flags().BoolVarP(&options.BuildToGlobal, "global", "", false, "push the image to the global registry")
	return cmd
}

func keepOnlySelectedServices(service string, manifest model.ManifestBuild) {
	for key := range manifest {
		if key != service {
			delete(manifest, key)
		}
	}
}

func buildV2(manifest model.ManifestBuild, options build.BuildOptions, args []string) error {
	service := ""
	if len(args) == 1 {
		service = args[0]
	}

	// settings for single build
	if service != "" {
		_, ok := manifest[service]
		if !ok {
			return fmt.Errorf("invalid service name: %s", service)
		}

		keepOnlySelectedServices(service, manifest)

		if options.Target != "" {
			manifest[service].Target = options.Target
		}
		if len(options.CacheFrom) != 0 {
			manifest[service].CacheFrom = options.CacheFrom
		}
		if options.Tag != "" {
			manifest[service].Image = options.Tag
		}

	} else if options.Tag != "" || options.Target != "" || options.CacheFrom != nil || options.Secrets != nil {
		return fmt.Errorf("flags only allowed when building a single image with `okteto build [NAME]`")
	}

	for srv, manifestOptions := range manifest {
		if !okteto.Context().IsOkteto && manifestOptions.Image == "" {
			oktetoLog.Errorf("image is required for service %s", srv)
			continue
		}
		if cwd, err := os.Getwd(); err == nil && manifestOptions.Name == "" {
			manifestOptions.Name = utils.InferApplicationName(cwd)
		}

		opts := build.OptsFromManifest(srv, manifestOptions, options)

		// check if image is at registry and skip
		if build.ShouldOptimizeBuild(opts.Tag) && !options.BuildToGlobal {
			oktetoLog.Debug("found OKTETO_GIT_COMMIT, optimizing the build flow")
			globalReference := strings.Replace(opts.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)
			if _, err := registry.GetImageTagWithDigest(globalReference); err == nil {
				oktetoLog.Information("skipping build: image already exists at global registry -  %s", globalReference)
				return nil
			}
			if registry.IsDevRegistry(opts.Tag) {
				// check if image already is at the registry
				if _, err := registry.GetImageTagWithDigest(opts.Tag); err == nil {
					oktetoLog.Information("skipping build: image already exists at registry - %s", opts.Tag)
					return nil
				}
			}
		}
		// when single build, transfer the secrets from the flag to the options
		if srv == service {
			opts.Secrets = options.Secrets
		}

		err := buildV1(opts, []string{opts.Path})
		if err != nil {
			oktetoLog.Error(err)
			continue
		}
	}
	return nil
}

func buildV1(options build.BuildOptions, args []string) error {
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

	if okteto.Context().Builder == "" {
		oktetoLog.Information("Building your image using your local docker daemon")
	} else {
		oktetoLog.Information("Running your build in %s...", okteto.Context().Builder)
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

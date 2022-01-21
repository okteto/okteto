// Copyright 2021 The Okteto Authors
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

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
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
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}

				manifest, err := contextCMD.GetManifestV2(cwd, options.File)
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
	return cmd
}

func buildV2(m model.ManifestBuild, options build.BuildOptions, args []string) error {
	service := ""
	if len(args) == 1 {
		service = args[0]
	}

	if service != "" {
		buildInfo, ok := m[service]
		if !ok {
			return fmt.Errorf("invalid service name: %s", service)
		}
		if !okteto.Context().IsOkteto && buildInfo.Image == "" {
			return fmt.Errorf("image tag is required when context is not okteto")
		}

		if options.Target != "" {
			buildInfo.Target = options.Target
		}
		if len(options.CacheFrom) != 0 {
			buildInfo.CacheFrom = options.CacheFrom
		}
		if options.Tag != "" {
			buildInfo.Image = options.Tag
		}

		opts := build.OptsFromManifest(service, buildInfo, options)
		opts.Secrets = options.Secrets

		return buildV1(opts, []string{opts.Path})
	}

	if options.Tag != "" || options.Target != "" || options.CacheFrom != nil || options.Secrets != nil {
		return fmt.Errorf("flags are not allowed when building services from manifest")
	}

	for service, buildInfo := range m {
		if !okteto.Context().IsOkteto && buildInfo.Image == "" {
			oktetoLog.Errorf("image is required")
			continue
		}
		opts := build.OptsFromManifest(service, buildInfo, options)
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

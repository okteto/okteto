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
	"path/filepath"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Build build and optionally push a Docker image
func Build(ctx context.Context) *cobra.Command {

	options := build.BuildOptions{}
	cmd := &cobra.Command{
		Use:   "build [service]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#build"),
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := contextCMD.Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}

			service := ""
			if len(args) == 1 {
				service = args[0]
			}

			manifestPath := "."
			if options.File != "" {
				if err := utils.CheckIfRegularFile(options.File); err != nil {
					return fmt.Errorf("invalid File: %s", err.Error())
				}
				manifestPath = options.File
			} else {
				manifestPath = contextCMD.GetOktetoManifestPath(manifestPath)
			}
			if manifestPath != "" {
				buildManifest, err := model.GetBuildManifest(manifestPath)
				if err != nil {
					return err
				}
				if len(buildManifest) != 0 {
					if service != "" {
						b, ok := buildManifest[service]
						if !ok {
							return fmt.Errorf("invalid service name")
						}

						if options.Target != "" {
							b.Target = options.Target
						}
						if len(options.CacheFrom) != 0 {
							b.CacheFrom = options.CacheFrom
						}

						opts := build.OptsFromManifest(service, b)
						opts.Secrets = options.Secrets

						err := buildWithOptions(opts)
						if err != nil {
							return err
						}
					} else {
						for service, b := range buildManifest {
							opts := build.OptsFromManifest(service, b)
							err := buildWithOptions(opts)
							if err != nil {
								return err
							}
						}
					}
					return nil
				}
			}
			log.Warning("Okteto Manifest not found, looking for Dockerfile")

			options.Path = "."
			if len(args) == 1 {
				options.Path = args[0]
			}
			err := buildWithOptions(options)
			if err != nil {
				return err
			}
			return nil

		},
	}

	cmd.Flags().StringVarP(&options.File, "file", "f", "", "name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&options.Tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&options.Target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&options.CacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringVarP(&options.OutputMode, "progress", "", "tty", "show plain/tty build output")
	cmd.Flags().StringArrayVar(&options.BuildArgs, "build-arg", nil, "set build-time variables")
	cmd.Flags().StringArrayVar(&options.Secrets, "secret", nil, "secret files exposed to the build. Format: id=mysecret,src=/local/secret")
	return cmd
}

func buildWithOptions(opts build.BuildOptions) error {

	if err := utils.CheckIfDirectory(opts.Path); err != nil {
		return fmt.Errorf("invalid build context: %s", err.Error())
	}

	if opts.File == "" {
		opts.File = filepath.Join(opts.Path, "Dockerfile")
	}

	if err := utils.CheckIfRegularFile(opts.File); err != nil {
		return fmt.Errorf("invalid Dockerfile: %s", err.Error())
	}

	if okteto.Context().Builder == "" {
		log.Information("Building your image using your local docker daemon")
	} else {
		log.Information("Running your build in %s...", okteto.Context().Builder)
	}

	ctx := context.Background()
	if err := build.Run(ctx, opts); err != nil {
		analytics.TrackBuild(okteto.Context().Builder, false)
		return err
	}

	if opts.Tag == "" {
		log.Success("Build succeeded")
		log.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
	} else {
		log.Success(fmt.Sprintf("Image '%s' successfully pushed", opts.Tag))
	}

	analytics.TrackBuild(okteto.Context().Builder, true)
	return nil
}

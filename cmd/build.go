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
	"strconv"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
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
		Use:   "build [PATH]",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#build"),
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {

			if okteto.IsOkteto() && options.Namespace != "" {
				create, err := utils.ShouldCreateNamespace(ctx, options.Namespace)
				if err != nil {
					return err
				}
				if create {
					err = namespace.ExecuteCreateNamespace(ctx, options.Namespace, nil)
					if err != nil {
						return err
					}
				}
			}

			ctxOpts := &contextCMD.ContextOptions{
				Namespace: options.Namespace,
			}
			if err := contextCMD.Run(ctx, ctxOpts); err != nil {
				return err
			}

			m, ok := isManifestV2(options.File)
			if m != nil && ok {
				return buildV2(m, options, args)
			}
			return buildV1(options, args)
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
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "namespace against which the image will be consumed. Default is the one defined at okteto context or okteto manifest")
	return cmd
}

func isManifestV2(file string) (model.ManifestBuild, bool) {
	r, err := strconv.ParseBool(os.Getenv("OKTETO_ENABLE_MANIFEST_V2"))
	if err != nil || !r {
		return nil, false
	}
	mPath := contextCMD.GetOktetoManifestPath(file)
	if mPath == "" {
		log.Errorf("okteto manifest not found")
		return nil, false
	}

	mBuild, err := model.GetBuildManifest(mPath)
	if err != nil {
		log.Errorf("Error retrieving build manifest, %s", err.Error())
		return nil, false
	}
	if mBuild == nil {
		log.Errorf("build manifest is empty")
		return nil, false
	}
	return mBuild, true
}

func buildV2(m model.ManifestBuild, options build.BuildOptions, args []string) error {
	service := ""
	if len(args) == 1 {
		service = args[0]
	}

	if service != "" {
		buildInfo, ok := m[service]
		if !ok {
			return fmt.Errorf("invalid service name")
		}
		if !okteto.Context().IsOkteto && buildInfo.Image == "" {
			return fmt.Errorf("image is required")
		}

		if options.Target != "" {
			buildInfo.Target = options.Target
		}
		if len(options.CacheFrom) != 0 {
			buildInfo.CacheFrom = options.CacheFrom
		}

		opts := build.OptsFromManifest(service, buildInfo, options)
		opts.Secrets = options.Secrets

		return buildV1(opts, []string{opts.Path})
	}

	for service, buildInfo := range m {
		if !okteto.Context().IsOkteto && buildInfo.Image == "" {
			log.Errorf("image is required")
			continue
		}
		opts := build.OptsFromManifest(service, buildInfo, options)
		err := buildV1(opts, []string{opts.Path})
		if err != nil {
			log.Error(err)
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
		log.Information("Building your image using your local docker daemon")
	} else {
		log.Information("Running your build in %s...", okteto.Context().Builder)
	}

	ctx := context.Background()
	if err := build.Run(ctx, options); err != nil {
		analytics.TrackBuild(okteto.Context().Builder, false)
		return err
	}

	if options.Tag == "" {
		log.Success("Build succeeded")
		log.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
	} else {
		log.Success(fmt.Sprintf("Image '%s' successfully pushed", options.Tag))
	}

	analytics.TrackBuild(okteto.Context().Builder, true)
	return nil
}

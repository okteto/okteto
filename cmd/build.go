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

			if err := contextCMD.Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}

			if options.Tag != "" {
				path := "."
				if len(args) == 1 {
					path = args[0]
				}

				err := buildTaggedImage(options, path)
				if err != nil {
					return err
				}
				return nil
			}

			manifestPath := contextCMD.GetOktetoManifestPath()
			if manifestPath == "" {
				return fmt.Errorf("no okteto manifest found")
			}

			image := ""
			if len(args) == 1 {
				image = args[0]
			}

			buildManifest, err := model.GetBuildManifest(manifestPath)
			if err != nil {
				return err
			}

			if image != "" {
				i, ok := buildManifest[image]
				if !ok {
					return fmt.Errorf("image was not found at build manifest")
				}

				err := buildFromManifest(image, i)
				if err != nil {
					return err
				}

			} else {
				for name, i := range buildManifest {
					err := buildFromManifest(name, i)
					if err != nil {
						return err
					}
				}

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

func buildTaggedImage(options build.BuildOptions, path string) error {

	if err := utils.CheckIfDirectory(path); err != nil {
		return fmt.Errorf("invalid build context: %s", err.Error())
	}
	options.Path = path

	options.File = filepath.Join(path, "Dockerfile")

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

	log.Success(fmt.Sprintf("Image '%s' successfully pushed", options.Tag))

	analytics.TrackBuild(okteto.Context().Builder, true)
	return nil
}

func buildFromManifest(name string, i *model.BuildInfo) error {
	if i.Image == "" {
		i.Image = setOktetoImageTag(name)
	}

	opts := build.BuildOptions{
		BuildArgs: model.SerializeBuildArgs(i.Args),
		CacheFrom: i.CacheFrom,
		Target:    i.Target,
		File:      i.Dockerfile,
		Path:      i.Context,
		Tag:       i.Image,
	}

	err := buildTaggedImage(opts, i.Context)
	if err != nil {
		return err
	}
	return nil
}

func setOktetoImageTag(name string) string {
	imageTag := "dev"
	okGitCommit := os.Getenv("OKTETO_GIT_COMMIT")
	if okGitCommit != "" {
		imageTag = okGitCommit
	}
	return fmt.Sprintf("%s/%s:%s", okteto.DevRegistry, name, imageTag)
}

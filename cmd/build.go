// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/login"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/cobra"
)

//Build build and optionally push a Docker image
func Build(ctx context.Context) *cobra.Command {
	var file string
	var tag string
	var target string
	var noCache bool
	var progress string
	var buildArgs []string

	cmd := &cobra.Command{
		Use:   "build [PATH]",
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting build command")

			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			dev, err := utils.LoadDevOrDefault(utils.DefaultDevManifest, "build")
			if err != nil {
				return err
			}
			path := ""
			if len(args) == 1 {
				path = args[0]
			}
			overwriteFieldsWithArgs(dev, path, file, tag, target)
			if len(buildArgs) == 0 {
				buildArgs = model.SerializeBuildArgs(dev.Build.Args)
			}

			buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
			if err != nil {
				return err
			}

			if _, err := build.Run(buildKitHost, isOktetoCluster, dev.Build.Context, dev.Build.Dockerfile, dev.Image, dev.Build.Target, noCache, buildArgs, progress); err != nil {
				analytics.TrackBuild(false)
				return err
			}
			if dev.Image == "" {
				log.Success("Build succeeded")
				log.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
			} else {
				log.Success(fmt.Sprintf("Image '%s' successfully pushed", dev.Image))
			}
			analytics.TrackBuild(true)
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&noCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringVarP(&progress, "progress", "", "tty", "show plain/tty build output")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "set build-time variables")
	return cmd
}

func overwriteFieldsWithArgs(dev *model.Dev, path, file, tag, target string) {
	if path != "" {
		dev.Build.Context = path
	}
	if file != "" {
		dev.Build.Dockerfile = file
	} else {
		dev.Build.Dockerfile = filepath.Join(dev.Build.Context, "Dockerfile")
	}
	if tag != "" {
		dev.Image = tag
	}
	if target != "" {
		dev.Build.Target = target
	}
}

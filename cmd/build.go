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
	"github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

//Build build and optionally push a Docker image
func Build(ctx context.Context) *cobra.Command {
	var file string
	var tag string
	var target string
	var noCache bool
	var cacheFrom []string
	var progress string
	var buildArgs []string

	cmd := &cobra.Command{
		Use:   "build [PATH]",
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {

			if err := login.WithEnvVarIfAvailable(ctx); err != nil {
				return err
			}

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			if err := utils.CheckIfDirectory(path); err != nil {
				return fmt.Errorf("invalid build context: %s", err.Error())
			}

			if file == "" {
				file = filepath.Join(path, "Dockerfile")
			}

			if err := utils.CheckIfRegularFile(file); err != nil {
				return fmt.Errorf("invalid Dockerfile: %s", err.Error())
			}

			buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
			if err != nil {
				return err
			}
			log.Information("Running your build in %s...", buildKitHost)

			_, _, namespace, err := client.GetLocal("")
			if err != nil {
				return fmt.Errorf("failed to load your local Kubeconfig: %s", err)
			}

			ctx := context.Background()
			if err := build.Run(ctx, namespace, buildKitHost, isOktetoCluster, path, file, tag, target, noCache, cacheFrom, buildArgs, progress); err != nil {
				analytics.TrackBuild(false)
				return err
			}

			if tag == "" {
				log.Success("Build succeeded")
				log.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
			} else {
				log.Success(fmt.Sprintf("Image '%s' successfully pushed", tag))
			}

			analytics.TrackBuild(true)
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&noCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&cacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringVarP(&progress, "progress", "", "tty", "show plain/tty build output")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "set build-time variables")
	return cmd
}

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
	"strings"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/login"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

//Build build and optionally push a Docker image
func Build(ctx context.Context) *cobra.Command {
	var file string
	var tag string
	var target string
	var noCache bool
	var cacheFrom string
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

			path := "."
			if len(args) == 1 {
				path = args[0]
			}

			buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
			if err != nil {
				return err
			}

			tag, err = expandOktetoDevRegistry(tag)
			if err != nil {
				return err
			}

			if _, err := build.Run(buildKitHost, isOktetoCluster, path, file, tag, target, noCache, cacheFrom, buildArgs, progress); err != nil {
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
	cmd.Flags().StringVarP(&cacheFrom, "cache-from", "", "", "cache source image")
	cmd.Flags().StringVarP(&progress, "progress", "", "tty", "show plain/tty build output")
	cmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "set build-time variables")
	return cmd
}

func expandOktetoDevRegistry(tag string) (string, error) {
	if !strings.HasPrefix(tag, okteto.DevRegistry) {
		return tag, nil
	}

	c, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return "", fmt.Errorf("failed to load your local Kubeconfig: %s", err)
	}
	n, err := namespaces.Get(namespace, c)
	if err != nil {
		return "", fmt.Errorf("failed to get your current namespace '%s': %s", namespace, err.Error())
	}
	if !namespaces.IsOktetoNamespace(n) {
		return "", fmt.Errorf("cannot use the okteto.dev container registry: your current namespace '%s' is not managed by okteto", namespace)
	}

	oktetoRegistryURL, err := okteto.GetRegistry()
	if err != nil {
		return "", fmt.Errorf("cannot use the okteto.dev container registry: unable to get okteto registry url: %s", err)
	}

	oldTag := tag
	tag = strings.Replace(tag, okteto.DevRegistry, fmt.Sprintf("%s/%s", oktetoRegistryURL, namespace), 1)

	log.Information("'%s' expanded to '%s'.", oldTag, tag)
	return tag, nil
}

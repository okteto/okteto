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
	"net/url"
	"os"
	"path/filepath"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials/oauth"
)

//Build build and optionally push a Docker image
func Build() *cobra.Command {
	var file string
	var tag string
	var target string
	var noCache bool

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting build command")
			ctx := context.Background()
			buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
			if err != nil {
				return err
			}
			if _, err := RunBuild(ctx, buildKitHost, isOktetoCluster, args[0], file, tag, target, noCache); err != nil {
				analytics.TrackBuild(false)
				return err
			}
			analytics.TrackBuild(true)
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("build requires the PATH context argument (e.g. '.' for the current directory)")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&noCache, "no-cache", "", false, "do not use cache when building the image")
	return cmd
}

//RunBuild starts the build sequence
func RunBuild(ctx context.Context, buildKitHost string, isOktetoCluster bool, path, file, tag, target string, noCache bool) (string, error) {

	b, err := url.Parse(buildKitHost)
	if err != nil {
		return "", errors.Wrapf(err, "invalid buildkit host %s", buildKitHost)
	}

	okToken, err := okteto.GetToken()
	if err != nil {
		return "", errors.Wrapf(err, "invalid okteto token, please run `okteto login` again")
	}

	creds := client.WithCredentials(b.Hostname(), okteto.GetCertificatePath(), "", "")

	oauthToken := &oauth2.Token{
		AccessToken: okToken.Token,
	}

	rpc := client.WithRPCCreds(oauth.NewOauthAccess(oauthToken))
	c, err := client.New(ctx, buildKitHost, client.WithFailFast(), creds, rpc)
	if err != nil {
		return "", errors.Wrap(err, "failed to create build client")
	}

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	if file == "" {
		file = filepath.Join(path, "Dockerfile")
	}
	if isOktetoCluster {
		fileWithCacheHandler, err := build.GetDockerfileWithCacheHandler(file)
		if err != nil {
			return "", errors.Wrap(err, "failed to create temporary build folder")
		}
		defer os.Remove(fileWithCacheHandler)
		file = fileWithCacheHandler
	}

	solveOpt, err := build.GetSolveOpt(path, file, tag, target, noCache)
	if err != nil {
		return "", errors.Wrap(err, "failed to create build solver")
	}

	if tag == "" {
		log.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
	}

	var solveResp *client.SolveResponse
	eg.Go(func() error {
		var err error
		solveResp, err = c.Solve(ctx, nil, *solveOpt, ch)
		return errors.Wrap(err, "build failed")
	})

	eg.Go(func() error {
		var c console.Console
		if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cn
		}
		// not using shared context to not disrupt display but let it finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, ch)
	})

	if err := eg.Wait(); err != nil {
		return "", err
	}

	return solveResp.ExporterResponse["containerimage.digest"], nil
}

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
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/buildkit"
	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"
)

const (
	frontend          = "dockerfile.v0"
	buildKitContainer = "buildkit-0"
	buildKitPort      = 1234
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
			if err := RunBuild(args[0], file, tag, target, noCache); err != nil {
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
func RunBuild(path, file, tag, target string, noCache bool) error {
	ctx := context.Background()

	buildKitHost, err := getBuildKitHost()
	if err != nil {
		return err
	}

	c, err := client.New(ctx, buildKitHost, client.WithFailFast())
	if err != nil {
		return err
	}

	log.Infof("created buildkit client: %+v", c)

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)
	solveOpt, err := getSolveOpt(path, file, tag, target, noCache)
	if err != nil {
		return err
	}

	eg.Go(func() error {
		_, err := c.Solve(ctx, nil, *solveOpt, ch)
		return err
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
		return err
	}

	return nil
}

func getBuildKitHost() (string, error) {
	buildKitHost := os.Getenv("BUILDKIT_HOST")
	if buildKitHost != "" {
		return buildKitHost, nil
	}

	ctx := context.Background()
	c, restConfig, namespace, err := okteto.GetOktetoInternalNamespaceClient(ctx)
	if err != nil {
		return "", fmt.Errorf("The variable 'BUILDKIT_HOST' is not defined and Okteto cannot be queried: %s", err)
	}

	localPort, err := model.GetAvailablePort()
	if err != nil {
		return "", err
	}

	forwarder := forward.NewPortForwardManager(ctx, restConfig, c)
	if err := forwarder.Add(model.Forward{Local: localPort, Remote: buildKitPort}); err != nil {
		return "", err
	}

	if err := forwarder.Start(buildKitContainer, namespace); err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://localhost:%d", localPort), nil
}

func getSolveOpt(buildCtx, file, imageTag, target string, noCache bool) (*client.SolveOpt, error) {
	if file == "" {
		file = filepath.Join(buildCtx, "Dockerfile")
	}
	localDirs := map[string]string{
		"context":    buildCtx,
		"dockerfile": filepath.Dir(file),
	}

	frontendAttrs := map[string]string{
		"filename": filepath.Base(file),
	}
	if target != "" {
		frontendAttrs["target"] = target
	}
	if noCache {
		frontendAttrs["no-cache"] = ""
	}

	attachable := []session.Attachable{}
	if strings.HasPrefix(imageTag, okteto.RegistryURL) && okteto.GetURL() == okteto.CloudURL {
		// set Okteto Cloud credentials
		token, err := okteto.GetToken()
		if err != nil {
			return nil, err
		}
		attachable = append(attachable, buildkit.NewRegistryAuthProvider(okteto.RegistryURL, okteto.GetUserID(), token.Token))
	} else {
		// read docker credentials from `.docker/config.json`
		attachable = append(attachable, authprovider.NewDockerAuthProvider(os.Stderr))
	}
	opt := &client.SolveOpt{
		LocalDirs:     localDirs,
		Frontend:      frontend,
		FrontendAttrs: frontendAttrs,
		Session:       attachable,
	}

	if imageTag != "" {
		opt.Exports = []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"name": imageTag,
					"push": "true",
				},
			},
		}
		opt.CacheExports = []client.CacheOptionsEntry{
			{
				Type: "inline",
			},
		}
		opt.CacheImports = []client.CacheOptionsEntry{
			{
				Type:  "registry",
				Attrs: map[string]string{"ref": imageTag},
			},
		}
	}

	return opt, nil
}

func getRepoNameWithoutTag(name string) string {
	var domain, remainder string
	i := strings.IndexRune(name, '/')
	if i == -1 || (!strings.ContainsAny(name[:i], ".:") && name[:i] != "localhost") {
		domain, remainder = "", name
	} else {
		domain, remainder = name[:i], name[i+1:]
	}
	i = strings.LastIndex(remainder, ":")
	if i == -1 {
		return name
	}
	if domain == "" {
		return remainder[:i]
	}
	return fmt.Sprintf("%s/%s", domain, remainder[:i])
}

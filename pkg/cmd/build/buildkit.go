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

package build

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	frontend = "dockerfile.v0"
)

// getSolveOpt returns the buildkit solve options
func getSolveOpt(buildOptions BuildOptions) (*client.SolveOpt, error) {
	var localDirs map[string]string
	var frontendAttrs map[string]string

	if uri, err := url.ParseRequestURI(buildOptions.Path); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {

		if buildOptions.File == "" {
			buildOptions.File = filepath.Join(buildOptions.Path, "Dockerfile")
		}
		if _, err := os.Stat(buildOptions.File); os.IsNotExist(err) {
			return nil, fmt.Errorf("Dockerfile '%s' does not exist", buildOptions.File)
		}
		localDirs = map[string]string{
			"context":    buildOptions.Path,
			"dockerfile": filepath.Dir(buildOptions.File),
		}
		frontendAttrs = map[string]string{
			"filename": filepath.Base(buildOptions.File),
		}
	} else {
		frontendAttrs = map[string]string{
			"context": buildOptions.Path,
		}
	}

	if buildOptions.Target != "" {
		frontendAttrs["target"] = buildOptions.Target
	}
	if buildOptions.NoCache {
		frontendAttrs["no-cache"] = ""
	}
	for _, buildArg := range buildOptions.BuildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid build-arg value %s", buildArg)
		}
		frontendAttrs["build-arg:"+kv[0]] = kv[1]
	}
	attachable := []session.Attachable{}
	if okteto.IsOkteto() {
		attachable = append(attachable, newDockerAndOktetoAuthProvider(okteto.Context().Registry, okteto.Context().UserID, okteto.Context().Token, os.Stderr))
	} else {
		attachable = append(attachable, authprovider.NewDockerAuthProvider(os.Stderr))
	}

	if len(buildOptions.Secrets) > 0 {
		secretProvider, err := build.ParseSecret(buildOptions.Secrets)
		if err != nil {
			return nil, err
		}
		attachable = append(attachable, secretProvider)
	}
	opt := &client.SolveOpt{
		LocalDirs:     localDirs,
		Frontend:      frontend,
		FrontendAttrs: frontendAttrs,
		Session:       attachable,
		CacheImports:  []client.CacheOptionsEntry{},
	}

	if buildOptions.Tag != "" {
		opt.Exports = []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"name": buildOptions.Tag,
					"push": "true",
				},
			},
		}
	}
	for _, cacheFromImage := range buildOptions.CacheFrom {
		opt.CacheImports = append(
			opt.CacheImports,
			client.CacheOptionsEntry{
				Type:  "registry",
				Attrs: map[string]string{"ref": cacheFromImage},
			},
		)
	}

	return opt, nil
}

func getBuildkitClient(ctx context.Context) (*client.Client, error) {
	buildkitHost := okteto.Context().Buildkit
	octxStore := okteto.ContextStore()
	for name, octx := range octxStore.Contexts {
		//if a context configures buildkit with an Okteto Cluster
		if okteto.IsOktetoURL(name) && octx.Buildkit == buildkitHost {
			okteto.Context().Token = octx.Token
			okteto.Context().Certificate = octx.Certificate
		}
	}
	if okteto.Context().Certificate != "" {
		certBytes, err := base64.StdEncoding.DecodeString(okteto.Context().Certificate)
		if err != nil {
			return nil, fmt.Errorf("certificate decoding error: %w", err)
		}

		if err := os.WriteFile(config.GetCertificatePath(), certBytes, 0600); err != nil {
			return nil, err
		}

		c, err := getClientForOktetoCluster(ctx)
		if err != nil {
			log.Infof("failed to create okteto build client: %s", err)
			return nil, fmt.Errorf("failed to create the builder client: %v", err)
		}

		return c, nil
	}

	c, err := client.New(ctx, okteto.Context().Buildkit, client.WithFailFast())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create the builder client for %s", okteto.Context().Buildkit)
	}
	return c, nil
}

func getClientForOktetoCluster(ctx context.Context) (*client.Client, error) {

	b, err := url.Parse(okteto.Context().Buildkit)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid buildkit host %s", okteto.Context().Buildkit)
	}

	creds := client.WithCredentials(b.Hostname(), config.GetCertificatePath(), "", "")

	oauthToken := &oauth2.Token{
		AccessToken: okteto.Context().Token,
	}

	rpc := client.WithRPCCreds(oauth.NewOauthAccess(oauthToken))
	c, err := client.New(ctx, okteto.Context().Buildkit, client.WithFailFast(), creds, rpc)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func solveBuild(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string) error {
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		_, err = c.Solve(ctx, nil, *opt, ch)
		return errors.Wrap(err, "build failed")
	})

	eg.Go(func() error {
		var c console.Console
		if progress == "tty" {
			if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
				c = cn
			}
		}
		// not using shared context to not disrupt display but let it finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, ch)
	})

	return eg.Wait()
}

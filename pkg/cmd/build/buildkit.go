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

package build

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	okErrors "github.com/okteto/okteto/pkg/errors"
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

//GetBuildKitHost returns the buildkit url and if Okteto Build Service is configured, or an error
func GetBuildKitHost() (string, bool, error) {
	buildKitHost := os.Getenv("BUILDKIT_HOST")
	if buildKitHost != "" {
		log.Information("Running your build in %s...", buildKitHost)
		return buildKitHost, false, nil
	}
	buildkitURL, err := okteto.GetBuildKit()
	if err != nil {
		return "", false, err
	}
	if buildkitURL == okteto.CloudBuildKitURL {
		log.Information("Running your build in Okteto Cloud...")
	} else {
		log.Information("Running your build in Okteto Enterprise...")
	}
	return buildkitURL, true, err
}

//getSolveOpt returns the buildkit solve options
func getSolveOpt(buildCtx, file, imageTag, target string, noCache bool, buildArgs []string) (*client.SolveOpt, error) {
	if file == "" {
		file = filepath.Join(buildCtx, "Dockerfile")
	}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, fmt.Errorf("Dockerfile '%s' does not exist", file)
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
	for _, buildArg := range buildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid build-arg value %s", buildArg)
		}
		frontendAttrs["build-arg:"+kv[0]] = kv[1]
	}
	attachable := []session.Attachable{}
	token, err := okteto.GetToken()
	if err == nil {
		registryURL, err := okteto.GetRegistry()
		if err != nil {
			return nil, err
		}
		attachable = append(attachable, newDockerAndOktetoAuthProvider(registryURL, okteto.GetUserID(), token.Token, os.Stderr))
	} else {
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

func getDockerFile(path, dockerFile string, isOktetoCluster bool) (string, error) {
	if dockerFile == "" {
		dockerFile = filepath.Join(path, "Dockerfile")
	}

	if !isOktetoCluster {
		return dockerFile, nil
	}

	fileWithCacheHandler, err := getDockerfileWithCacheHandler(dockerFile)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary build folder")
	}

	return fileWithCacheHandler, nil
}

func getBuildkitClient(ctx context.Context, isOktetoCluster bool, buildKitHost string) (*client.Client, error) {
	if isOktetoCluster {
		c, err := getClientForOktetoCluster(ctx, buildKitHost)
		if err != nil {
			log.Infof("failed to create okteto build client: %s", err)
			return nil, okErrors.UserError{E: fmt.Errorf("failed to create okteto build client"), Hint: okErrors.ErrNotLogged.Error()}
		}

		return c, nil
	}

	c, err := client.New(ctx, buildKitHost, client.WithFailFast())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create build client for %s", buildKitHost)
	}
	return c, nil
}

func getClientForOktetoCluster(ctx context.Context, buildKitHost string) (*client.Client, error) {

	b, err := url.Parse(buildKitHost)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid buildkit host %s", buildKitHost)
	}

	okToken, err := okteto.GetToken()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get the token")
	}

	if len(okToken.Token) == 0 {
		return nil, fmt.Errorf("auth token missing from token file")
	}

	creds := client.WithCredentials(b.Hostname(), okteto.GetCertificatePath(), "", "")

	oauthToken := &oauth2.Token{
		AccessToken: okToken.Token,
	}

	rpc := client.WithRPCCreds(oauth.NewOauthAccess(oauthToken))
	c, err := client.New(ctx, buildKitHost, client.WithFailFast(), creds, rpc)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func solveBuild(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string) (string, error) {
	var solveResp *client.SolveResponse
	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		solveResp, err = c.Solve(ctx, nil, *opt, ch)
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

	if err := eg.Wait(); err != nil {
		return "", err
	}

	return solveResp.ExporterResponse["containerimage.digest"], nil
}

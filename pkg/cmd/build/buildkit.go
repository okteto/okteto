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
	"os"
	"path/filepath"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/okteto/okteto/pkg/buildkit"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/errgroup"
)

const (
	frontend = "dockerfile.v0"
)

//Run starts the build sequence
func Run(buildKitHost string, isOktetoCluster bool, path, file, tag, target string, noCache bool) (string, error) {
	ctx := context.Background()

	c, err := client.New(ctx, buildKitHost, client.WithFailFast())
	if err != nil {
		return "", err
	}

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)

	if file == "" {
		file = filepath.Join(path, "Dockerfile")
	}
	if isOktetoCluster {
		fileWithCacheHandler, err := getDockerfileWithCacheHandler(file)
		if err != nil {
			return "", fmt.Errorf("failed to create temporary build folder: %s", err)
		}
		defer os.Remove(fileWithCacheHandler)
		file = fileWithCacheHandler
	}

	solveOpt, err := getSolveOpt(path, file, tag, target, noCache)
	if err != nil {
		return "", err
	}

	if tag == "" {
		log.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
	}

	var solveResp *client.SolveResponse
	eg.Go(func() error {
		var err error
		solveResp, err = c.Solve(ctx, nil, *solveOpt, ch)
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
		return "", err
	}

	return solveResp.ExporterResponse["containerimage.digest"], nil
}

//GetBuildKitHost returns thee buildkit url
func GetBuildKitHost() (string, bool, error) {
	buildKitHost := os.Getenv("BUILDKIT_HOST")
	if buildKitHost != "" {
		log.Information("Running your build in %s", buildKitHost)
		return buildKitHost, false, nil
	}
	buildkitURL, err := okteto.GetBuildKit()
	if err != nil {
		if err == errors.ErrNotLogged {
			return "", false, fmt.Errorf("please run 'okteto login [URL]' to build your images in Okteto Cloud for free or set the variable 'BUILDKIT_HOST' to point to your own BuildKit instance")
		}
		return "", false, err
	}
	if buildkitURL == okteto.CloudBuildKitURL {
		log.Information("Running your build in Okteto Cloud")
	} else {
		log.Information("Running your build in Okteto Enterprise")
	}
	return buildkitURL, true, err
}

func getSolveOpt(buildCtx, file, imageTag, target string, noCache bool) (*client.SolveOpt, error) {
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

	attachable := []session.Attachable{}
	token, err := okteto.GetToken()
	if err == nil {
		registryURL, err := okteto.GetRegistry()
		if err != nil {
			return nil, err
		}
		attachable = append(attachable, buildkit.NewDockerAndOktetoAuthProvider(registryURL, okteto.GetUserID(), token.Token, os.Stderr))
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

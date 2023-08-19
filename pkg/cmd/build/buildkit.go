// Copyright 2023 The Okteto Authors
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
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	frontend = "dockerfile.v0"
)

type buildWriter struct{}

// getSolveOpt returns the buildkit solve options
func getSolveOpt(buildOptions *types.BuildOptions) (*client.SolveOpt, error) {
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

	if buildOptions.Platform != "" {
		frontendAttrs["platform"] = buildOptions.Platform
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
		ap := newDockerAndOktetoAuthProvider(okteto.Context().Registry, okteto.Context().UserID, okteto.Context().Token, os.Stderr)
		attachable = append(attachable, ap)
	} else {
		attachable = append(attachable, authprovider.NewDockerAuthProvider(os.Stderr))
	}

	for _, sess := range buildOptions.SshSessions {
		oktetoLog.Debugf("mounting ssh agent to build from %s with key %s", sess.Target, sess.Id)
		ssh, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{
			ID:    sess.Id,
			Paths: []string{sess.Target},
		}})

		if err != nil {
			return nil, fmt.Errorf("Failed to mount ssh agent for %s: %w", sess.Id, err)
		}
		attachable = append(attachable, ssh)
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
		CacheExports:  []client.CacheOptionsEntry{},
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

	for _, exportCacheTo := range buildOptions.ExportCache {
		exportType := "inline"
		if exportCacheTo != buildOptions.Tag {
			exportType = "registry"
		}
		opt.CacheExports = append(
			opt.CacheExports,
			client.CacheOptionsEntry{
				Type: exportType,
				Attrs: map[string]string{
					"ref":  exportCacheTo,
					"mode": "max",
				},
			},
		)
	}

	// TODO(#3548): remove when we upgrade buildkit to 0.11
	if len(opt.CacheExports) > 1 {
		opt.CacheExports = opt.CacheExports[:1]
	}

	return opt, nil
}

func getBuildkitClient(ctx context.Context) (*client.Client, error) {
	buildkitHost := okteto.Context().Builder
	octxStore := okteto.ContextStore()
	for _, octx := range octxStore.Contexts {
		// if a context configures buildkit with an Okteto Cluster
		if octx.IsOkteto && octx.Builder == buildkitHost {
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
			oktetoLog.Infof("failed to create okteto build client: %s", err)
			return nil, fmt.Errorf("failed to create the builder client: %v", err)
		}

		return c, nil
	}

	c, err := client.New(ctx, okteto.Context().Builder, client.WithFailFast())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create the builder client for %s", okteto.Context().Builder)
	}
	return c, nil
}

func getClientForOktetoCluster(ctx context.Context) (*client.Client, error) {

	b, err := url.Parse(okteto.Context().Builder)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid buildkit host %s", okteto.Context().Builder)
	}

	creds := client.WithCredentialsAndSystemRoots(b.Hostname(), config.GetCertificatePath(), "", "")

	oauthToken := &oauth2.Token{
		AccessToken: okteto.Context().Token,
	}

	rpc := client.WithRPCCreds(oauth.NewOauthAccess(oauthToken))
	c, err := client.New(ctx, okteto.Context().Builder, client.WithFailFast(), creds, rpc)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func solveBuild(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string) error {
	logFilterRules := []Rule{
		{
			condition:   BuildKitMissingCacheCondition,
			transformer: BuildKitMissingCacheTransformer,
		},
	}
	logFilter := NewBuildKitLogsFilter(logFilterRules)
	ch := make(chan *client.SolveStatus)
	ttyChannel := make(chan *client.SolveStatus)
	plainChannel := make(chan *client.SolveStatus)
	commandFailChannel := make(chan error, 1)

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		_, err = c.Solve(ctx, nil, *opt, ch)
		return errors.Wrap(err, "build failed")
	})

	eg.Go(func() error {
		done := false
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ss, ok := <-ch:
				if ok {
					logFilter.Run(ss, progress)
					plainChannel <- ss
					if progress == oktetoLog.TTYFormat {
						ttyChannel <- ss
					}
				} else {
					done = true
				}
			}
			if done {
				close(plainChannel)
				if progress == oktetoLog.TTYFormat {
					close(ttyChannel)
				}
				break
			}
		}
		return nil
	})

	eg.Go(func() error {

		w := &buildWriter{}
		switch progress {
		case oktetoLog.TTYFormat:
			var c console.Console

			if cn, err := console.ConsoleFromFile(os.Stdout); err == nil {
				c = cn
			} else {
				oktetoLog.Debugf("could not create console from file: %s ", err)
			}
			go func() {
				// We use the plain channel to store the logs into a buffer and then show them in the UI
				if err := progressui.DisplaySolveStatus(context.TODO(), "", nil, w, plainChannel); err != nil {
					oktetoLog.Infof("could not display solve status: %s", err)
				}
			}()
			// not using shared context to not disrupt display but let it finish reporting errors
			// We need to wait until the tty channel is closed to avoid writing to stdout while the tty is being used
			return progressui.DisplaySolveStatus(context.TODO(), "", c, oktetoLog.GetOutputWriter(), ttyChannel)
		case "deploy":
			err := deployDisplayer(context.TODO(), plainChannel, &types.BuildOptions{OutputMode: "deploy"})
			commandFailChannel <- err
			return err
		case "destroy":
			err := deployDisplayer(context.TODO(), plainChannel, &types.BuildOptions{OutputMode: "destroy"})
			commandFailChannel <- err
			return err
		default:
			// not using shared context to not disrupt display but let it finish reporting errors
			return progressui.DisplaySolveStatus(context.TODO(), "", nil, oktetoLog.GetOutputWriter(), plainChannel)
		}
	})

	err := eg.Wait()
	// If the command failed, we want to return the error from the command instead of the buildkit error
	if err != nil {
		select {
		case commandErr := <-commandFailChannel:
			if commandErr != nil {
				return commandErr
			}
			return err
		default:
			return err
		}
	}
	return nil
}

func (*buildWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, msg)
	return 0, nil
}

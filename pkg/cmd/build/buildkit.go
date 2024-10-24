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
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	dockerConfig "github.com/docker/cli/cli/config"
	"github.com/moby/buildkit/client"
	buildkit "github.com/moby/buildkit/cmd/buildctl/build"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/okteto/okteto/pkg/analytics"
	okbuildkit "github.com/okteto/okteto/pkg/build/buildkit"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

const (
	defaultFrontend = "dockerfile.v0"
)

type buildWriter struct{}

// getSolveOpt returns the buildkit solve options
func getSolveOpt(buildOptions *types.BuildOptions, okctx OktetoContextInterface, secretTempFolder string, fs afero.Fs) (*client.SolveOpt, error) {

	if buildOptions.Tag != "" {
		err := validateImages(okctx, buildOptions.Tag)
		if err != nil {
			return nil, err
		}
	}

	imageCtrl := registry.NewImageCtrl(GetRegistryConfigFromOktetoConfig(okctx))
	if okctx.IsOktetoCluster() {
		buildOptions.Tag = imageCtrl.ExpandOktetoDevRegistry(buildOptions.Tag)
		buildOptions.Tag = imageCtrl.ExpandOktetoGlobalRegistry(buildOptions.Tag)
		for i := range buildOptions.CacheFrom {
			buildOptions.CacheFrom[i] = imageCtrl.ExpandOktetoDevRegistry(buildOptions.CacheFrom[i])
			buildOptions.CacheFrom[i] = imageCtrl.ExpandOktetoGlobalRegistry(buildOptions.CacheFrom[i])
		}
		for i := range buildOptions.ExportCache {
			buildOptions.ExportCache[i] = imageCtrl.ExpandOktetoDevRegistry(buildOptions.ExportCache[i])
			buildOptions.ExportCache[i] = imageCtrl.ExpandOktetoGlobalRegistry(buildOptions.ExportCache[i])
		}
	}

	// inject secrets to buildkit from temp folder
	if err := replaceSecretsSourceEnvWithTempFile(afero.NewOsFs(), secretTempFolder, buildOptions); err != nil {
		return nil, fmt.Errorf("%w: secret should have the format 'id=mysecret,src=/local/secret'", err)
	}

	var localDirs map[string]string
	var frontendAttrs map[string]string

	if uri, err := url.ParseRequestURI(buildOptions.Path); err != nil || (uri != nil && (uri.Scheme == "" || uri.Host == "")) {

		if buildOptions.File == "" {
			buildOptions.File = filepath.Join(buildOptions.Path, "Dockerfile")
		}
		if _, err := fs.Stat(buildOptions.File); os.IsNotExist(err) {
			return nil, fmt.Errorf("file '%s' not found: %w", buildOptions.File, err)
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

	frontend := defaultFrontend

	if len(buildOptions.ExtraHosts) > 0 {
		hosts := ""
		for _, eh := range buildOptions.ExtraHosts {
			hosts += fmt.Sprintf("%s=%s,", eh.Hostname, eh.IP)
		}
		frontend = "gateway.v0"
		frontendAttrs["source"] = "docker/dockerfile"
		frontendAttrs["add-hosts"] = strings.TrimSuffix(hosts, ",")
	}

	maxArgFormatParts := 2
	for _, buildArg := range buildOptions.BuildArgs {
		kv := strings.SplitN(buildArg, "=", maxArgFormatParts)
		if len(kv) != maxArgFormatParts {
			return nil, fmt.Errorf("invalid build-arg value %s", buildArg)
		}
		frontendAttrs["build-arg:"+kv[0]] = kv[1]
	}
	attachable := []session.Attachable{}
	if okctx.IsOktetoCluster() {
		apCtx := &authProviderContext{
			isOkteto: okctx.IsOktetoCluster(),
			context:  okctx.GetCurrentName(),
			token:    okctx.GetCurrentToken(),
			cert:     okctx.GetCurrentCertStr(),
		}

		ap := newDockerAndOktetoAuthProvider(okctx.GetRegistryURL(), okctx.GetCurrentUser(), okctx.GetCurrentToken(), apCtx, os.Stderr)
		attachable = append(attachable, ap)
	} else {
		dockerCfg := dockerConfig.LoadDefaultConfigFile(os.Stderr)
		attachable = append(attachable, authprovider.NewDockerAuthProvider(dockerCfg, map[string]*authprovider.AuthTLSConfig{}))
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
		secretProvider, err := buildkit.ParseSecret(buildOptions.Secrets)
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

	if buildOptions.LocalOutputPath != "" {
		opt.Exports = append(opt.Exports, client.ExportEntry{
			Type:      "local",
			OutputDir: buildOptions.LocalOutputPath,
		})
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

func SolveBuild(ctx context.Context, c *client.Client, opt *client.SolveOpt, progress string, ioCtrl *io.Controller) error {
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
			go func() {
				// We use the plain channel to store the logs into a buffer and then show them in the UI
				d, err := progressui.NewDisplay(w, progressui.PlainMode)
				if err != nil {
					// If an error occurs while attempting to create the tty display,
					// fallback to using plain mode on stdout (in contrast to stderr).
					d, err = progressui.NewDisplay(w, progressui.PlainMode)
					if err != nil {
						oktetoLog.Infof("could not display build status: %s", err)
						return
					}
				}
				// not using shared context to not disrupt display but let is finish reporting errors
				if _, err := d.UpdateFrom(context.TODO(), plainChannel); err != nil {
					oktetoLog.Infof("could not display build status: %s", err)
				}
			}()
			// not using shared context to not disrupt display but let it finish reporting errors
			// We need to wait until the tty channel is closed to avoid writing to stdout while the tty is being used
			d, err := progressui.NewDisplay(os.Stdout, progressui.TtyMode)
			if err != nil {
				// If an error occurs while attempting to create the tty display,
				// fallback to using plain mode on stdout (in contrast to stderr).
				d, err = progressui.NewDisplay(os.Stdout, progressui.PlainMode)
				if err != nil {
					oktetoLog.Infof("could not display build status: %s", err)
					return err
				}
			}
			// not using shared context to not disrupt display but let is finish reporting errors
			if _, err := d.UpdateFrom(context.TODO(), ttyChannel); err != nil {
				oktetoLog.Infof("could not display build status: %s", err)
			}
			return err
		case DeployOutputModeOnBuild, DestroyOutputModeOnBuild, TestOutputModeOnBuild:
			err := deployDisplayer(context.TODO(), plainChannel, &types.BuildOptions{OutputMode: progress})
			commandFailChannel <- err
			return err
		default:
			// not using shared context to not disrupt display but let it finish reporting errors
			d, err := progressui.NewDisplay(os.Stdout, progressui.PlainMode)
			if err != nil {
				// If an error occurs while attempting to create the tty display,
				// fallback to using plain mode on stdout (in contrast to stderr).
				d, err = progressui.NewDisplay(os.Stdout, progressui.PlainMode)
				if err != nil {
					oktetoLog.Infof("could not display build status: %s", err)
					return err
				}
			}
			// not using shared context to not disrupt display but let is finish reporting errors
			if _, err := d.UpdateFrom(context.TODO(), plainChannel); err != nil {
				oktetoLog.Infof("could not display build status: %s", err)
			}
			return err
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

func shouldRetryBuild(err error, tag string, okCtx OktetoContextInterface) bool {
	if okbuildkit.IsRetryable(err) {
		oktetoLog.Yellow(`Failed to push '%s' to the registry:
  %s,
  Retrying...`, tag, err.Error())
		analytics.TrackBuildTransientError(true)
		return true
	}

	if err == nil && tag != "" {
		tags := strings.Split(tag, ",")
		reg := registry.NewOktetoRegistry(GetRegistryConfigFromOktetoConfig(okCtx))
		for _, tag := range tags {
			if _, err := reg.GetImageTagWithDigest(tag); err != nil {
				oktetoLog.Yellow(`Failed to push '%s' metadata to the registry:
	  %s,
	  Retrying...`, tag, err.Error())
				analytics.TrackBuildPullError(true)
				return true
			}
		}
	}
	return false
}

func (*buildWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, msg)
	return 0, nil
}

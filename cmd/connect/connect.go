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

package connect

import (
	"context"
	"fmt"
	"os"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// analyticsTrackerInterface is the analytics tracking interface required by this command.
type analyticsTrackerInterface interface {
	TrackUp(*analytics.UpMetricsMetadata)
}

// Options holds the CLI flags for the connect command.
type Options struct {
	Namespace  string
	K8sContext string
	Image      string
	Envs       []string
}

// Connect starts a manifest-free development session against an existing Deployment or StatefulSet.
// It injects a dev container (like okteto up), syncs the local CWD to the container's workdir,
// installs Claude Code via an init container, and leaves the dev container running when the
// local session ends.
func Connect(at analyticsTrackerInterface, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, fs afero.Fs) *cobra.Command {
	opts := &Options{}

	cmd := &cobra.Command{
		Use:   "connect <name>",
		Short: "Connect to an existing Deployment or StatefulSet without an Okteto Manifest",
		Example: `# Connect to the 'api' deployment in the current namespace
okteto connect api

# Connect using a custom dev image
okteto connect api --image myorg/dev:latest
`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if okteto.InDevContainer() {
				return oktetoErrors.ErrNotInDevContainer
			}

			ctx := context.Background()

			ctxOpts := &contextCMD.Options{
				Show:      true,
				Context:   opts.K8sContext,
				Namespace: opts.Namespace,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			name := args[0]
			ns := okteto.GetContext().Namespace

			k8sClient, _, err := okteto.GetK8sClientWithLogger(k8sLogger)
			if err != nil {
				return fmt.Errorf("failed to load k8s client: %w", err)
			}

			manifest, err := inferDevFromDeployment(ctx, name, ns, opts, k8sClient)
			if err != nil {
				return err
			}

			dev := manifest.Dev[name]

			// SSH keys are required for remote mode.
			if err := sshKeys(); err != nil {
				return err
			}
			dev.LoadRemote(ssh.GetPublicKey())

			if syncthing.ShouldUpgrade() {
				oktetoLog.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					oktetoLog.Infof("failed to upgrade syncthing: %s", err)
					if !syncthing.IsInstalled() {
						return fmt.Errorf("couldn't download syncthing, please try again")
					}
					oktetoLog.Yellow("couldn't upgrade syncthing, will try again later")
				} else {
					oktetoLog.Success("Dependencies successfully installed")
				}
			}

			oktetoLog.ConfigureFileLogger(config.GetAppHome(ns, dev.Name), config.VersionString)

			if err := addStignoreSecrets(dev, ns); err != nil {
				return err
			}

			if err := addSyncFieldHash(dev); err != nil {
				return err
			}

			upMeta := analytics.NewUpMetricsMetadata()
			defer at.TrackUp(upMeta)

			c := &connectContext{
				Namespace:         ns,
				Manifest:          manifest,
				Dev:               dev,
				Exit:              make(chan error, 1),
				StartTime:         time.Now(),
				Options:           opts,
				Fs:                fs,
				analyticsTracker:  at,
				analyticsMeta:     upMeta,
				K8sClientProvider: okteto.NewK8sClientProviderWithLogger(k8sLogger),
			}

			return c.start()
		},
	}

	cmd.Flags().StringVarP(&opts.Image, "image", "i", "", "dev container image (defaults to the deployment's existing image)")
	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "overrides the namespace where the deployment is running")
	cmd.Flags().StringVarP(&opts.K8sContext, "context", "c", "", "kubernetes context to use")
	cmd.Flags().StringArrayVarP(&opts.Envs, "env", "e", []string{}, "environment variable overrides (KEY=VALUE)")

	return cmd
}

func sshKeys() error {
	if !ssh.KeyExists() {
		oktetoLog.Spinner("Generating your client certificates...")
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()

		if err := ssh.GenerateKeys(); err != nil {
			return err
		}

		oktetoLog.Success("Client certificates generated")
	}

	return nil
}

func downloadSyncthing() error {
	t := time.NewTicker(1 * time.Second)
	maxRetries := 2
	var err error
	for i := 0; i < 3; i++ {
		p := &utils.ProgressBar{}
		err = syncthing.Install(p)
		if err == nil {
			return nil
		}

		if i < maxRetries {
			oktetoLog.Infof("failed to download syncthing, retrying: %s", err)
			<-t.C
		}
	}
	return err
}

// applyEnvOverrides merges --env flag values into dev.Environment, following the same pattern
// as loadManifestOverrides in cmd/up.
func applyEnvOverrides(devEnv *env.Environment, envFlags []string) error {
	if len(envFlags) == 0 {
		return nil
	}

	envMap := make(map[string]string, len(*devEnv))
	for _, e := range *devEnv {
		envMap[e.Name] = e.Value
	}

	for _, v := range envFlags {
		parts := splitEnvFlag(v)
		if parts == nil {
			return fmt.Errorf("invalid --env value %q: expected KEY=VALUE", v)
		}
		envMap[parts[0]] = parts[1]
	}

	merged := make(env.Environment, 0, len(envMap))
	for k, v := range envMap {
		merged = append(merged, env.Var{Name: k, Value: v})
	}
	*devEnv = merged
	return nil
}

// splitEnvFlag splits "KEY=VALUE" into [KEY, VALUE], expanding from the OS environment when
// only KEY is provided. Returns nil on invalid input.
func splitEnvFlag(v string) []string {
	for i, c := range v {
		if c == '=' {
			return []string{v[:i], v[i+1:]}
		}
	}
	// KEY with no '=' — expand from environment
	if val, ok := os.LookupEnv(v); ok {
		return []string{v, val}
	}
	return nil
}

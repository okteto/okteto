// Copyright 2024 The Okteto Authors
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

package exec

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	okerrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// execFlags is the input of the user to exec command
type execFlags struct {
	manifestPath string
	namespace    string
	k8sContext   string
}

// metadataTracker is an interface to track metadata
type metadataTracker interface {
	Track(metadata *analytics.TrackExecMetadata)
}

// executorProviderInterface provides an executor for a development container
type executorProviderInterface interface {
	provide(dev *model.Dev, podName string) (executor, error)
}

// Exec executes a command on the remote development container
type Exec struct {
	ioCtrl            *io.Controller
	fs                afero.Fs
	appRetriever      *appRetriever
	k8sClientProvider okteto.K8sClientProvider
	mixpanelTracker   metadataTracker

	executorProvider executorProviderInterface
}

// NewExec creates a new exec command
func NewExec(fs afero.Fs, ioCtrl *io.Controller, k8sProvider okteto.K8sClientProvider) *Exec {
	mixpanelTracker := &mixpannelTrack{
		trackFunc: analytics.TrackExec,
	}
	e := &Exec{
		ioCtrl:            ioCtrl,
		fs:                fs,
		k8sClientProvider: k8sProvider,
		appRetriever:      newAppRetriever(ioCtrl, k8sProvider),
		mixpanelTracker:   mixpanelTracker,
		executorProvider: executorProvider{
			ioCtrl:            ioCtrl,
			k8sClientProvider: k8sProvider,
		},
	}
	return e
}

// Cmd returns the cobra exec command
func (e *Exec) Cmd(ctx context.Context) *cobra.Command {
	execFlags := &execFlags{}

	cmd := &cobra.Command{
		Use:                   "exec (DEV SVC) [flags] -- COMMAND [args...]",
		DisableFlagsInUseLine: true,
		Short:                 "Execute a command in your development container",
		Long: `Executes the provided command or the default shell inside a running pod. 
This command allows you to run tools or perform operations directly on a dev environment that is already running.
For more information on managing pods, refer to the okteto documentation: https://www.okteto.com/docs/`,
		Example: `# Run the 'echo this is a test' command inside the pod named 'my-pod'
okteto exec my-pod -- echo this is a test

# Get an interactive shell session inside the pod named 'my-pod'
okteto exec my-pod`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestOpts := contextCMD.ManifestOptions{Filename: execFlags.manifestPath, Namespace: execFlags.namespace, K8sContext: execFlags.k8sContext}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts, e.fs)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %w", err)
			}

			argsLenAtDash := cmd.ArgsLenAtDash()
			opts, err := newOptions(args, argsLenAtDash)
			if err != nil {
				return fmt.Errorf("failed to create exec options: %w", err)
			}
			if err := opts.setDevFromManifest(ctx, manifest.Dev, okteto.GetContext().Namespace, e.k8sClientProvider, e.ioCtrl); err != nil {
				return okerrors.UserError{
					E:    fmt.Errorf("development containers not found in namespace '%s'", okteto.GetContext().Namespace),
					Hint: "Run 'okteto up' to deploy your development container or use 'okteto context' to change your current context",
				}
			}
			if err := opts.validate(manifest.Dev); err != nil {
				return fmt.Errorf("error validating exec command: %w", err)
			}

			return e.Run(ctx, opts, manifest.Dev[opts.devName])
		},
	}
	cmd.Flags().StringVarP(&execFlags.manifestPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&execFlags.namespace, "namespace", "n", "", "namespace where the exec command is executed")
	cmd.Flags().StringVarP(&execFlags.k8sContext, "context", "c", "", "context where the exec command is executed")
	return cmd
}

// Run executes the exec command
func (e *Exec) Run(ctx context.Context, opts *options, dev *model.Dev) error {
	e.ioCtrl.Logger().Infof("executing command '%s' in development container '%s'", opts.command, opts.devName)

	app, err := e.appRetriever.getApp(ctx, dev)
	if err != nil {
		return okerrors.UserError{
			E:    fmt.Errorf("development containers not found in namespace '%s'", okteto.GetContext().Namespace),
			Hint: "Run 'okteto up' to deploy your development container or use 'okteto context' to change your current context",
		}
	}

	c, _, err := e.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return fmt.Errorf("failed to get k8s client: %w", err)
	}

	pod, err := app.GetRunningPod(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get running pod: %w", err)
	}
	e.ioCtrl.Logger().Infof("executing command '%s' in pod '%s'", opts.command, pod.Name)

	if dev.Container == "" {
		dev.Container = pod.Spec.Containers[0].Name
	}
	e.ioCtrl.Logger().Infof("executing command '%s' in container '%s'", opts.command, dev.Container)
	executor, err := e.executorProvider.provide(dev, pod.Name)
	if err != nil {
		return fmt.Errorf("failed to get executor: %w", err)
	}
	err = executor.execute(ctx, opts.command)
	e.mixpanelTracker.Track(&analytics.TrackExecMetadata{
		Mode:               dev.Mode,
		FirstArgIsDev:      opts.firstArgIsDevName,
		Success:            err == nil,
		IsOktetoRepository: utils.IsOktetoRepo(),
		IsInteractive:      dev.IsInteractive(),
	})
	return err
}

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

type onExecFinish func()

type metadataTracker interface {
	SetMetadata(metadata *analytics.TrackExecMetadata)
	Track()
}

// Exec executes a command on the remote development container
type Exec struct {
	ioCtrl            *io.Controller
	fs                afero.Fs
	appRetriever      *appRetriever
	k8sClientProvider okteto.K8sClientProvider
	mixpannelTracker  metadataTracker

	getExecutorFunc func(dev *model.Dev, podName string) (executor, error)
	onExecFinish    []onExecFinish
}

// NewExec creates a new exec command
func NewExec(fs afero.Fs, ioCtrl *io.Controller) *Exec {
	k8sProvider := okteto.NewK8sClientProvider()
	mixpannelTracker := &mixpannelTrack{
		trackFunc: analytics.TrackExec,
	}
	e := &Exec{
		ioCtrl:            ioCtrl,
		fs:                fs,
		k8sClientProvider: k8sProvider,
		appRetriever:      newAppRetriever(ioCtrl, k8sProvider),
		mixpannelTracker:  mixpannelTracker,
		onExecFinish: []onExecFinish{
			mixpannelTracker.Track,
		},
	}
	e.getExecutorFunc = e.getExecutor
	return e
}

// NewCmdExec returns the cobra exec command
func (e *Exec) NewCmdExec(ctx context.Context) *cobra.Command {
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
			opts := NewOptions(args, argsLenAtDash)
			if err := opts.setDevFromManifest(manifest.Dev, e.ioCtrl); err != nil {
				return fmt.Errorf("failed to set dev from manifest: %w", err)
			}
			if err := opts.Validate(manifest.Dev); err != nil {
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
func (e *Exec) Run(ctx context.Context, opts *Options, dev *model.Dev) error {
	e.ioCtrl.Logger().Infof("executing command '%s' in development container '%s'", opts.command, opts.devName)

	app, err := e.appRetriever.getApp(ctx, dev)
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
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
	executor, err := e.getExecutorFunc(dev, pod.Name)
	if err != nil {
		return fmt.Errorf("failed to get executor: %w", err)
	}
	err = executor.execute(ctx, opts.command)
	e.mixpannelTracker.SetMetadata(&analytics.TrackExecMetadata{
		Mode:               dev.Mode,
		FirstArgIsDev:      opts.firstArgIsDevName,
		Success:            err == nil,
		IsOktetoRepository: utils.IsOktetoRepo(),
		IsInteractive:      dev.IsInteractive(),
	})
	for _, f := range e.onExecFinish {
		f()
	}
	return err
}
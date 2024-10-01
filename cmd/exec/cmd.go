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
	"errors"
	"fmt"

	oargs "github.com/okteto/okteto/cmd/args"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	okerrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
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
	provide(dev *model.Dev, podName, namespace string) (executor, error)
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
		Use:                   "exec [devContainer] [flags] -- COMMAND [args...]",
		DisableFlagsInUseLine: true,
		Short:                 "Execute a command inside your Development Container",
		Example: `# Run the 'echo this is a test' command inside the Development Container 'api'
okteto exec api -- echo this is a test

# Get an interactive shell session inside the Development Container 'api'
okteto exec api -- bash`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if execFlags.manifestPath != "" {
				// check that the manifest file exists
				if !filesystem.FileExistsWithFilesystem(execFlags.manifestPath, e.fs) {
					return okerrors.ErrManifestPathNotFound
				}

				// the Okteto manifest flag should specify a file, not a directory
				if filesystem.IsDir(execFlags.manifestPath, e.fs) {
					return okerrors.ErrManifestPathIsDir
				}
			}

			ctxOpts := &contextCMD.Options{
				Show:      true,
				Context:   execFlags.k8sContext,
				Namespace: execFlags.namespace,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			manifestOpts := contextCMD.ManifestOptions{Filename: execFlags.manifestPath}
			manifest, err := model.GetManifestV2(manifestOpts.Filename, e.fs)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %w", err)
			}

			if !okteto.IsOkteto() {
				if err := manifest.ValidateForCLIOnly(); err != nil {
					return err
				}
			}

			argParser := oargs.NewDevCommandArgParser(oargs.NewDevModeOnLister(e.k8sClientProvider), e.ioCtrl, true)
			argsLenAtDash := cmd.ArgsLenAtDash()

			argsResult, err := argParser.Parse(ctx, args, argsLenAtDash, manifest.Dev, okteto.GetContext().Namespace)
			if err != nil {
				var userErr okerrors.UserError
				if errors.As(err, &userErr) {
					userErr.Hint = "Run 'okteto up' to deploy your development container or use 'okteto context' to change your current context"
					return userErr
				}
				return err
			}

			e.ioCtrl.Out().Infof("Executing command in development container '%s'", argsResult.DevName)
			return e.Run(ctx, argsResult, manifest.Dev[argsResult.DevName], okteto.GetContext().Namespace)
		},
	}
	cmd.Flags().StringVarP(&execFlags.manifestPath, "file", "f", "", "the path to the Okteto Manifest")
	cmd.Flags().StringVarP(&execFlags.namespace, "namespace", "n", "", "overwrite the current Okteto Namespace")
	cmd.Flags().StringVarP(&execFlags.k8sContext, "context", "c", "", "overwrite the current Okteto Context")
	return cmd
}

// Run executes the exec command
func (e *Exec) Run(ctx context.Context, opts *oargs.Result, dev *model.Dev, namespace string) error {
	e.ioCtrl.Logger().Infof("executing command '%s' in development container '%s'", opts.Command, opts.DevName)

	app, err := e.appRetriever.getApp(ctx, dev, namespace)
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
	e.ioCtrl.Logger().Infof("executing command '%s' in pod '%s'", opts.Command, pod.Name)

	if dev.Container == "" {
		dev.Container = pod.Spec.Containers[0].Name
	}
	e.ioCtrl.Logger().Infof("executing command '%s' in container '%s'", opts.Command, dev.Container)
	executor, err := e.executorProvider.provide(dev, pod.Name, namespace)
	if err != nil {
		return fmt.Errorf("failed to get executor: %w", err)
	}
	err = executor.execute(ctx, opts.Command)
	e.mixpanelTracker.Track(&analytics.TrackExecMetadata{
		Mode:               dev.Mode,
		FirstArgIsDev:      opts.FirstArgIsDevName,
		Success:            err == nil,
		IsOktetoRepository: utils.IsOktetoRepo(),
		IsInteractive:      dev.IsInteractive(),
	})
	return err
}

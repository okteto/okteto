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

package cmd

import (
	"context"
	"os"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	initCMD "github.com/okteto/okteto/pkg/cmd/init"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Init creates okteto manifest
func Init() *cobra.Command {
	opts := &initCMD.InitOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#init"),
		Short: "Automatically generate your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			ctxResource := &model.ContextResource{}
			if err := ctxResource.UpdateNamespace(opts.Namespace); err != nil {
				return err
			}

			if err := ctxResource.UpdateContext(opts.Context); err != nil {
				return err
			}
			ctxOptions := &contextCMD.ContextOptions{
				Context:   ctxResource.Context,
				Namespace: ctxResource.Namespace,
				Show:      true,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opts.Workdir = cwd
			opts.ShowCTA = oktetoLog.IsInteractive()
			mc := &initCMD.ManifestCommand{
				K8sClientProvider: okteto.NewK8sClientProvider(),
			}

			if okteto.IsOkteto() {
				_, err = mc.RunInitV2(ctx, opts)
			} else {
				err = mc.RunInitV1(ctx, opts)
			}
			return err
		},
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "namespace target for generating the okteto manifest")
	cmd.Flags().StringVarP(&opts.Context, "context", "c", "", "context target for generating the okteto manifest")
	cmd.Flags().StringVarP(&opts.DevPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&opts.Overwrite, "replace", "r", false, "overwrite existing manifest file")
	cmd.Flags().BoolVarP(&opts.AutoDeploy, "deploy", "", false, "deploy the application after generate the okteto manifest")
	cmd.Flags().BoolVarP(&opts.AutoConfigureDev, "configure-devs", "", false, "configure devs after deploying the application")
	if err := cmd.Flags().MarkHidden("deploy"); err != nil {
		oktetoLog.Infof("failed to mark 'deploy' flag as hidden: %s", err)
	}
	if err := cmd.Flags().MarkHidden("configure-devs"); err != nil {
		oktetoLog.Infof("failed to mark 'configure-devs' flag as hidden: %s", err)
	}
	return cmd
}

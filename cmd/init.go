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
	"github.com/okteto/okteto/cmd/manifest"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

type buildTrackerInterface interface {
	TrackImageBuild(ctx context.Context, meta *analytics.ImageBuildMetadata)
}

type deployTrackerInterface interface {
	TrackDeploy(ctx context.Context, name, namespace string, success bool)
}

type buildDeployTrackerInterface interface {
	buildTrackerInterface
	deployTrackerInterface
}

// Init creates okteto manifest
func Init(at buildTrackerInterface, insights buildDeployTrackerInterface, ioCtrl *io.Controller) *cobra.Command {
	opts := &manifest.InitOpts{}
	cmd := &cobra.Command{
		Use:   "init",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/okteto-cli/#init"),
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
			ctxOptions := &contextCMD.Options{
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
			mc := &manifest.Command{
				K8sClientProvider: okteto.NewK8sClientProvider(),
				AnalyticsTracker:  at,
				InsightsTracker:   insights,
				IoCtrl:            ioCtrl,
			}

			if opts.Version1 {
				if err := mc.RunInitV1(ctx, opts); err != nil {
					return err
				}
			} else {
				_, err := mc.RunInitV2(ctx, opts)
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Namespace, "namespace", "n", "", "namespace target for generating the okteto manifest")
	cmd.Flags().StringVarP(&opts.Context, "context", "c", "", "context target for generating the okteto manifest")
	cmd.Flags().StringVarP(&opts.DevPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().BoolVarP(&opts.Overwrite, "replace", "r", false, "overwrite existing manifest file")
	cmd.Flags().BoolVarP(&opts.Version1, "v1", "", false, "create a v1 okteto manifest: www.okteto.com/docs/0.10/reference/manifest/")
	cmd.Flags().BoolVarP(&opts.AutoDeploy, "deploy", "", false, "deploy the application after generate the okteto manifest")
	cmd.Flags().BoolVarP(&opts.AutoConfigureDev, "configure-devs", "", false, "configure devs after deploying the application")
	return cmd
}

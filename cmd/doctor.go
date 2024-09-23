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
	"errors"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/doctor"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// doctorOptions refers to all the options that can be passed to Doctor command
type doctorOptions struct {
	DevPath    string
	Namespace  string
	K8sContext string
	Dev        string
}

// Doctor generates a zip file with all okteto-related log files
func Doctor(k8sLogger *io.K8sLogger, fs afero.Fs) *cobra.Command {
	doctorOpts := &doctorOptions{}
	cmd := &cobra.Command{
		Use:   "doctor [devContainer]",
		Short: "Generate a doctor file with all the information relevant for troubleshooting an issue. Use it when filing an issue or asking the Okteto community for help",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/okteto-cli/#doctor"),
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Info("starting doctor command")

			if doctorOpts.DevPath != "" {
				// check that the manifest file exists
				if !filesystem.FileExistsWithFilesystem(doctorOpts.DevPath, fs) {
					return oktetoErrors.ErrManifestPathNotFound
				}

				// the Okteto manifest flag should specify a file, not a directory
				if filesystem.IsDir(doctorOpts.DevPath, fs) {
					return oktetoErrors.ErrManifestPathIsDir
				}
			}

			ctx := context.Background()

			ctxOpts := &contextCMD.Options{
				Show:      true,
				Context:   doctorOpts.K8sContext,
				Namespace: doctorOpts.Namespace,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}

			if okteto.InDevContainer() {
				return oktetoErrors.ErrNotInDevContainer
			}

			manifest, err := model.GetManifestV2(doctorOpts.DevPath, afero.NewOsFs())
			if err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				if manifest.Type == model.StackType {
					return oktetoErrors.UserError{
						E: fmt.Errorf("docker Compose format is only available using the Okteto Platform"),
						Hint: `Follow this link to install the Okteto Platform in your Kubernetes cluster:
    https://www.okteto.com/docs/get-started/install`,
					}
				}
				if err := manifest.ValidateForCLIOnly(); err != nil {
					return err
				}
			}
			c, _, err := okteto.GetK8sClientWithLogger(k8sLogger)
			if err != nil {
				return err
			}

			devName := ""
			if len(args) == 1 {
				devName = args[0]
			}
			dev, err := utils.GetDevFromManifest(manifest, devName)
			if err != nil {
				if !errors.Is(err, utils.ErrNoDevSelected) {
					return err
				}
				selector := utils.NewOktetoSelector("Select which development container's logs to download:", "Development container")
				dev, err = utils.SelectDevFromManifest(manifest, selector, manifest.Dev.GetDevs())
				if err != nil {
					return err
				}
			}
			filename, err := doctor.Run(ctx, dev, doctorOpts.DevPath, okteto.GetContext().Namespace, c)
			if err == nil {
				oktetoLog.Information("Your doctor file is available at %s", filename)
			}
			analytics.TrackDoctor(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&doctorOpts.DevPath, "file", "f", "", "the path to the Okteto Manifest")
	cmd.Flags().StringVarP(&doctorOpts.Namespace, "namespace", "n", "", "overwrite the current Okteto Namespace")
	cmd.Flags().StringVarP(&doctorOpts.K8sContext, "context", "c", "", "overwrite the current Okteto Context")
	return cmd
}

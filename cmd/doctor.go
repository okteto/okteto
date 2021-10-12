// Copyright 2021 The Okteto Authors
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

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/doctor"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Doctor generates a zip file with all okteto-related log files
func Doctor() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Generates a zip file with the okteto logs",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#doctor"),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting doctor command")
			ctx := context.Background()

			if okteto.InDevContainer() {
				return errors.ErrNotInDevContainer
			}

			ctxResource, err := utils.LoadDevContext(devPath)
			if err != nil {
				return err
			}

			if err := ctxResource.UpdateNamespace(namespace); err != nil {
				return err
			}

			if err := ctxResource.UpdateContext(k8sContext); err != nil {
				return err
			}
			if err := contextCMD.Init(ctx, ctxResource); err != nil {
				return err
			}

			dev, err := utils.LoadDev(devPath)
			if err != nil {
				return err
			}

			c, _, err := okteto.GetK8sClient()
			if err != nil {
				return err
			}

			filename, err := doctor.Run(ctx, dev, devPath, c)
			if err == nil {
				log.Information("Your doctor file is available at %s", filename)
			}
			analytics.TrackDoctor(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command was executing")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the up command was executing")
	return cmd
}

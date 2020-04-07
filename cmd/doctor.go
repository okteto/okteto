// Copyright 2020 The Okteto Authors
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
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/doctor"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

//Doctor generates a zip file with all okteto-related log files
func Doctor() *cobra.Command {
	var devPath string
	var namespace string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: fmt.Sprintf("Generates a zip file with the okteto logs"),
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Info("starting doctor command")

			if k8Client.InCluster() {
				return errors.ErrNotInCluster
			}

			dev, err := utils.LoadDev(devPath)
			if err != nil {
				return err
			}
			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}

			c, _, namespace, err := k8Client.GetLocal()
			if err != nil {
				return err
			}

			if dev.Namespace == "" {
				dev.Namespace = namespace
			}

			ctx := context.Background()
			filename, err := doctor.Run(ctx, dev, c)
			if err == nil {
				log.Information("Your doctor file is available at %s", filename)
			}
			analytics.TrackDoctor(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command was executing")
	return cmd
}

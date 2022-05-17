// Copyright 2022 The Okteto Authors
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
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/release"
	"github.com/spf13/cobra"
)

func Channel() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel [name]",
		Short: "show or set the current release channel",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#channel"),
		RunE: func(cmd *cobra.Command, args []string) error {
			u := release.UpgradeAvailable()
			fmt.Println("update available: ", u)

			if len(args) > 0 {
				if err := release.UpdateReleaseChannel(args[0]); err != nil {
					return fmt.Errorf("failed to update release channel: %w", err)
				}
				return nil
			}

			rc, err := release.GetReleaseChannel()
			if err != nil {
				return fmt.Errorf("failed to get release channel: %w", err)
			}
			fmt.Print(rc)
			return nil
		},
	}
	return cmd
}

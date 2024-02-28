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
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"
)

// Analytics turns analytics on/off
func Analytics() *cobra.Command {
	var disable bool
	cmd := &cobra.Command{
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/okteto-cli/#analytics"),
		Use:   "analytics",
		Short: "Enable / Disable analytics",
		RunE: func(cmd *cobra.Command, args []string) error {
			if disable {
				return disableAnalytics()
			}

			return enableAnalytics()
		},
	}
	cmd.Flags().BoolVarP(&disable, "disable", "d", false, "disable analytics")
	return cmd
}

func disableAnalytics() error {
	if err := analytics.Disable(); err != nil {
		return err
	}

	oktetoLog.Success("Analytics have been disabled")
	return nil
}

func enableAnalytics() error {
	if err := analytics.Enable(); err != nil {
		return err
	}

	oktetoLog.Success("Analytics have been enabled")
	return nil
}

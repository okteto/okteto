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

package context

import (
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Lists all contexts managed by okteto
func List() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#context"),
		Short:   "Lists all the context managed by okteto",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc, err := okteto.GetOktetoContextConfig()
			if err != nil {
				return err
			}
			for name := range cc.Contexts {
				if name == cc.CurrentContext {
					fmt.Printf("* %s\n", name)
				} else {
					fmt.Printf("  %s\n", name)
				}
			}

			return nil
		},
	}

	return cmd
}

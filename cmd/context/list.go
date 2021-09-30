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
	"sort"

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
		Short:   "Lists okteto contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			oCtxs := okteto.ContextStore()
			contexts := make([]string, 0)
			for name := range oCtxs.Contexts {
				if name == oCtxs.CurrentContext {
					contexts = append(contexts, fmt.Sprintf("* %s", name))
				} else {
					contexts = append(contexts, fmt.Sprintf("  %s", name))
				}
			}
			sort.Slice(contexts, func(i, j int) bool {
				return len(contexts[i]) < len(contexts[j])
			})
			for _, ctx := range contexts {
				fmt.Println(ctx)
			}

			return nil
		},
	}

	return cmd
}

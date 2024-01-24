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

package context

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Show current context
func Show() *cobra.Command {
	var output string
	var includeToken bool
	cmd := &cobra.Command{
		Use:   "show",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#show"),
		Short: "Print the current context",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := NewContextCommand().Run(ctx, &Options{raiseNotCtxError: true}); err != nil {
				return err
			}
			ctxStore := okteto.GetContextStore()
			current := ctxStore.Contexts[ctxStore.CurrentContext]
			if err := validateOutput(output); err != nil {
				return err
			}

			if !includeToken {
				current.Token = ""
			}

			current.Certificate = ""
			switch output {
			case "json":
				bytes, err := json.MarshalIndent(current, "", "  ")
				if err != nil {
					return err
				}
				oktetoLog.Println(string(bytes))
			case "yaml":
				bytes, err := yaml.Marshal(current)
				if err != nil {
					return err
				}
				oktetoLog.Println(string(bytes))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "json", "output format. One of: ['json', 'yaml']")
	cmd.Flags().BoolVar(&includeToken, "include-token", false, "include the token in the output")
	return cmd
}

func validateOutput(output string) error {
	if output != "json" && output != "yaml" {
		return fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'yaml']")
	}
	return nil
}

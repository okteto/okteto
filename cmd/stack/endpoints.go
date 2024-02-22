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

package stack

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/cmd/stack"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// Endpoints show all the endpoints of a stack
func Endpoints(ctx context.Context) *cobra.Command {
	var (
		output    string
		name      string
		namespace string
		stackPath []string
	)
	cmd := &cobra.Command{
		Use:   "endpoints [service...]",
		Short: "Show endpoints for a stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoLog.Warning("'okteto stack endpoints' is deprecated and will be removed in a future version")
			s, err := contextCMD.LoadStackWithContext(ctx, name, namespace, stackPath, afero.NewOsFs())
			if err != nil {
				return err
			}
			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			if err := validateOutput(output); err != nil {
				return err
			}

			if err := stack.ListEndpoints(ctx, s); err != nil {
				oktetoLog.Success("Compose '%s' successfully deployed", s.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output format. One of: ['json']")
	cmd.Flags().StringArrayVarP(&stackPath, "file", "f", []string{}, "path to the compose manifest files. If more than one is passed the latest will overwrite the fields from the previous")
	cmd.Flags().StringVarP(&name, "name", "", "", "overwrites the compose name")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the compose namespace where the compose is deployed")
	return cmd
}

func validateOutput(output string) error {
	if output != "" && output != "json" {
		return fmt.Errorf("output format is not accepted. Value must be one of: ['json']")
	}
	return nil
}

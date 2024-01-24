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

package preview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// Endpoints show all the endpoints of a preview environment
func Endpoints(ctx context.Context) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "endpoints <name>",
		Short: "Show endpoints for a preview environment",
		Args:  utils.ExactArgsAccepted(1, ""),
		RunE: func(cmd *cobra.Command, args []string) error {

			previewName := args[0]

			ctxResource := &model.ContextResource{}
			if err := ctxResource.UpdateNamespace(previewName); err != nil {
				return err
			}

			jsonContextBuffer := bytes.NewBuffer([]byte{})
			if output == "json" {
				oktetoLog.SetOutput(jsonContextBuffer)
			}

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
				return err
			}
			if output != "json" {
				oktetoLog.Information("Using %s @ %s as context", previewName, okteto.RemoveSchema(okteto.GetContext().Name))
			} else {
				oktetoLog.Info(jsonContextBuffer.String())
				oktetoLog.SetOutput(os.Stdout)
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			if err := validateOutput(output); err != nil {
				return err
			}
			err := executeListPreviewEndpoints(ctx, previewName, output)
			return err
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "output format. One of: ['json', 'md']")

	return cmd
}

func validateOutput(output string) error {
	switch output {
	case "", "json", "md":
		return nil
	default:
		return fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'md']")
	}
}

func executeListPreviewEndpoints(ctx context.Context, name, output string) error {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return err
	}
	endpointList, err := oktetoClient.Previews().ListEndpoints(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get preview environments: %w", err)
	}

	switch output {
	case "json":
		bytes, err := json.MarshalIndent(endpointList, "", "  ")
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	case "md":
		if len(endpointList) == 0 {
			oktetoLog.Printf("There are no available endpoints for preview '%s'\n", name)
		} else {
			endpoints := make([]string, 0)
			for _, endpoint := range endpointList {
				endpoints = append(endpoints, endpoint.URL)
			}
			sort.Slice(endpoints, func(i, j int) bool {
				return len(endpoints[i]) < len(endpoints[j])
			})
			oktetoLog.Printf("Available endpoints for preview [%s](%s):\n", name, getPreviewURL(name))
			for _, e := range endpoints {
				oktetoLog.Printf("\n - [%s](%s)\n", e, e)
			}
		}
	default:
		if len(endpointList) == 0 {
			oktetoLog.Printf("There are no available endpoints for preview '%s'\n", name)
		} else {
			endpoints := make([]string, 0)
			for _, endpoint := range endpointList {
				endpoints = append(endpoints, endpoint.URL)
			}
			sort.Slice(endpoints, func(i, j int) bool {
				return len(endpoints[i]) < len(endpoints[j])
			})
			oktetoLog.Printf("Available endpoints for preview '%s':\n  - %s\n", name, strings.Join(endpoints, "\n  - "))
		}
	}
	return nil
}

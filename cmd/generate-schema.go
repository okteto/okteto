// Copyright 2024 The Okteto Authors
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
	"encoding/json"
	"os"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/schema"
	"github.com/spf13/cobra"
)

var outputFilePath string

// GenerateSchema generates and outputs to stdout or file the Okteto Manifest JSON Schema
func GenerateSchema() *cobra.Command {
	cmd := &cobra.Command{
		Args:   cobra.NoArgs,
		Hidden: true,
		Use:    "generate-schema",
		Short:  "Generates the JSON Schema for the Okteto Manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := schema.NewJsonSchema()

			if outputFilePath == "-" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(s)
			}

			f, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer f.Close()

			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			if err := enc.Encode(s); err != nil {
				return err
			}

			oktetoLog.Success("Okteto JSON Schema generated and saved to: %s", outputFilePath)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFilePath, "output-file", "o", "-", "Path to the file where the json schema will be stored")
	return cmd
}

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
	"os"

	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/schema"
	"github.com/spf13/cobra"
)

var outputFilePath string

func toFile(schemaBytes []byte, outputFilePath string) error {
	err := os.WriteFile(outputFilePath, schemaBytes, 0644)
	if err != nil {
		return err
	}
	oktetoLog.Success("okteto json schema has been generated and stored in %s", schemaBytes)

	return nil
}

// GenerateSchema create the json schema for the okteto manifest
func GenerateSchema() *cobra.Command {
	cmd := &cobra.Command{
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#generate-schema"),
		Hidden: true,
		Use:    "generate-schema",
		Short:  "Generates the json schema for the okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := schema.NewJsonSchema()

			json, err := s.ToJSON()
			if err != nil {
				return err
			}
			err = toFile(json, outputFilePath)

			return err
		},
	}

	cmd.Flags().StringVarP(&outputFilePath, "output", "o", "", "Path to the file where the json schema will be stored")
	return cmd
}

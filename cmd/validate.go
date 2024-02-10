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
	"errors"
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/schema"
	"github.com/spf13/cobra"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// Validate validates a Okteto Manifest file
func Validate() *cobra.Command {
	cmd := &cobra.Command{
		Args:  cobra.MaximumNArgs(1),
		Use:   "validate [manifest]",
		Short: "Validate a Okteto Manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			var manifestFile string
			if len(args) > 0 {
				manifestFile = args[0]
			} else {
				// look for okteto.yml or okteto.yaml
				if _, err := os.Stat("okteto.yml"); err == nil {
					manifestFile = "okteto.yml"
				} else if _, err := os.Stat("okteto.yaml"); err == nil {
					manifestFile = "okteto.yaml"
				} else {
					return errors.New("unable to locate manifest file: okteto.yml or okteto.yaml")
				}
			}

			manifest, err := os.ReadFile(manifestFile)
			if err != nil {
				return err
			}

			var obj interface{}
			// TODO: evaluate if returning a friendly err here?
			_ = yaml.Unmarshal(manifest, &obj) //nolint:errcheck

			s := schema.NewJsonSchema()
			json, err := s.ToJSON()
			if err != nil {
				return err
			}

			// Load JSON schema
			jsonLoader := gojsonschema.NewStringLoader(string(json))

			// Load Okteto Manifest
			documentLoader := gojsonschema.NewGoLoader(obj)

			// Validate Okteto Manifest
			result, err := gojsonschema.Validate(jsonLoader, documentLoader)
			if err != nil {
				return err
			}

			if !result.Valid() {
				fmt.Printf("The Okteto Manifest contains the following errors:\n")
				for _, desc := range result.Errors() {
					fmt.Printf("- %s\n", desc)
				}
			}

			return nil
		},
	}

	return cmd
}

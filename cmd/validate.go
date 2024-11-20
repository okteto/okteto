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
	"fmt"
	"github.com/okteto/okteto/pkg/model"
	"gopkg.in/yaml.v2"
	"os"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/discovery"
	"github.com/okteto/okteto/pkg/schema"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type options struct {
	file string
}

func validateOktetoManifest(content string) error {
	oktetoJsonSchema, err := schema.NewJsonSchema().ToJSON()
	if err != nil {
		return err
	}

	var obj interface{}
	_ = schema.Unmarshal([]byte(content), &obj) //nolint:errcheck

	compiler := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(oktetoJsonSchema)))
	if err != nil {
		return err
	}

	resourceName := "schema.json"
	err = compiler.AddResource(resourceName, doc)
	if err != nil {
		return err
	}

	s, err := compiler.Compile(resourceName)
	if err != nil {
		return err
	}

	err = s.Validate(obj)
	if err != nil {
		return err
	}

	return nil
}

// Validate validates a Okteto Manifest file
func Validate(fs afero.Fs) *cobra.Command {
	options := &options{}

	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "validate",
		Short: "Validate the Okteto Manifest file syntax",
		RunE: func(cmd *cobra.Command, args []string) error {
			var manifestFile string
			if options.file != "" {
				manifestFile = options.file
				if err := validator.FileArgumentIsNotDir(fs, manifestFile); err != nil {
					return err
				}
			} else {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				manifestFile, err = discovery.GetOktetoManifestPathWithFilesystem(cwd, fs)
				if err != nil {
					return err
				}
			}

			content, err := os.ReadFile(manifestFile)
			if err != nil {
				return err
			}

			err = validateOktetoManifest(string(content))
			if err != nil {
				re := regexp.MustCompile(`(?m)^.*\n`)
				errStr := re.ReplaceAllString(err.Error(), "")
				var output strings.Builder
				//fmt.Fprintf(&output, "%s\n", oktetoErrors.ErrInvalidManifest)

				var manifest model.Manifest
				err = yaml.UnmarshalStrict(content, &manifest)
				if err != nil {
					friendlyErr := model.NewManifestFriendlyError(err)
					fmt.Fprintf(&output, "%s\n", friendlyErr.Error())
				}
				fmt.Fprintf(&output, "\n    JSON Schema Validation errors:\n")
				fmt.Fprintf(&output, "    %s\n", errStr)
				return fmt.Errorf(output.String())

				//

				//
				//return fmt.Errorf("%s\n\n%v\n\n%v\n\nSee the documentation: https://www.okteto.com/docs/reference/okteto-manifest/", oktetoErrors.ErrInvalidManifest, frie,errStr)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&options.file, "file", "f", "", "the path to the Okteto Manifest or Dockerfile")

	return cmd
}

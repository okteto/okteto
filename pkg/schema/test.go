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

package schema

import "github.com/kubeark/jsonschema"

type test struct{}

func (test) JSONSchema() *jsonschema.Schema {
	// Properties for each test container
	testProps := jsonschema.NewProperties()

	// Artifacts
	testProps.Set("artifacts", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "artifacts",
		Description: "Files and/or folders to be exported after test execution",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// Caches
	testProps.Set("caches", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "caches",
		Description: "Cache mounts to speed up test execution",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// Commands
	testProps.Set("commands", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "commands",
		Description: "Commands to run the tests. Each command must exit with zero exit code for success",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
		Required: []string{"commands"},
	})

	// Context
	testProps.Set("context", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "context",
		Description: "Root folder for running the tests. Defaults to the Okteto Manifest location",
	})

	// Depends On
	testProps.Set("depends_on", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "depends_on",
		Description: "List of Test Containers this test depends on",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	// Hosts
	hostsItemProps := jsonschema.NewProperties()
	hostsItemProps.Set("hostname", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	hostsItemProps.Set("ip", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})

	testProps.Set("hosts", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "hosts",
		Description: "List of hostnames and IPs to add to /etc/hosts during test execution",
		Items: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{
					// Simple string format: "hostname:ip"
					Type:    &jsonschema.Type{Types: []string{"string"}},
					Pattern: "^[a-zA-Z0-9.-]+:[0-9.]+$",
				},
				{
					// Extended object notation
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           hostsItemProps,
					Required:             []string{"hostname", "ip"},
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
	})

	// Image
	testProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "image",
		Description: "Base image used to run your test. Defaults to pipeline-runner if not specified",
	})

	return &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "test",
		Description: "Dictionary of Test Containers to run tests using Remote Execution",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           testProps,
				Required:             []string{"commands"},
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}

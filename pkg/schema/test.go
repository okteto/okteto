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
	testProps := jsonschema.NewProperties()

	testProps.Set("artifacts", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "artifacts",
		Description: withManifestRefDocLink("A list of files and/or folder to be exported after the execution of the tests. They will be added relative to root context of the tests. If you want to export coverage reports and test results this is where they should go.", "artifacts-string-optional"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	testProps.Set("caches", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "caches",
		Description: withManifestRefDocLink("A list of cache mounts to be used as part of running the tests. This is used to speed up recurrent test executions where, for example, dependencies will not be reinstalled and will instead be mounted from the cache.", "caches-string-optional"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	commandProps := jsonschema.NewProperties()
	commandProps.Set("name", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Name of the command",
	})
	commandProps.Set("command", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Command to execute",
	})

	testProps.Set("commands", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "commands",
		Description: withManifestRefDocLink("Provide a list of commands to run the tests. For the tests to be considered successful, each command must exit with a zero exit code. If any command returns a non-zero exit code, the Test Container will be marked as failed", "commands-string-required"),
		Items: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{
					Type: &jsonschema.Type{Types: []string{"string"}},
				},
				{
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           commandProps,
					Required:             []string{"command"},
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
		Required: []string{"commands"},
	})

	testProps.Set("context", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "context",
		Description: withManifestRefDocLink("The folder to use as the root for running the tests. If this is empty, the location of the Okteto Manifest will be used (usually the root of the project).", "context-string-optional"),
	})

	testProps.Set("depends_on", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "depends_on",
		Description: withManifestRefDocLink("A list of Test Containers this test depends on. When a Test Container is executed, all its dependencies are executed first. The Test Containers defined in depends_on must exist in the current Okteto Manifest.", "depends_on-string-optional"),
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

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
		Description: withManifestRefDocLink("A list of hostnames and ips. For each pair, an entry is created in /etc/hosts during the test execution. The following extended notation is also supported: hosts[0].hostname=hostname1 hosts[0].ip=ip1", "hosts-string-optional"),
		Items: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{
					Type:    &jsonschema.Type{Types: []string{"string"}},
					Pattern: "^[a-zA-Z0-9.-]+:[0-9.]+$", // example: hostname:ip
				},
				{
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           hostsItemProps,
					Required:             []string{"hostname", "ip"},
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
	})

	testProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "image",
		Description: withManifestRefDocLink("The base image used to run your test.", "image-string-optional-1"),
	})

	testProps.Set("skipIfNoFileChanges", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Title:       "skipIfNoFileChanges",
		Description: "Skip the test execution if no files have changed since the last test run. This is useful to avoid running tests when the code hasn't changed.",
	})

	return &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
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

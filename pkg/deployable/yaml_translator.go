// Copyright 2025 The Okteto Authors
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

package deployable

import (
	"github.com/okteto/okteto/pkg/divert"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"sigs.k8s.io/yaml"
)

type yamlTranslator struct {
	jsonTranslator *jsonTranslator
}

func newYAMLTranslator(name string, divertDriver divert.Driver) *yamlTranslator {
	return &yamlTranslator{
		jsonTranslator: newJSONTranslator(name, divertDriver),
	}
}

// Translate converts YAML to JSON, processes it using the JSON translator, and converts back to YAML
func (y *yamlTranslator) Translate(b []byte) ([]byte, error) {
	// Convert YAML to JSON
	jsonBytes, err := yaml.YAMLToJSON(b)
	if err != nil {
		oktetoLog.Infof("error converting YAML to JSON on proxy: %s", err.Error())
		return nil, nil
	}

	// Use the JSON translator to process the JSON
	processedJSON, err := y.jsonTranslator.Translate(jsonBytes)
	if err != nil {
		return nil, err
	}

	// Convert JSON back to YAML
	yamlBytes, err := yaml.JSONToYAML(processedJSON)
	if err != nil {
		oktetoLog.Infof("error converting JSON to YAML on proxy: %s", err.Error())
		return nil, nil
	}

	return yamlBytes, nil
}

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

package model

import (
	"bytes"

	"github.com/okteto/okteto/pkg/env"
	yaml3 "gopkg.in/yaml.v3"
)

func expandEnvScalarNode(node *yaml3.Node) (*yaml3.Node, error) {
	if node.Kind == yaml3.ScalarNode {
		// when is a ScalarNode, replace its value with the ENV replaced
		expandValue, err := env.ExpandEnv(node.Value)
		if err != nil {
			return node, err
		}
		node.Value = expandValue
		return node, nil
	}

	for indx, subNode := range node.Content {
		expandedNode, err := expandEnvScalarNode(subNode)
		if err != nil {
			return node, err
		}
		node.Content[indx] = expandedNode
	}
	return node, nil
}

// ExpandStackEnvs returns the stack manifest with expanded envs
func ExpandStackEnvs(file []byte) ([]byte, error) {
	doc := yaml3.Node{}
	if err := yaml3.Unmarshal(file, &doc); err != nil {
		return nil, err
	}

	expandedDoc, err := expandEnvScalarNode(doc.Content[0])
	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(nil)
	encoder := yaml3.NewEncoder(buffer)
	encoder.SetIndent(2)

	err = encoder.Encode(expandedDoc)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil

}

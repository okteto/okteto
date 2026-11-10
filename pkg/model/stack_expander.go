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
		// when it is a ScalarNode, replace its value with the ENV replaced
		expandValue, err := env.ExpandEnv(node.Value)
		if err != nil {
			return node, err
		}
		if expandValue != node.Value && node.Style == 0 && node.ShortTag() == "!!str" {
			// a plain scalar like ${REPLICAS} is resolved as a string before expansion. Keep the value as a
			// number or boolean when it expands to one, otherwise the encoder quotes it and fields like
			// replicas or public fail to unmarshal
			if tag := resolveExpandedScalarTag(expandValue); tag != "" {
				node.Tag = tag
			}
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

// resolveExpandedScalarTag returns the tag of value when it is a plain int, float or bool scalar, or "" otherwise
func resolveExpandedScalarTag(value string) string {
	doc := yaml3.Node{}
	if err := yaml3.Unmarshal([]byte(value), &doc); err != nil || len(doc.Content) != 1 {
		return ""
	}
	node := doc.Content[0]
	if node.Kind != yaml3.ScalarNode || node.Style != 0 || node.Value != value {
		return ""
	}
	switch tag := node.ShortTag(); tag {
	case "!!int", "!!float", "!!bool":
		return tag
	default:
		return ""
	}
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

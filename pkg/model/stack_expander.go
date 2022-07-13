package model

import (
	"bytes"

	yaml3 "gopkg.in/yaml.v3"
)

func expandEnvScalarNode(node *yaml3.Node) (*yaml3.Node, error) {
	if node.Kind == yaml3.ScalarNode {
		// when is a ScalarNode, replace its value with the ENV replaced
		expandValue, err := ExpandEnv(node.Value, true)
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

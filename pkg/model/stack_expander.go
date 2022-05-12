package model

import (
	"bytes"
	"fmt"
	"strings"

	yaml3 "gopkg.in/yaml.v3"
)

func expandEnvScalarNode(node *yaml3.Node) (*yaml3.Node, error) {
	// depeding on the kind of node
	switch node.Kind {
	// when is a ScalarNode, replace its value with the ENV replaced
	case yaml3.ScalarNode:
		expandValue, err := ExpandEnv(node.Value, true)
		if err != nil {
			return node, err
		}
		node.Value = expandValue
		return node, nil
	// when is a Sequence and starts with $ can be a list of envs, so transform the list to key=value format
	case yaml3.SequenceNode:
		for indx, subNode := range node.Content {
			if strings.HasPrefix(subNode.Value, "$") {
				value := subNode.Value
				key := strings.TrimSuffix(strings.TrimPrefix(value, "$"), "=")
				node.Content[indx].Value = fmt.Sprintf("%s=%s", key, value)
			}
		}
	// when MappingNode and only the ENV, transform the node by adding a ScalarNode so there is key and value nodes for the map
	case yaml3.MappingNode:
		for indx, subNode := range node.Content {
			value := subNode.Value
			if indx%2 == 0 && strings.HasPrefix(value, "$") && node.Content[indx+1] != nil && node.Content[indx+1].Value == "" {
				node.Content[indx].Value = strings.TrimPrefix(value, "$")
				node.Content[indx+1] = &yaml3.Node{
					Kind:  yaml3.ScalarNode,
					Value: value,
				}
			}
		}
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

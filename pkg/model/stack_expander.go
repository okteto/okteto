package model

import (
	"bytes"
	"strings"

	yaml3 "gopkg.in/yaml.v3"
)

func isMapString(value string) bool {
	colonSplit := strings.SplitN(value, ":", 3)
	switch len(colonSplit) {
	case 2:
		indxColon := strings.Index(value, ":")
		indxRightCurlyBracket := strings.Index(value, "}")
		return indxColon > indxRightCurlyBracket
	case 3:
		return true
	}
	return false
}
func isCurlyEnv(value string) bool {
	return strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}")
}
func hasEnvDefaultValue(value string) (int, bool) {
	return strings.Index(value, ":-"), strings.Contains(value, ":-")
}

func isEnvStringKey(value string) (string, bool) {
	if !strings.HasPrefix(value, "$") {
		return "", false
	}
	if isMapString(value) {
		return "", false
	}

	key := value
	if ok := isCurlyEnv(value); ok {
		key = strings.TrimPrefix(strings.TrimSuffix(key, "}"), "${")
		if indx, ok := hasEnvDefaultValue(key); ok {
			key = key[:indx]
		}
		return key, true
	}

	return strings.TrimPrefix(key, "$"), true
}

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

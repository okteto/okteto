package model

import (
	"strings"
)

func uniqueStrings(values []string) []string {
	visitedValues := map[string]bool{}
	result := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if visitedValues[v] {
			continue
		}
		visitedValues[v] = true
		result = append(result, v)
	}
	return result
}

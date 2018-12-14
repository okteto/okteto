package cmd

import (
	"reflect"
	"testing"
)

func Test_parseArguments(t *testing.T) {
	var tests = []struct {
		scriptArgs string
		extraArgs  []string
		expected   []string
	}{
		{"run", nil, []string{"run"}},
		{"run", []string{}, []string{"run"}},
		{"run", []string{"run", "--all"}, []string{"run", "--all"}},
	}

	for _, tt := range tests {
		result := parseArguments(tt.scriptArgs, tt.extraArgs)
		if !reflect.DeepEqual(result, tt.expected) {
			t.Errorf("Actual: %v Expected: %v", result, tt.expected)
		}
	}

}

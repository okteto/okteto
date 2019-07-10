package cmd

import "testing"

func Test_getDeploymentName(t *testing.T) {
	var tests = []struct {
		name       string
		deployment string
		expected   string
	}{
		{name: "all lower case", deployment: "lowercase", expected: "lowercase"},
		{name: "with some lower case", deployment: "lowerCase", expected: "lowercase"},
		{name: "upper case", deployment: "UpperCase", expected: "uppercase"},
		{name: "valid symbols", deployment: "getting-started.test", expected: "getting-started-test"},
		{name: "invalid symbols", deployment: "getting_$#started", expected: "getting-started"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getDeploymentName(tt.deployment)
			if actual != tt.expected {
				t.Errorf("got: %s expected: %s", actual, tt.expected)
			}
		})
	}
}

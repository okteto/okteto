package model

import (
	"fmt"
	"os"
	"testing"
)

func TestReadDev(t *testing.T) {
	wd, _ := os.Getwd()

	var tests = []struct {
		name     string
		source   string
		target   string
		devPath  string
		expected string
	}{
		{
			name:     "relative-source",
			source:   ".",
			target:   "/go/src/github.com/okteto/cnd",
			devPath:  "/go/src/github.com/okteto/cnd/cnd.yml",
			expected: "/go/src/github.com/okteto/cnd"},
		{
			name:     "relative-source-abs",
			source:   "/go/src/github.com/okteto/cnd",
			target:   "/src/github.com/okteto/cnd",
			devPath:  "cnd.yml",
			expected: "/go/src/github.com/okteto/cnd"},
		{
			name:     "relative-dev-path",
			source:   "k8/src",
			target:   "/go/src/github.com/okteto/cnd",
			devPath:  "cnd/cnd.yml",
			expected: fmt.Sprintf("%s/cnd/k8/src", wd),
		},
		{
			name:     "relative-source-path",
			source:   "./frontend",
			target:   "/usr/src/frontend",
			devPath:  "cnd-frontend.yml",
			expected: fmt.Sprintf("%s/frontend", wd),
		},
	}

	for _, tt := range tests {
		dev := Dev{
			Name: "test",
			Mount: mount{
				Source: tt.source,
				Target: tt.target,
			},
		}

		dev.fixPath(tt.devPath)
		if dev.Mount.Source != tt.expected {
			t.Errorf("%s != %s", dev.Mount.Source, tt.expected)
		}
	}
}

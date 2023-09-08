package preview

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func Test_getPreviewOutput(t *testing.T) {
	var tests = []struct {
		name           string
		previews       []types.Preview
		expectedResult []previewOutput
	}{
		{
			name: "List all previews",
			previews: []types.Preview{
				{
					ID:            "test",
					PreviewLabels: []string{"label-1", "label-2"},
				},
				{
					ID:            "test-1",
					PreviewLabels: []string{"label-3"},
				},
				{
					ID:            "test-2",
					Sleeping:      true,
					PreviewLabels: []string{"-"},
				},
			},
			expectedResult: []previewOutput{
				{
					Name:     "test",
					Scope:    "",
					Sleeping: false,
					Labels:   []string{"label-1", "label-2"},
				},
				{
					Name:     "test-1",
					Scope:    "",
					Sleeping: false,
					Labels:   []string{"label-3"},
				},
				{
					Name:     "test-2",
					Scope:    "",
					Sleeping: true,
					Labels:   []string{"-"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPreviewOutput(tt.previews)
			assert.ElementsMatch(t, result, tt.expectedResult)
		})
	}
}

func Test_validatePreviewListOutput(t *testing.T) {
	var tests = []struct {
		name        string
		output      string
		expectedErr error
	}{
		{
			name:        "output format is yaml",
			output:      "yaml",
			expectedErr: nil,
		},
		{
			name:        "output format is json",
			output:      "json",
			expectedErr: nil,
		},
		{
			name:        "output format is not valid",
			output:      "xml",
			expectedErr: fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'yaml']"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePreviewListOutput(tt.output)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func Test_getPreviewDefaultOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    previewOutput
		expected string
	}{
		{
			name: "preview with no labels",
			input: previewOutput{
				Name:     "my-preview",
				Scope:    "personal",
				Sleeping: false,
			},
			expected: "my-preview\tpersonal\tfalse\t-\n",
		},
		{
			name: "preview with labels",
			input: previewOutput{
				Name:     "my-preview",
				Scope:    "personal",
				Sleeping: false,
				Labels:   []string{"one", "two"},
			},
			expected: "my-preview\tpersonal\tfalse\tone, two\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPreviewDefaultOutput(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_executeListPreviews(t *testing.T) {
	tests := []struct {
		name   string
		format string
		input  []previewOutput

		expectedOutput string
	}{
		{
			name:           "empty list ",
			format:         "",
			expectedOutput: "There are no previews\n",
		},
		{
			name:           "empty list ",
			format:         "json",
			expectedOutput: "[]\n",
		},
		{
			name:           "empty list ",
			format:         "json",
			expectedOutput: "[]\n",
		},
		{
			name:   "list - default format",
			format: "",
			input: []previewOutput{
				{
					Name:     "test",
					Scope:    "personal",
					Sleeping: true,
					Labels:   []string{"test", "okteto"},
				},
				{
					Name:     "test2",
					Scope:    "global",
					Sleeping: true,
				},
			},
			expectedOutput: `Name   Scope     Sleeping  Labels
test   personal  true      test, okteto
test2  global    true      -
`,
		},
		{
			name:   "list - json format",
			format: "json",
			input: []previewOutput{
				{
					Name:     "test",
					Scope:    "personal",
					Sleeping: true,
					Labels:   []string{"test", "okteto"},
				},
				{
					Name:     "test2",
					Scope:    "global",
					Sleeping: true,
				},
			},
			expectedOutput: "[\n {\n  \"name\": \"test\",\n  \"scope\": \"personal\",\n  \"sleeping\": true,\n  \"labels\": [\n   \"test\",\n   \"okteto\"\n  ]\n },\n {\n  \"name\": \"test2\",\n  \"scope\": \"global\",\n  \"sleeping\": true,\n  \"labels\": null\n }\n]\n",
		},
		{
			name:   "list - yaml format",
			format: "yaml",
			input: []previewOutput{
				{
					Name:     "test",
					Scope:    "personal",
					Sleeping: true,
					Labels:   []string{"test", "okteto"},
				},
				{
					Name:     "test2",
					Scope:    "global",
					Sleeping: true,
				},
			},
			expectedOutput: "- name: test\n  scope: personal\n  sleeping: true\n  labels:\n  - test\n  - okteto\n- name: test2\n  scope: global\n  sleeping: true\n  labels: []\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			initialStdout := os.Stdout
			r, w, _ := os.Pipe()

			// replace Stdout for tests
			os.Stdout = w

			err := executeListPreviews(tt.input, tt.format)
			assert.NoError(t, err)

			w.Close()
			out, err := io.ReadAll(r)
			assert.NoError(t, err)

			// return back to initial
			os.Stdout = initialStdout

			assert.Equal(t, tt.expectedOutput, string(out))
		})
	}

}
